// cmd/ghost-silicon/main.go
// Ghost-Silicon — Windows 11 privacy browser.
//
// Startup sequence (GUI mode, default):
//  1. Parse CLI flags; auto-discover config if none is given.
//  2. Bootstrap telemetry, run storage migrations.
//  3. Load (or auto-generate) the active identity profile.
//  4. Create the IPC bridge; start the named-pipe listener.
//  5. Optionally start a secondary renderer process.
//  6. Open the Walk browser window — blocks until the window closes.
//  7. Graceful LIFO shutdown.
//
// Double-click behaviour:
//  No -config flag is needed. The binary searches for ghost-silicon.yaml in:
//    1. Next to the .exe  (bin\configs\ghost-silicon.yaml)
//    2. One directory up  (configs\ghost-silicon.yaml — dev layout)
//    3. %APPDATA%\ghost-silicon\ghost-silicon.yaml
//  If none is found the built-in defaults are used (all paths go to APPDATA).
//
//  Any fatal error is shown as a Windows MessageBox AND written to
//  %APPDATA%\ghost-silicon\crash.log so it is never silently swallowed.

package main

//go:generate windres -i resource.rc -o resource.syso -O coff

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"

	"ghost-silicon/internal/app/bootstrap"
	"ghost-silicon/internal/app/lifecycle"
	"ghost-silicon/internal/app/shutdown"
	"ghost-silicon/internal/engine/adapter"
	eruntime "ghost-silicon/internal/engine/runtime"
	"ghost-silicon/internal/ipc/jsonrpc"
	"ghost-silicon/internal/ipc/namedpipe"
	"ghost-silicon/internal/platform/windows/filesystem"
	"ghost-silicon/internal/platform/windows/process"
	"ghost-silicon/pkg/bridge"
	"ghost-silicon/pkg/browser"
	"ghost-silicon/pkg/identity"
	"ghost-silicon/pkg/renderer"
	"ghost-silicon/pkg/storage"
	"ghost-silicon/pkg/version"
)

func main() {
	if err := run(); err != nil {
		showFatalError(err)
		os.Exit(1)
	}
}

func run() error {
	// ── CLI flags ────────────────────────────────────────────────────────
	var (
		configPath  = flag.String("config", "", "path to ghost-silicon.yaml (auto-discovered if empty)")
		showVersion = flag.Bool("version", false, "print version and exit")
		headless    = flag.Bool("headless", false, "supervisor-only mode, no GUI")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println(version.FullString())
		return nil
	}

	// ── Config discovery ─────────────────────────────────────────────────
	resolvedConfig := *configPath
	if resolvedConfig == "" {
		resolvedConfig = discoverConfig()
	}

	// ── Bootstrap ────────────────────────────────────────────────────────
	app, err := bootstrap.Run(bootstrap.Options{
		ConfigPath: resolvedConfig,
	})
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	cfg := app.Config
	log := app.Log
	auditor := app.Auditor

	// ── Storage migrations ───────────────────────────────────────────────
	if err := storage.Migrate(cfg.App.DataDir); err != nil {
		return fmt.Errorf("storage migration: %w", err)
	}

	// ── Profile store ────────────────────────────────────────────────────
	profileStore, err := storage.NewProfileStore(cfg.Identity.ProfileDir)
	if err != nil {
		return fmt.Errorf("profile store: %w", err)
	}

	// ── Load or generate profile ─────────────────────────────────────────
	activeProfile, err := resolveProfile(
		profileStore,
		cfg.Identity.DefaultProfile,
		cfg.Identity.AutoGenerate,
	)
	if err != nil {
		return fmt.Errorf("resolve profile: %w", err)
	}
	log.Info("active profile",
		"profile_id", activeProfile.ID,
		"profile_name", activeProfile.Name,
	)

	// ── Session setup ────────────────────────────────────────────────────
	sessionID := newSessionID()
	layout, err := filesystem.CreateSessionLayout(cfg.Storage.BaseDir, sessionID)
	if err != nil {
		return fmt.Errorf("session layout: %w", err)
	}

	// ── Browser profile store ────────────────────────────────────────────
	// Named accounts that persist tabs, history, bookmarks, and WebView2
	// session data (cookies, localStorage) across restarts.
	bpStore, err := browser.NewBrowserProfileStore(
		filepath.Join(cfg.App.DataDir, "browser-profiles"),
	)
	if err != nil {
		return fmt.Errorf("browser profile store: %w", err)
	}

	// ── Setup wizard (GUI mode only) ─────────────────────────────────────
	// Always shown on launch so the user picks an identity before browsing.
	// Headless mode skips it and uses the stored profile.
	var searchEngineURL string
	userDataDir := layout.Root // fallback for headless mode
	if !*headless {
		wizardResult, wizardErr := browser.RunSetupWizard(layout.Root, bpStore)
		if wizardErr != nil {
			return fmt.Errorf("setup wizard: %w", wizardErr)
		}
		if wizardResult == nil {
			return nil // user closed the wizard without launching
		}
		activeProfile = wizardResult.Profile
		searchEngineURL = wizardResult.SearchEngineURL
		if wizardResult.BrowserProfileDir != "" {
			userDataDir = wizardResult.BrowserProfileDir
		}
		log.Info("setup wizard completed",
			"profile_name", activeProfile.Name,
			"os", activeProfile.Hardware.Platform,
			"search_engine", searchEngineURL,
			"profile_dir", userDataDir,
		)
	}

	// ── IPC bridge ───────────────────────────────────────────────────────
	rpcSrv := jsonrpc.NewServer(log)
	br := bridge.New(activeProfile, sessionID, log, auditor)
	br.Register(rpcSrv)

	pipeListener := namedpipe.NewListener(cfg.IPC.PipeName, log)
	if err := pipeListener.Listen(); err != nil {
		return fmt.Errorf("pipe listener: %w", err)
	}

	// ── Lifecycle manager ─────────────────────────────────────────────────
	lc := lifecycle.NewManager(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lc.OnStart("ipc-bridge", func(ctx context.Context) error {
		go func() {
			_ = pipeListener.Serve(ctx, func(conn net.Conn) {
				rpcSrv.ServeConn(ctx, conn)
			})
		}()
		return nil
	})
	lc.OnStop("ipc-bridge", func(_ context.Context) error {
		return pipeListener.Close()
	})

	// ── Adapter & runtime (only when a separate renderer is configured) ──
	// In GUI mode the embedded WebView2 IS the renderer.  Only start a
	// secondary renderer process when engine.executable is explicitly set.
	if cfg.Engine.Executable != "" {
		eng, err := adapter.Global.Get("mock", cfg.Engine.Executable)
		if err != nil {
			log.Warn("renderer adapter not found — using mock", "engine", cfg.Engine.Executable)
			eng = adapter.NewMockAdapter()
		}

		rt := eruntime.New(eruntime.Options{
			Adapter: eng,
			Log:     log,
			Auditor: auditor,
			RestartPolicy: process.RestartPolicy{
				MaxCrashes:   cfg.Engine.CrashRestartLimit,
				InitialDelay: cfg.Engine.CrashRestartDelay,
				MaxDelay:     cfg.Engine.CrashRestartDelay * 10,
			},
		})

		startOpts := renderer.StartOptions{
			ProfileID:   activeProfile.ID,
			SessionID:   sessionID,
			PipeName:    cfg.IPC.PipeName,
			UserDataDir: layout.Root,
			CacheDir:    layout.Cache,
			ExtraArgs:   cfg.Engine.Args,
		}

		lc.OnStart("renderer", func(ctx context.Context) error {
			go func() {
				if err := rt.Start(ctx, startOpts); err != nil {
					log.Error("renderer exited", "error", err.Error())
				}
			}()
			return nil
		})
		lc.OnStop("renderer", func(_ context.Context) error {
			return rt.Stop(cfg.App.ShutdownTimeout)
		})
	} else {
		log.Info("renderer: using embedded WebView2 — no separate process")
	}

	// ── Start all subsystems ─────────────────────────────────────────────
	if err := lc.Start(ctx); err != nil {
		return fmt.Errorf("startup: %w", err)
	}

	log.Info("ghost-silicon running",
		"session_id", sessionID,
		"pipe", cfg.IPC.PipeName,
		"version", version.Version,
		"headless", *headless,
		"config", resolvedConfig,
	)

	// ── HEADLESS MODE ─────────────────────────────────────────────────────
	if *headless {
		done := make(chan struct{})
		shutHandler := shutdown.NewHandler(cfg.App.ShutdownTimeout, log)
		go func() {
			defer close(done)
			shutCtx, shutCancel := context.WithTimeout(
				context.Background(), cfg.App.ShutdownTimeout)
			defer shutCancel()
			_ = lc.Stop(shutCtx)
		}()
		shutHandler.WaitForSignal(cancel, done)
		return nil
	}

	// ── GUI MODE ──────────────────────────────────────────────────────────
	win, err := browser.NewWindow(browser.WindowOptions{
		Bridge:          br,
		UserDataDir:     userDataDir,
		PipeName:        cfg.IPC.PipeName,
		SearchEngineURL: searchEngineURL,
		Log:             log,
	})
	if err != nil {
		return fmt.Errorf("browser window create: %w", err)
	}

	winDone := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigCh)
		select {
		case sig := <-sigCh:
			log.Info("signal received — closing window", "signal", sig.String())
			win.Close()
			cancel()
		case <-winDone:
		}
	}()

	if err := win.Open(); err != nil {
		return fmt.Errorf("browser window: %w", err)
	}

	close(winDone)
	cancel()

	log.Info("browser window closed — shutting down subsystems")
	shutCtx, shutCancel := context.WithTimeout(
		context.Background(), cfg.App.ShutdownTimeout)
	defer shutCancel()
	_ = lc.Stop(shutCtx)

	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// discoverConfig searches well-known locations for ghost-silicon.yaml so the
// binary works when double-clicked without any CLI arguments.
func discoverConfig() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	exeDir := filepath.Dir(exePath)

	candidates := []string{
		filepath.Join(exeDir, "configs", "ghost-silicon.yaml"),                     // next to exe
		filepath.Join(exeDir, "..", "configs", "ghost-silicon.yaml"),               // dev layout
		filepath.Join(os.Getenv("APPDATA"), "ghost-silicon", "ghost-silicon.yaml"), // installed
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "" // loader falls back to built-in defaults
}

// showFatalError shows a Windows MessageBox with the error message (visible
// when double-clicking with no console) and writes a crash.log to APPDATA.
func showFatalError(err error) {
	msg := fmt.Sprintf(
		"Ghost-Silicon failed to start:\n\n%v\n\nA crash log has been written to:\n%%APPDATA%%\\ghost-silicon\\crash.log",
		err,
	)

	if appData := os.Getenv("APPDATA"); appData != "" {
		logDir := filepath.Join(appData, "ghost-silicon")
		_ = os.MkdirAll(logDir, 0o700)
		_ = os.WriteFile(filepath.Join(logDir, "crash.log"), []byte(msg+"\n"), 0o600)
	}

	title, _ := windows.UTF16PtrFromString("Ghost-Silicon — Startup Error")
	text, _ := windows.UTF16PtrFromString(msg)
	_, _ = windows.MessageBox(0, text, title, windows.MB_ICONERROR|windows.MB_OK)
}

// resolveProfile loads the default profile, the first available profile,
// or auto-generates one when AutoGenerate is true.
func resolveProfile(
	store *storage.ProfileStore,
	defaultID string,
	autoGenerate bool,
) (*identity.Profile, error) {
	if defaultID != "" {
		if p, err := store.Load(defaultID); err == nil {
			return p, nil
		}
	}
	if metas, err := store.List(); err == nil && len(metas) > 0 {
		return store.Load(metas[0].ID)
	}
	if autoGenerate {
		p := identity.Windows11DesktopTemplate()
		if saveErr := store.Save(p); saveErr != nil {
			fmt.Fprintf(os.Stderr,
				"warning: could not save auto-generated profile: %v\n", saveErr)
		}
		return p, nil
	}
	return nil, fmt.Errorf("no profiles found and auto_generate is disabled")
}

// newSessionID returns a cryptographically random 16-byte hex session ID.
func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "session-fallback-0000000000000000"
	}
	return hex.EncodeToString(b)
}

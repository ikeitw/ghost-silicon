// cmd/ghost-silicon/main.go
// Ghost-Silicon — Windows 11 browser supervisor.
// Entry point: loads config, bootstraps telemetry, wires all subsystems,
// starts the IPC bridge, launches the renderer, and blocks until a signal.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"

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
	"ghost-silicon/pkg/identity"
	"ghost-silicon/pkg/renderer"
	"ghost-silicon/pkg/storage"
	"ghost-silicon/pkg/version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ghost-silicon: fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// ── CLI flags ────────────────────────────────────────────────────────
	var (
		configPath  = flag.String("config", "", "path to ghost-silicon.yaml")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println(version.FullString())
		return nil
	}

	// ── Bootstrap ────────────────────────────────────────────────────────
	app, err := bootstrap.Run(bootstrap.Options{
		ConfigPath: *configPath,
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
	activeProfile, err := resolveProfile(profileStore, cfg.Identity.DefaultProfile, cfg.Identity.AutoGenerate)
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

	// ── IPC bridge ───────────────────────────────────────────────────────
	rpcSrv := jsonrpc.NewServer(log)
	br := bridge.New(activeProfile, sessionID, log, auditor)
	br.Register(rpcSrv)

	pipeListener := namedpipe.NewListener(cfg.IPC.PipeName, log)
	if err := pipeListener.Listen(); err != nil {
		return fmt.Errorf("pipe listener: %w", err)
	}

	// ── Adapter & runtime ────────────────────────────────────────────────
	eng, err := adapter.Global.Get("mock", cfg.Engine.Executable)
	if err != nil {
		// Fall back to mock adapter when no engine is registered.
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

	// ── Lifecycle ────────────────────────────────────────────────────────
	lc := lifecycle.NewManager(log)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	// IPC bridge hook
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

	// Renderer hook
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

	// ── Start ────────────────────────────────────────────────────────────
	if err := lc.Start(ctx); err != nil {
		cancel()
		return fmt.Errorf("startup: %w", err)
	}

	log.Info("ghost-silicon running",
		"session_id", sessionID,
		"pipe", cfg.IPC.PipeName,
		"version", version.Version,
	)

	// ── Wait for OS signal ───────────────────────────────────────────────
	shutHandler := shutdown.NewHandler(cfg.App.ShutdownTimeout, log)
	go func() {
		defer close(done)
		shutCtx, shutCancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
		defer shutCancel()
		_ = lc.Stop(shutCtx)
	}()

	shutHandler.WaitForSignal(cancel, done)
	return nil
}

// resolveProfile loads the configured default profile, or the first available
// profile, or auto-generates one if AutoGenerate is set.
func resolveProfile(store *storage.ProfileStore, defaultID string, autoGenerate bool) (*identity.Profile, error) {
	if defaultID != "" {
		p, err := store.Load(defaultID)
		if err == nil {
			return p, nil
		}
	}
	metas, err := store.List()
	if err == nil && len(metas) > 0 {
		return store.Load(metas[0].ID)
	}
	if autoGenerate {
		p := identity.Windows11DesktopTemplate()
		if saveErr := store.Save(p); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save auto-generated profile: %v\n", saveErr)
		}
		return p, nil
	}
	return nil, fmt.Errorf("no profiles found and auto_generate is disabled")
}

// newSessionID generates a cryptographically random 16-byte hex session ID.
func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "session-fallback-0000000000000000"
	}
	return hex.EncodeToString(b)
}

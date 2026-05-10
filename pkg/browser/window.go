// pkg/browser/window.go
//go:build windows

// Package browser — main browser window.
// Window is the root Walk MainWindow. It owns the WebView panel, bookmarks,
// download manager, devtools, and menu actions. The browser chrome (address
// bar, tabs, nav buttons) is rendered as an HTML overlay inside WebView2.
package browser

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/lxn/walk"

	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/bridge"
	"ghost-silicon/pkg/version"
)

const (
	defaultHomeURL = "ghost://newtab"
	defaultWindowW = 1280
	defaultWindowH = 800
	bookmarkFile   = "bookmarks.json"
)

// WindowOptions carries everything Window needs at construction time.
type WindowOptions struct {
	Bridge      *bridge.Bridge
	UserDataDir string
	PipeName    string
	Log         *logging.Logger
}

// Window is the top-level Ghost-Silicon browser window.
type Window struct {
	mw *walk.MainWindow

	webview   *WebViewPanel
	devtools  *DevToolsPanel
	bookmarks *BookmarkStore
	dlMgr     *DownloadManager
	actions   *Actions

	opts WindowOptions
	log  *logging.Logger
	zoom float64

	cancelFn context.CancelFunc
}

// NewWindow creates the browser window but does not open it.
// Call Open() to display it and run the message loop.
func NewWindow(opts WindowOptions) (*Window, error) {
	if opts.Bridge == nil {
		return nil, fmt.Errorf("browser.NewWindow: Bridge must not be nil")
	}
	if opts.Log == nil {
		opts.Log, _ = logging.New(nil)
	}

	w := &Window{
		opts: opts,
		log:  opts.Log.WithComponent("browser"),
		zoom: 1.0,
	}
	return w, nil
}

// Open creates the Walk MainWindow, lays out all sub-components, navigates to
// the home page, and starts the Walk message loop.
// It blocks until the window is closed.
func (w *Window) Open() error {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelFn = cancel
	defer cancel()

	// ── MainWindow ────────────────────────────────────────────────────────
	mw, err := walk.NewMainWindow()
	if err != nil {
		return fmt.Errorf("walk main window: %w", err)
	}
	w.mw = mw
	mw.SetTitle(w.windowTitle(""))
	mw.SetSize(walk.Size{Width: defaultWindowW, Height: defaultWindowH})

	// ── Menu bar + Actions ────────────────────────────────────────────────
	w.actions, err = BuildMenu(mw)
	if err != nil {
		return fmt.Errorf("build menu: %w", err)
	}

	// ── WebView panel (fills entire client area) ──────────────────────────
	// The browser chrome (toolbar, tabs, address bar) is rendered as a
	// position:fixed HTML overlay injected into every page by WebViewPanel.
	// This eliminates all Win32 child-window Z-order / WndProc conflicts.
	w.webview, err = NewWebViewPanel(mw, w.opts.Bridge, w.opts.UserDataDir, w.log)
	if err != nil {
		return fmt.Errorf("webview panel: %w", err)
	}

	// ── Download manager + bookmarks ──────────────────────────────────────
	w.dlMgr = NewDownloadManager()
	bmPath := filepath.Join(w.opts.UserDataDir, bookmarkFile)
	w.bookmarks, err = NewBookmarkStore(bmPath)
	if err != nil {
		w.log.Warn("bookmark store unavailable", "error", err.Error())
		w.bookmarks, _ = NewBookmarkStore("")
	}

	// ── DevTools ──────────────────────────────────────────────────────────
	w.devtools = NewDevToolsPanel(w.webview.WebView())
	w.devtools.InjectConsoleShortcut(w.actions.DevTools)

	// ── Wire callbacks ────────────────────────────────────────────────────
	w.wireActions()
	w.wireWebViewCallbacks()

	// ── Status bar ────────────────────────────────────────────────────────
	if err := w.buildStatusBar(); err != nil {
		w.log.Warn("status bar unavailable", "error", err.Error())
	}

	// ── Closing hook ──────────────────────────────────────────────────────
	mw.Closing().Attach(func(cancelled *bool, reason walk.CloseReason) {
		w.log.Info("browser window closing")
		cancel()
		DisposeThemeFonts()
	})

	// ── Initial navigation ────────────────────────────────────────────────
	w.log.Info("browser window open",
		"profile", w.opts.Bridge.Profile().Name,
		"pipe", w.opts.PipeName,
		"version", version.Version,
	)
	w.webview.Navigate(defaultHomeURL)

	// ── Post-layout resize ────────────────────────────────────────────────
	// Walk's SizeChanged fires during the first layout pass, but ClientBounds()
	// returns zero at that point because mw.Run() hasn't processed its first
	// WM_PAINT yet.  We schedule a ForceResize 300 ms after startup so that
	// WebView2 is sized and the initial page is visible immediately on open.
	go func() {
		time.Sleep(300 * time.Millisecond)
		w.mw.Synchronize(func() {
			w.webview.ForceResize()
			w.webview.Navigate(defaultHomeURL)
		})
	}()

	// ── Message loop ──────────────────────────────────────────────────────
	_ = ctx // reserved for future goroutines owned by Window
	mw.Run()
	return nil
}

// Close requests that the Walk window close.
// Safe to call from any goroutine.
func (w *Window) Close() {
	if w.mw != nil {
		w.mw.Synchronize(func() {
			w.mw.Close()
		})
	}
}

// UpdateProfile hot-swaps the active identity profile backing the polyfill.
// Safe to call from any goroutine; schedules work on the UI thread.
func (w *Window) UpdateProfile() {
	if w.mw == nil {
		return
	}
	w.mw.Synchronize(func() {
		w.webview.UpdateProfile()
	})
}

// ── Tab wiring ────────────────────────────────────────────────────────────────

// ── Action wiring ─────────────────────────────────────────────────────────────

func (w *Window) wireActions() {
	a := w.actions

	a.Back.Triggered().Attach(func() { w.webview.GoBack() })
	a.Forward.Triggered().Attach(func() { w.webview.GoForward() })
	a.Reload.Triggered().Attach(func() { w.webview.Reload() })
	a.Stop.Triggered().Attach(func() { w.webview.Stop() })
	a.Home.Triggered().Attach(func() { w.webview.Navigate(defaultHomeURL) })

	a.NewTab.Triggered().Attach(func() { w.webview.Navigate(defaultHomeURL) })
	a.CloseTab.Triggered().Attach(func() { w.mw.Close() })

	a.ZoomIn.Triggered().Attach(func() { w.adjustZoom(+0.1) })
	a.ZoomOut.Triggered().Attach(func() { w.adjustZoom(-0.1) })
	a.ZoomReset.Triggered().Attach(func() { w.setZoom(1.0) })

	a.Fullscreen.Triggered().Attach(func() {
		w.mw.SetFullscreen(!w.mw.Fullscreen())
	})

	a.SwitchProfile.Triggered().Attach(func() {
		walk.MsgBox(w.mw, "Profile Switcher",
			"Profile switcher UI coming in Phase 2.\nCurrent: "+
				w.opts.Bridge.Profile().Name,
			walk.MsgBoxOK|walk.MsgBoxIconInformation)
	})

	a.Settings.Triggered().Attach(func() {
		w.webview.Navigate("ghost://settings")
	})

	a.About.Triggered().Attach(func() {
		walk.MsgBox(w.mw,
			"About Ghost-Silicon",
			fmt.Sprintf("Ghost-Silicon %s\n\nProfile-driven privacy browser.\nProfile: %s",
				version.FullString(),
				w.opts.Bridge.Profile().Name),
			walk.MsgBoxOK|walk.MsgBoxIconInformation)
	})

	a.Quit.Triggered().Attach(func() { w.mw.Close() })
}

// ── WebView event callbacks ───────────────────────────────────────────────────

func (w *Window) wireWebViewCallbacks() {
	wv := w.webview

	wv.OnTitleChange = func(title string) {
		w.mw.Synchronize(func() {
			w.mw.SetTitle(w.windowTitle(title))
		})
	}

	wv.OnLoadError = func(url, msg string) {
		w.log.Warn("page load error", "url", url, "error", msg)
	}
}

// ── Status bar ────────────────────────────────────────────────────────────────

func (w *Window) buildStatusBar() error {
	sb, err := walk.NewStatusBar(w.mw)
	if err != nil {
		return err
	}
	item := walk.NewStatusBarItem()
	item.SetText("Ready")
	sb.Items().Add(item)
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (w *Window) windowTitle(pageTitle string) string {
	if pageTitle == "" {
		return "Ghost-Silicon"
	}
	return pageTitle + " — Ghost-Silicon"
}

func (w *Window) adjustZoom(delta float64) {
	z := w.zoom + delta
	if z < 0.25 {
		z = 0.25
	}
	if z > 5.0 {
		z = 5.0
	}
	w.setZoom(z)
}

func (w *Window) setZoom(z float64) {
	w.zoom = z
	w.webview.SetZoom(z)
}

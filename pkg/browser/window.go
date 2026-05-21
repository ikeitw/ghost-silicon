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
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"

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
	Bridge          *bridge.Bridge
	UserDataDir     string
	PipeName        string
	SearchEngineURL string
	Log             *logging.Logger
}

// Window is the top-level Ghost-Silicon browser window.
type Window struct {
	mw *walk.MainWindow

	webview      *WebViewPanel
	devtools     *DevToolsPanel
	bookmarks    *BookmarkStore
	browsingHist *BrowsingHistoryStore
	dlMgr        *DownloadManager
	actions      *Actions

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

	// ── Frameless chrome ──────────────────────────────────────────────────
	// Remove the OS title bar so the HTML overlay tab strip acts as the
	// window frame (Chrome-style). We keep WS_THICKFRAME so the window
	// remains resizable from all edges, and call DwmExtendFrameIntoClientArea
	// with a 1-pixel top margin to restore the DWM drop-shadow and Windows 11
	// rounded corners that are otherwise lost when WS_CAPTION is removed.
	mwHWND := win.HWND(uintptr(mw.Handle()))
	curStyle := win.GetWindowLong(mwHWND, win.GWL_STYLE)
	win.SetWindowLong(mwHWND, win.GWL_STYLE, curStyle&^win.WS_CAPTION)
	win.SetWindowPos(mwHWND, 0, 0, 0, 0, 0,
		win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)

	// Restore shadow + rounded corners via DWM.
	type dwmMargins struct{ L, R, T, B int32 }
	m := dwmMargins{0, 0, 1, 0}
	syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmExtendFrameIntoClientArea").Call(
		uintptr(mwHWND), uintptr(unsafe.Pointer(&m)))

	mw.SetSize(walk.Size{Width: defaultWindowW, Height: defaultWindowH})

	// Dark background — prevents a white flash when the window resizes before
	// WebView2 catches up. Colour matches the ghost:// page background.
	if bgBrush, err := walk.NewSolidColorBrush(walk.RGB(10, 10, 30)); err == nil {
		mw.SetBackground(bgBrush)
	}

	// WS_CLIPCHILDREN: prevent Walk's background fill from painting over the
	// WebView2 area during resize/maximize transitions.
	curStyle2 := win.GetWindowLong(mwHWND, win.GWL_STYLE)
	win.SetWindowLong(mwHWND, win.GWL_STYLE, curStyle2|win.WS_CLIPCHILDREN)

	// Walk requires a non-nil layout on the main window so its internal
	// startLayout call (triggered on every WM_SIZE) does not panic.
	// The VBoxLayout has no Walk-managed children — WebView2 is a raw HWND
	// invisible to Walk's widget tree — so it is a safe no-op.
	emptyLayout := walk.NewVBoxLayout()
	emptyLayout.SetMargins(walk.Margins{})
	emptyLayout.SetSpacing(0)
	mw.SetLayout(emptyLayout)

	// ── Menu bar + Actions ────────────────────────────────────────────────
	w.actions, err = BuildMenu(mw)
	if err != nil {
		return fmt.Errorf("build menu: %w", err)
	}

	// ── Stores (created before WebView so they can be passed in) ─────────
	w.dlMgr = NewDownloadManager()
	bmPath := filepath.Join(w.opts.UserDataDir, bookmarkFile)
	w.bookmarks, err = NewBookmarkStore(bmPath)
	if err != nil {
		w.log.Warn("bookmark store unavailable", "error", err.Error())
		w.bookmarks, _ = NewBookmarkStore("")
	}
	histPath := filepath.Join(w.opts.UserDataDir, "history.json")
	w.browsingHist, err = NewBrowsingHistoryStore(histPath)
	if err != nil {
		w.log.Warn("history store unavailable", "error", err.Error())
		w.browsingHist, _ = NewBrowsingHistoryStore("")
	}

	// ── WebView panel (fills entire client area) ──────────────────────────
	// The browser chrome (toolbar, tabs, address bar) is rendered as a
	// position:fixed HTML overlay injected into every page by WebViewPanel.
	// This eliminates all Win32 child-window Z-order / WndProc conflicts.
	w.webview, err = NewWebViewPanel(mw, w.opts.Bridge, w.opts.UserDataDir, w.opts.SearchEngineURL, w.log,
		w.bookmarks, w.browsingHist, w.dlMgr)
	if err != nil {
		return fmt.Errorf("webview panel: %w", err)
	}

	// go-webview2 creates its own top-level window; Walk's MainWindow is an
	// unused background window.  Hide it so only the WebView2 window is
	// visible to the user.
	win.ShowWindow(mwHWND, win.SW_HIDE)

	// ── DevTools ──────────────────────────────────────────────────────────
	w.devtools = NewDevToolsPanel(w.webview.WebView())
	w.devtools.InjectConsoleShortcut(w.actions.DevTools)

	// ── Wire callbacks ────────────────────────────────────────────────────
	w.wireActions()
	w.wireWebViewCallbacks()

	// ── Initial navigation ────────────────────────────────────────────────
	w.log.Info("browser window open",
		"profile", w.opts.Bridge.Profile().Name,
		"pipe", w.opts.PipeName,
		"version", version.Version,
	)
	w.webview.Navigate(w.webview.InitialURL())

	// ── Message loop ──────────────────────────────────────────────────────
	_ = ctx // reserved for future goroutines owned by Window
	mw.Run()
	w.log.Info("browser window closing")
	DisposeThemeFonts()
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

// pkg/browser/window.go
//go:build windows

// Package browser — main browser window.
// Window is the root Walk MainWindow.  It owns the tab strip, toolbar,
// WebView panel, download panel, and all actions.  It is the single
// integration point between the supervisor's pkg/bridge and the visible UI.
//
// Lifecycle:
//
//	Open() — creates the window and blocks the calling goroutine (Walk
//	         message pump) until the window is closed.
//	Close() — may be called from any goroutine to request shutdown.
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
	// Bridge is the active IPC bridge — Window reads the profile from it and
	// calls UpdateProfile on hot-swap.
	Bridge *bridge.Bridge

	// UserDataDir is the session-isolated directory for WebView2 state and
	// bookmark persistence.
	UserDataDir string

	// PipeName is the named pipe the supervisor is listening on (informational;
	// logged on startup).
	PipeName string

	Log *logging.Logger
}

// Window is the top-level Ghost-Silicon browser window.
type Window struct {
	mw *walk.MainWindow

	// Sub-components
	tabs      *TabStrip
	toolbar   *Toolbar
	webview   *WebViewPanel
	downloads *DownloadPanel
	devtools  *DevToolsPanel
	bookmarks *BookmarkStore
	dlMgr     *DownloadManager
	actions   *Actions

	opts WindowOptions
	log  *logging.Logger

	// Current zoom level (1.0 = 100 %).
	zoom float64

	// cancelFn is called by Close() to stop any background goroutines.
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

	// Dark background to prevent white flash on startup.
	bgBrush, err := walk.NewSolidColorBrush(ColorBackground)
	if err == nil {
		mw.SetBackground(bgBrush)
	}

	// ── Menu bar + Actions ────────────────────────────────────────────────
	w.actions, err = BuildMenu(mw)
	if err != nil {
		return fmt.Errorf("build menu: %w", err)
	}

	// ── WebView2 panel — created FIRST so it gets the lowest Z-order ─────
	// Walk assigns Z-order by creation sequence. By creating WebView2 before
	// the toolbar and tab strip, those Walk widgets will paint on top of the
	// WebView2 area automatically — no manual Z-order management needed.
	// WebView2 uses the main window HWND as its parent and fills the entire
	// client area; the toolbar and tabs cover their own region on top.
	w.webview, err = NewWebViewPanel(mw, w.opts.Bridge, w.opts.UserDataDir, w.log)
	if err != nil {
		return fmt.Errorf("webview panel: %w", err)
	}

	// ── Root VBox layout (manages toolbar and tab strip only) ─────────────
	// WebView2 is NOT part of this layout — it fills the full window
	// independently. Walk's layout only sees the Walk widget objects it owns.
	rootLayout := walk.NewVBoxLayout()
	rootLayout.SetMargins(walk.Margins{})
	rootLayout.SetSpacing(0)
	mw.SetLayout(rootLayout)

	// ── Tab strip (row 1, on top of WebView2) ────────────────────────────
	w.tabs, err = NewTabStrip(mw)
	if err != nil {
		return fmt.Errorf("tab strip: %w", err)
	}

	// ── Toolbar (row 2, on top of WebView2) ──────────────────────────────
	w.toolbar, err = NewToolbar(mw)
	if err != nil {
		return fmt.Errorf("toolbar: %w", err)
	}
	w.toolbar.SetProfileName(w.opts.Bridge.Profile().Name)

	// ── Download manager (model only, no Walk widget in layout) ──────────
	w.dlMgr = NewDownloadManager()

	// ── Bookmarks ─────────────────────────────────────────────────────────
	bmPath := filepath.Join(w.opts.UserDataDir, bookmarkFile)
	w.bookmarks, err = NewBookmarkStore(bmPath)
	if err != nil {
		w.log.Warn("bookmark store unavailable", "error", err.Error())
		w.bookmarks, _ = NewBookmarkStore("")
	}

	// ── DevTools ──────────────────────────────────────────────────────────
	w.devtools = NewDevToolsPanel(w.webview.WebView())
	w.devtools.InjectConsoleShortcut(w.actions.DevTools)

	// ── Wire all callbacks ────────────────────────────────────────────────
	w.wireTabs()
	w.wireToolbar()
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
			w.webview.Navigate(defaultHomeURL) // re-navigate now that size is correct
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
		w.toolbar.SetProfileName(w.opts.Bridge.Profile().Name)
		w.webview.UpdateProfile()
	})
}

// ── Tab wiring ────────────────────────────────────────────────────────────────

func (w *Window) wireTabs() {
	w.tabs.OnSelect = func(tabID string) {
		tab := w.tabByID(tabID)
		if tab == nil {
			return
		}
		// Navigate WebView to the tab's current URL.
		if tab.URL != "" {
			w.webview.Navigate(tab.URL)
			w.toolbar.SetURL(tab.URL)
		}
		w.toolbar.SetNavState(
			tab.History.CanGoBack(),
			tab.History.CanGoForward(),
		)
		w.mw.SetTitle(w.windowTitle(tab.Title))
	}

	w.tabs.OnClose = func(tabID string) {
		w.tabs.CloseTab(tabID)
		// After close, the tab strip auto-selects the new active tab and fires
		// OnSelect, so we do not need to navigate here.
	}

	w.tabs.OnNew = func() {
		id := w.tabs.AddTab("", "New Tab")
		w.tabs.SelectTab(id)
		w.webview.Navigate(defaultHomeURL)
		w.toolbar.SetURL("")
	}
}

// ── Toolbar wiring ────────────────────────────────────────────────────────────

func (w *Window) wireToolbar() {
	w.toolbar.OnNavigate = func(url string) {
		active := w.tabs.ActiveTab()
		if active == nil {
			return
		}
		w.tabs.UpdateTab(active.ID, url, "", true)
		w.webview.Navigate(url)
	}

	w.toolbar.OnBack = func() {
		active := w.tabs.ActiveTab()
		if active == nil {
			return
		}
		if entry, ok := active.History.Back(); ok {
			w.webview.Navigate(entry.URL)
			w.toolbar.SetURL(entry.URL)
		} else {
			w.webview.GoBack()
		}
		w.refreshNavButtons()
	}

	w.toolbar.OnForward = func() {
		active := w.tabs.ActiveTab()
		if active == nil {
			return
		}
		if entry, ok := active.History.Forward(); ok {
			w.webview.Navigate(entry.URL)
			w.toolbar.SetURL(entry.URL)
		} else {
			w.webview.GoForward()
		}
		w.refreshNavButtons()
	}

	w.toolbar.OnReload = func() { w.webview.Reload() }
	w.toolbar.OnStop = func() { w.webview.Stop() }

	w.toolbar.OnHome = func() {
		w.webview.Navigate(defaultHomeURL)
		w.toolbar.SetURL(defaultHomeURL)
	}

	w.toolbar.OnBookmark = func(url string) {
		if url == "" || w.bookmarks == nil {
			return
		}
		if w.bookmarks.Has(url) {
			// Toggle off — remove.
			bm := w.bookmarks.ForURL(url)
			if bm != nil {
				_ = w.bookmarks.Remove(bm.ID)
			}
			w.toolbar.SetBookmarked(false)
		} else {
			title := ""
			if active := w.tabs.ActiveTab(); active != nil {
				title = active.Title
			}
			_, _ = w.bookmarks.Add(url, title)
			w.toolbar.SetBookmarked(true)
		}
	}

	w.toolbar.OnToggleMenu = func() {
		// Show the hamburger popup menu adjacent to the menu button.
		// Walk pops a context menu at the current cursor position.
		if menu, err := BuildContextMenu(true); err == nil {
			defer menu.Dispose()
			_ = walk.MsgBox(w.mw, "Menu",
				"Profile: "+w.opts.Bridge.Profile().Name,
				walk.MsgBoxOK)
		}
	}
}

// ── Action wiring ─────────────────────────────────────────────────────────────

func (w *Window) wireActions() {
	a := w.actions

	a.Back.Triggered().Attach(func() { w.toolbar.OnBack() })
	a.Forward.Triggered().Attach(func() { w.toolbar.OnForward() })
	a.Reload.Triggered().Attach(func() { w.webview.Reload() })
	a.Stop.Triggered().Attach(func() { w.webview.Stop() })
	a.Home.Triggered().Attach(func() {
		w.webview.Navigate(defaultHomeURL)
		w.toolbar.SetURL(defaultHomeURL)
	})

	a.NewTab.Triggered().Attach(func() { w.tabs.OnNew() })
	a.CloseTab.Triggered().Attach(func() {
		if t := w.tabs.ActiveTab(); t != nil {
			w.tabs.CloseTab(t.ID)
		}
	})

	a.ZoomIn.Triggered().Attach(func() { w.adjustZoom(+0.1) })
	a.ZoomOut.Triggered().Attach(func() { w.adjustZoom(-0.1) })
	a.ZoomReset.Triggered().Attach(func() { w.setZoom(1.0) })

	a.Fullscreen.Triggered().Attach(func() {
		w.mw.SetFullscreen(!w.mw.Fullscreen())
	})

	a.Downloads.Triggered().Attach(func() {
		if w.downloads != nil {
			w.downloads.SetVisible(!w.downloads.Visible())
		}
	})

	a.SwitchProfile.Triggered().Attach(func() {
		// Phase 2: open profile switcher dialog.
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

	a.Quit.Triggered().Attach(func() {
		w.mw.Close()
	})
}

// ── WebView event callbacks ───────────────────────────────────────────────────

func (w *Window) wireWebViewCallbacks() {
	wv := w.webview

	wv.OnLoadStart = func(url string) {
		w.mw.Synchronize(func() {
			w.toolbar.SetLoading(true)
			w.toolbar.SetURL(url)
			if active := w.tabs.ActiveTab(); active != nil {
				w.tabs.UpdateTab(active.ID, url, "", true)
				active.History.Push(url, "")
				w.refreshNavButtons()
			}
		})
	}

	wv.OnLoadComplete = func(url string) {
		w.mw.Synchronize(func() {
			w.toolbar.SetLoading(false)
			w.toolbar.SetURL(url)
			if w.bookmarks != nil {
				w.toolbar.SetBookmarked(w.bookmarks.Has(url))
			}
			if active := w.tabs.ActiveTab(); active != nil {
				w.tabs.UpdateTab(active.ID, url, active.Title, false)
			}
		})
	}

	wv.OnTitleChange = func(title string) {
		w.mw.Synchronize(func() {
			if active := w.tabs.ActiveTab(); active != nil {
				w.tabs.UpdateTab(active.ID, active.URL, title, false)
				active.History.UpdateTitle(title)
			}
			w.mw.SetTitle(w.windowTitle(title))
		})
	}

	wv.OnURLChange = func(url string) {
		w.mw.Synchronize(func() {
			w.toolbar.SetURL(url)
		})
	}

	wv.OnLoadError = func(url, msg string) {
		w.mw.Synchronize(func() {
			w.toolbar.SetLoading(false)
			w.log.Warn("page load error", "url", url, "error", msg)
		})
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

func (w *Window) refreshNavButtons() {
	active := w.tabs.ActiveTab()
	if active == nil {
		w.toolbar.SetNavState(false, false)
		return
	}
	w.toolbar.SetNavState(
		active.History.CanGoBack(),
		active.History.CanGoForward(),
	)
}

func (w *Window) tabByID(id string) *Tab {
	for _, t := range w.tabs.Tabs() {
		if t.ID == id {
			return t
		}
	}
	return nil
}

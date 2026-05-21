// pkg/browser/webview.go
//go:build windows

// Package browser — WebView2 embed and browser chrome.
//
// Architecture (HTML chrome):
//
//	go-webview2 top-level window (frameless, subclassed WndProc)
//	└── WebView2 controller (fills entire client area)
//	    └── HTML chrome overlay (position:fixed, z-index max)
//	        — address bar, nav buttons, tab strip — all rendered as HTML
//
// The browser chrome (toolbar, tabs, address bar) is implemented as a
// position:fixed HTML overlay injected into every page via
// AddScriptToExecuteOnDocumentCreated.  Tab state is persisted in Go so it
// survives page navigations.  Window resize is handled via JS edge detection
// because WebView2's child HWND covers the entire frame, preventing the
// WndProc WM_NCHITTEST handler from receiving border mouse events.
package browser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"github.com/lxn/walk"
	"github.com/lxn/win"

	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/bridge"
)

// ── WebViewPanel ──────────────────────────────────────────────────────────────

// WebViewPanel wraps a go-webview2 WebView that occupies the entire Walk
// MainWindow client area.  The browser chrome (toolbar, tabs, address bar) is
// rendered inside WebView2 as a position:fixed HTML overlay so it is always
// visible and requires no Win32 child-window management.
type WebViewPanel struct {
	mainWindow *walk.MainWindow
	wv         webview2.WebView

	br          *bridge.Bridge
	log         *logging.Logger
	userDataDir string

	wndProcCb       uintptr // keeps subclassed WndProc callback alive (GC guard)
	tabsJSON        string  // JSON tab state persisted across navigations
	searchEngineURL string  // URL prefix for address-bar text searches

	// Feature stores wired to JS bindings.
	bookmarks    *BookmarkStore
	browsingHist *BrowsingHistoryStore
	dlMgr        *DownloadManager
	blocker      *Blocker

	// Callbacks set by Window after construction.
	OnTitleChange  func(title string)
	OnURLChange    func(url string)
	OnLoadStart    func(url string)
	OnLoadComplete func(url string)
	OnLoadError    func(url, errMsg string)
}

// ── data types for JS bindings ────────────────────────────────────────────────

type privacyData struct {
	ProfileName         string  `json:"profileName"`
	BlockedCount        int64   `json:"blockedCount"`
	UserAgent           string  `json:"userAgent"`
	Platform            string  `json:"platform"`
	Language            string  `json:"language"`
	Timezone            string  `json:"timezone"`
	HardwareConcurrency int     `json:"hardwareConcurrency"`
	DeviceMemory        float64 `json:"deviceMemory"`
	GPUVendor           string  `json:"gpuVendor"`
	GPURenderer         string  `json:"gpuRenderer"`
	CanvasSeed          int64   `json:"canvasSeed"`
	AudioSeed           int64   `json:"audioSeed"`
	WebGLSeed           int64   `json:"webglSeed"`
}

type profileDataFull struct {
	ProfileName         string   `json:"profileName"`
	UserAgent           string   `json:"userAgent"`
	Platform            string   `json:"platform"`
	Language            string   `json:"language"`
	Languages           []string `json:"languages"`
	Timezone            string   `json:"timezone"`
	HardwareConcurrency int      `json:"hardwareConcurrency"`
	DeviceMemory        float64  `json:"deviceMemory"`
	GPUVendor           string   `json:"gpuVendor"`
	GPURenderer         string   `json:"gpuRenderer"`
	CanvasSeed          int64    `json:"canvasSeed"`
	AudioSeed           int64    `json:"audioSeed"`
	WebGLSeed           int64    `json:"webglSeed"`
	FontSeed            int64    `json:"fontSeed"`
}

type profileListItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OS      string `json:"os"`
	Browser string `json:"browser"`
	Active  bool   `json:"active"`
}

// NewWebViewPanel creates WebView2 as a child of mw, filling the entire
// client area.  The browser chrome is injected as HTML.
func NewWebViewPanel(
	mw *walk.MainWindow,
	br *bridge.Bridge,
	userDataDir string,
	searchEngineURL string,
	log *logging.Logger,
	bookmarks *BookmarkStore,
	browsingHist *BrowsingHistoryStore,
	dlMgr *DownloadManager,
) (*WebViewPanel, error) {
	if searchEngineURL == "" {
		searchEngineURL = "https://duckduckgo.com/?q="
	}
	p := &WebViewPanel{
		mainWindow:      mw,
		br:              br,
		log:             log.WithComponent("webview"),
		userDataDir:     userDataDir,
		searchEngineURL: searchEngineURL,
		bookmarks:       bookmarks,
		browsingHist:    browsingHist,
		dlMgr:           dlMgr,
		blocker:         NewBlocker(),
	}

	hwnd := unsafe.Pointer(uintptr(mw.Handle()))
	wv := webview2.NewWithOptions(webview2.WebViewOptions{
		Window:    hwnd,
		Debug:     false,
		DataPath:  userDataDir,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Width:  defaultWindowW,
			Height: defaultWindowH,
			Center: true,
		},
	})
	if wv == nil {
		return nil, fmt.Errorf("webview2: failed to create instance — " +
			"ensure the WebView2 Runtime (Edge) is installed")
	}
	p.wv = wv
	p.log.Info("webview2 initialised")

	// ── Frameless chrome ──────────────────────────────────────────────────
	wvHWND := win.HWND(uintptr(p.wv.Window()))
	wvStyle := win.GetWindowLong(wvHWND, win.GWL_STYLE)
	win.SetWindowLong(wvHWND, win.GWL_STYLE, wvStyle&^win.WS_CAPTION|win.WS_CLIPCHILDREN)
	win.SetWindowPos(wvHWND, 0, 0, 0, 0, 0,
		win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)
	p.subclassFrameless(wvHWND)
	dwmapi := syscall.NewLazyDLL("dwmapi.dll")
	ncPolicy := uint32(2)
	dwmapi.NewProc("DwmSetWindowAttribute").Call(
		uintptr(wvHWND), 2, uintptr(unsafe.Pointer(&ncPolicy)), 4)
	cornerPref := uint32(2)
	dwmapi.NewProc("DwmSetWindowAttribute").Call(
		uintptr(wvHWND), 33, uintptr(unsafe.Pointer(&cornerPref)), 4)

	// ── Window management bindings ────────────────────────────────────────
	const swMaximize = 3

	p.wv.Bind("__ghostMinimize", func() {
		win.PostMessage(wvHWND, win.WM_SYSCOMMAND, win.SC_MINIMIZE, 0)
	})
	p.wv.Bind("__ghostMaximize", func() {
		var wp win.WINDOWPLACEMENT
		wp.Length = uint32(unsafe.Sizeof(wp))
		win.GetWindowPlacement(wvHWND, &wp)
		if wp.ShowCmd == swMaximize {
			win.PostMessage(wvHWND, win.WM_SYSCOMMAND, win.SC_RESTORE, 0)
		} else {
			win.PostMessage(wvHWND, win.WM_SYSCOMMAND, win.SC_MAXIMIZE, 0)
		}
	})
	p.wv.Bind("__ghostClose", func() {
		win.PostMessage(wvHWND, win.WM_CLOSE, 0, 0)
	})
	p.wv.Bind("__ghostStartDrag", func() {
		var pt win.POINT
		win.GetCursorPos(&pt)
		lp := uintptr(pt.Y)<<16 | uintptr(uint16(pt.X))
		win.ReleaseCapture()
		win.PostMessage(wvHWND, win.WM_NCLBUTTONDOWN, win.HTCAPTION, lp)
	})
	p.wv.Bind("__ghostIsMaximized", func() bool {
		var wp win.WINDOWPLACEMENT
		wp.Length = uint32(unsafe.Sizeof(wp))
		win.GetWindowPlacement(wvHWND, &wp)
		return wp.ShowCmd == swMaximize
	})

	// ── Debug helper (temporary) ──────────────────────────────────────────
	p.wv.Bind("__ghostDebug", func(msg string) {
		p.log.Info("JS-DEBUG: " + msg)
	})

	// ── Tab state ─────────────────────────────────────────────────────────
	// Load saved tab state from the profile directory (session restore).
	p.loadTabsJSON()

	// __ghostGetTabs is void: value-returning bindings don't resolve when using
	// Walk's mw.Run() instead of wv.Run(). Go pushes data to JS by calling
	// window.__ghostTabsCb (set by JS before invoking __ghostGetTabs) via
	// mainWindow.Synchronize + Eval, which is confirmed to work.
	p.wv.Bind("__ghostGetTabs", func() {
		data := p.tabsJSON
		p.mainWindow.Synchronize(func() {
			p.wv.Eval("if(window.__ghostTabsCb){var _f=window.__ghostTabsCb;window.__ghostTabsCb=null;_f(" + data + ");}")
		})
	})
	p.wv.Bind("__ghostSetTabs", func(j string) {
		p.tabsJSON = j
		go p.saveTabsJSON(j)
	})

	// ── Resize ────────────────────────────────────────────────────────────
	p.wv.Bind("__ghostStartResize", func(ht int) {
		var pt win.POINT
		win.GetCursorPos(&pt)
		lp := uintptr(pt.Y)<<16 | uintptr(uint16(pt.X))
		win.ReleaseCapture()
		win.PostMessage(wvHWND, win.WM_NCLBUTTONDOWN, uintptr(ht), lp)
	})

	// ── Bookmark bindings ─────────────────────────────────────────────────
	p.wv.Bind("__ghostAddBookmark", func(url, title string) {
		_, _ = p.bookmarks.Add(url, title)
	})
	p.wv.Bind("__ghostRemoveBookmark", func(url string) {
		_ = p.bookmarks.RemoveByURL(url)
	})
	p.wv.Bind("__ghostGetBookmarks", func() string {
		data, _ := json.Marshal(p.bookmarks.All())
		return string(data)
	})
	p.wv.Bind("__ghostIsBookmarked", func(url string) bool {
		return p.bookmarks.Has(url)
	})

	// ── History bindings ──────────────────────────────────────────────────
	p.wv.Bind("__ghostRecordHistory", func(url, title string) {
		p.browsingHist.Record(url, title)
	})
	p.wv.Bind("__ghostGetHistory", func() string {
		data, _ := json.Marshal(p.browsingHist.All())
		return string(data)
	})
	p.wv.Bind("__ghostDeleteHistoryEntry", func(url string) {
		p.browsingHist.DeleteByURL(url)
	})
	p.wv.Bind("__ghostClearHistory", func() {
		p.browsingHist.Clear()
	})

	// ── Download bindings ─────────────────────────────────────────────────
	p.wv.Bind("__ghostGetDownloads", func() string {
		data, _ := json.Marshal(p.dlMgr.All())
		return string(data)
	})
	p.wv.Bind("__ghostDownloadStarted", func(url, filename string, total int64) string {
		dir := filepath.Join(p.userDataDir, "Downloads")
		return p.dlMgr.Start(url, filename, filepath.Join(dir, filename), total)
	})
	p.wv.Bind("__ghostOpenFolder", func(path string) {
		if path == "" {
			return
		}
		_ = exec.Command("explorer", "/select,", path).Start()
	})

	// ── Blocker bindings ──────────────────────────────────────────────────
	p.wv.Bind("__ghostGetBlockedCount", func() int64 {
		return p.blocker.BlockedCount()
	})

	// ── Privacy bindings ──────────────────────────────────────────────────
	p.wv.Bind("__ghostGetPrivacyData", func() string {
		cfg := p.buildConfig()
		prof := p.br.Profile()
		d := privacyData{
			ProfileName:         prof.Name,
			BlockedCount:        p.blocker.BlockedCount(),
			UserAgent:           cfg.UserAgent,
			Platform:            cfg.Platform,
			Language:            cfg.Language,
			Timezone:            cfg.Timezone,
			HardwareConcurrency: cfg.HardwareConcurrency,
			DeviceMemory:        cfg.DeviceMemory,
			GPUVendor:           cfg.GPUVendor,
			GPURenderer:         cfg.GPURenderer,
			CanvasSeed:          cfg.CanvasSeed,
			AudioSeed:           cfg.AudioSeed,
			WebGLSeed:           cfg.WebGLSeed,
		}
		b, _ := json.Marshal(d)
		return string(b)
	})

	// ── Settings bindings ─────────────────────────────────────────────────
	p.wv.Bind("__ghostGetProfile", func() string {
		cfg := p.buildConfig()
		prof := p.br.Profile()
		d := profileDataFull{
			ProfileName:         prof.Name,
			UserAgent:           cfg.UserAgent,
			Platform:            cfg.Platform,
			Language:            cfg.Language,
			Languages:           cfg.Languages,
			Timezone:            cfg.Timezone,
			HardwareConcurrency: cfg.HardwareConcurrency,
			DeviceMemory:        cfg.DeviceMemory,
			GPUVendor:           cfg.GPUVendor,
			GPURenderer:         cfg.GPURenderer,
			CanvasSeed:          cfg.CanvasSeed,
			AudioSeed:           cfg.AudioSeed,
			WebGLSeed:           cfg.WebGLSeed,
			FontSeed:            cfg.FontSeed,
		}
		b, _ := json.Marshal(d)
		return string(b)
	})
	p.wv.Bind("__ghostSaveProfile", func(jsonStr string) {
		var cfg profileDataFull
		if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
			return
		}
		prof := *p.br.Profile()
		prof.Browser.UserAgent = cfg.UserAgent
		prof.Hardware.Platform = cfg.Platform
		prof.Network.Timezone = cfg.Timezone
		if cfg.HardwareConcurrency > 0 {
			prof.Hardware.CPUCores = cfg.HardwareConcurrency
		}
		if cfg.DeviceMemory > 0 {
			prof.Hardware.RAMMb = int(cfg.DeviceMemory * 1024)
		}
		if cfg.Language != "" {
			prof.Browser.Languages = []string{cfg.Language}
		}
		p.br.UpdateProfile(&prof)
		if err := p.injectPolyfill(); err != nil {
			p.log.Warn("polyfill re-injection after settings save failed", "error", err.Error())
		}
	})

	// ── Profile switcher bindings ─────────────────────────────────────────
	p.wv.Bind("__ghostListProfiles", func() string {
		prof := p.br.Profile()
		items := []profileListItem{{
			ID:      prof.ID,
			Name:    prof.Name,
			OS:      prof.Hardware.Platform,
			Browser: extractBrowserName(prof.Browser.UserAgent),
			Active:  true,
		}}
		b, _ := json.Marshal(items)
		return string(b)
	})
	p.wv.Bind("__ghostSwitchProfile", func(id string) {
		p.log.Info("ghost switch profile", "id", id)
	})

	// ── Navigate binding ──────────────────────────────────────────────────
	p.wv.Bind("__ghostNavigate", p.handleGhostScheme)
	// __ghostGoTo navigates to ghost:// pages from the Go side via
	// mainWindow.Synchronize+Navigate.  p.wv.Dispatch() is a no-op here
	// because p.wv.Run() is never called (Walk's mw.Run() drives the loop).
	// Synchronize posts onto Walk's loop, which runs on the same main thread
	// that created the WebView2 controller.
	// __ghostGoTo(url, tabsJSON) — tabsJSON is the current _T from JS, passed
	// so we never race against a pending __ghostSetTabs goroutine updating
	// p.tabsJSON.  JS always has the authoritative latest tab state.
	p.wv.Bind("__ghostGoTo", func(url, tabsJSON string) {
		if !strings.HasPrefix(url, "ghost://") {
			return
		}
		html := p.ghostPageHTML(url)
		if tabsJSON == "" {
			tabsJSON = p.tabsJSON
		}
		if tabsJSON != "" {
			script := `<script>window.__ghostInitTabs=` + tabsJSON + `;</script>`
			html = strings.Replace(html, "</head>", script+"</head>", 1)
		}
		dataURL := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))
		p.mainWindow.Synchronize(func() {
			p.wv.Navigate(dataURL)
		})
	})

	if err := p.injectPolyfill(); err != nil {
		p.log.Warn("polyfill injection failed", "error", err.Error())
	}
	p.injectChromeOverlay()
	p.injectResizeEdges()
	p.injectKeyboardShortcuts()
	p.bindEventBridge()

	return p, nil
}

// ── navigation ────────────────────────────────────────────────────────────────

func (p *WebViewPanel) Navigate(url string) {
	if p.wv == nil {
		return
	}
	p.log.Info("navigate", "url", url)
	if strings.HasPrefix(url, "ghost://") {
		p.wv.Navigate("data:text/html;base64," + p.ghostPageDataURL(url))
		return
	}
	p.wv.Navigate(url)
}

func (p *WebViewPanel) Reload() {
	if p.wv != nil {
		p.wv.Eval(`window.location.reload()`)
	}
}

func (p *WebViewPanel) Stop() {
	if p.wv != nil {
		p.wv.Eval(`window.stop()`)
	}
}

func (p *WebViewPanel) GoBack() {
	if p.wv != nil {
		p.wv.Eval(`window.history.back()`)
	}
}

func (p *WebViewPanel) GoForward() {
	if p.wv != nil {
		p.wv.Eval(`window.history.forward()`)
	}
}

func (p *WebViewPanel) SetZoom(factor float64) {
	if p.wv != nil {
		p.wv.Eval(fmt.Sprintf(`document.body.style.zoom=%f`, factor))
	}
}

func (p *WebViewPanel) Eval(js string) {
	if p.wv != nil {
		p.wv.Eval(js)
	}
}

func (p *WebViewPanel) WebView() webview2.WebView { return p.wv }

func (p *WebViewPanel) UpdateProfile() {
	if p.wv == nil {
		return
	}
	if err := p.injectPolyfill(); err != nil {
		p.log.Warn("polyfill re-injection failed", "error", err.Error())
	}
}

// ── HTML chrome overlay ───────────────────────────────────────────────────────

func (p *WebViewPanel) injectChromeOverlay() {
	profileName := p.br.Profile().Name

	script := fmt.Sprintf(`(function(){
'use strict';
try{if(window!==window.top)return;}catch(e){return;}
var H=82,profile=%q,_searchURL=%q;

/* ── Style element ──────────────────────────────────────────── */
var s=document.createElement('style');
s.id='_gs_st';
s.textContent=
  '#_gs_toolbar{position:fixed;top:0;left:0;right:0;height:'+H+'px;'+
  'background:linear-gradient(90deg,#3D1A0A 0%%,#2A1560 40%%,#0A1A6B 70%%,#050E40 100%%);'+
  'border-bottom:1px solid rgba(255,255,255,.1);z-index:2147483647;'+
  'display:flex;flex-direction:column;box-shadow:0 2px 12px rgba(0,0,0,.5)}'+
  '#_gs_tabs{height:38px;display:flex;align-items:flex-end;padding:0 0 0 8px;gap:0;'+
  'overflow:hidden;-webkit-app-region:drag}'+
  '#_gs_tablist{display:flex;align-items:flex-end;gap:2px;overflow:hidden;flex:1;min-width:0}'+
  '#_gs_new_tab,#_gs_wm_btns,._gs_wm_btn{-webkit-app-region:no-drag}'+
  '._gst{background:rgba(255,255,255,.10);border:1px solid rgba(255,255,255,.12);border-bottom:none;'+
  'border-radius:8px 8px 0 0;padding:0 4px 0 10px;height:30px;display:flex;align-items:center;'+
  'font-size:12px;color:rgba(255,255,255,.6);max-width:180px;min-width:80px;cursor:pointer;'+
  '-webkit-app-region:no-drag;flex-shrink:0;'+
  'transition:background .18s ease,color .18s ease,border-color .18s ease}'+
  '._gst._gst_a{background:rgba(255,255,255,.2);color:#fff;border-color:rgba(255,255,255,.28)}'+
  '._gst:hover:not(._gst_a){background:rgba(255,255,255,.15);color:rgba(255,255,255,.9)}'+
  '._gst>span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1}'+
  '._gst_x{background:transparent;border:none;color:rgba(255,255,255,.35);font-size:11px;'+
  'cursor:pointer;padding:0 3px;margin-left:2px;border-radius:3px;flex-shrink:0;line-height:1.5;'+
  'transition:background .15s ease,color .15s ease}'+
  '._gst_x:hover{background:rgba(255,255,255,.18);color:#fff}'+
  '._gst_fav{width:14px;height:14px;border-radius:2px;margin-right:4px;flex-shrink:0;object-fit:contain;'+
  'transition:opacity .15s ease}'+
  '#_gs_new_tab{background:transparent;border:none;color:rgba(255,255,255,.55);'+
  'font-size:20px;cursor:pointer;padding:0 8px;border-radius:50%%;align-self:center;line-height:1;flex-shrink:0;'+
  'transition:background .15s ease,color .15s ease,transform .12s ease}'+
  '#_gs_new_tab:hover{background:rgba(255,255,255,.14);color:#fff;transform:scale(1.12)}'+
  '#_gs_nav{height:44px;display:flex;align-items:center;padding:0 10px;gap:6px}'+
  '._gs_btn{width:28px;height:28px;border:none;background:transparent;border-radius:50%%;cursor:pointer;'+
  'font-size:16px;color:rgba(255,255,255,.7);display:flex;align-items:center;justify-content:center;'+
  'transition:background .15s ease,color .15s ease,transform .1s ease}'+
  '._gs_btn:hover{background:rgba(255,255,255,.14);color:#fff;transform:scale(1.1)}'+
  '._gs_btn:active{transform:scale(.93)}'+
  '#_gs_addr{flex:1;height:30px;border:1px solid rgba(255,255,255,.18);border-radius:15px;'+
  'padding:0 14px;font-size:13px;background:rgba(255,255,255,.10);color:#fff;outline:none;'+
  'transition:border-color .2s ease,background .2s ease,box-shadow .2s ease}'+
  '#_gs_addr::placeholder{color:rgba(255,255,255,.35)}'+
  '#_gs_addr:focus{border-color:rgba(130,150,255,.75);background:rgba(255,255,255,.16);'+
  'box-shadow:0 0 0 2px rgba(130,150,255,.18)}'+
  '#_gs_bm{width:28px;height:28px;border:none;background:transparent;border-radius:50%%;cursor:pointer;'+
  'font-size:16px;color:rgba(255,255,255,.4);display:flex;align-items:center;justify-content:center;flex-shrink:0;'+
  'transition:background .15s ease,color .15s ease,transform .12s ease}'+
  '#_gs_bm:hover{background:rgba(255,255,255,.13);color:#F4A460;transform:scale(1.1)}'+
  '#_gs_bm._bm_on{color:#F4A460}'+
  '#_gs_badge{min-width:28px;height:22px;border-radius:11px;background:rgba(244,164,96,.10);'+
  'border:1px solid rgba(244,164,96,.22);color:rgba(244,164,96,.65);font-size:11px;font-weight:600;'+
  'display:flex;align-items:center;justify-content:center;padding:0 5px;cursor:pointer;'+
  'flex-shrink:0;user-select:none;transition:background .15s ease,color .15s ease}'+
  '#_gs_badge:hover{background:rgba(244,164,96,.2);color:#F4A460}'+
  '#_gs_wm_btns{display:flex;align-items:stretch;margin-left:auto;height:38px;-webkit-app-region:no-drag}'+
  '._gs_wm_btn{width:46px;height:100%%;border:none;background:transparent;'+
  'color:rgba(255,255,255,.8);font-size:13px;cursor:pointer;'+
  'display:flex;align-items:center;justify-content:center;'+
  'transition:background .15s ease,color .15s ease}'+
  '._gs_wm_btn:hover{background:rgba(255,255,255,.18)}'+
  '#_gs_cls:hover{background:#E81123!important;color:#fff}'+
  'body{padding-top:'+H+'px!important}';

/* ── Toolbar element ────────────────────────────────────────── */
var bar=document.createElement('div');
bar.id='_gs_toolbar';
bar.innerHTML=
  '<div id="_gs_tabs"><div id="_gs_tablist">'+
  '<div class="_gst _gst_a"><span id="_gs_tab_title">Loading…</span>'+
  '<button class="_gst_x" style="display:none">&#10005;</button></div>'+
  '</div>'+
  '<button id="_gs_new_tab" title="New Tab (Ctrl+T)">+</button>'+
  '<div id="_gs_wm_btns">'+
  '<button class="_gs_wm_btn" id="_gs_min" title="Minimise">&#8212;</button>'+
  '<button class="_gs_wm_btn" id="_gs_max" title="Maximise">&#9633;</button>'+
  '<button class="_gs_wm_btn" id="_gs_cls" title="Close">&#10005;</button>'+
  '</div></div>'+
  '<div id="_gs_nav">'+
  '<button class="_gs_btn" id="_gs_back" title="Back">&#8592;</button>'+
  '<button class="_gs_btn" id="_gs_fwd" title="Forward">&#8594;</button>'+
  '<button class="_gs_btn" id="_gs_reload" title="Reload">&#8635;</button>'+
  '<input id="_gs_addr" type="text" spellcheck="false" placeholder="Search or enter address"/>'+
  '<button id="_gs_bm" title="Bookmark (Ctrl+D)">☆</button>'+
  '<div id="_gs_badge" title="Blocked trackers">0</div>'+
  '</div>';

bar.style.cssText=
  'position:fixed;top:0;left:0;right:0;'+
  'height:'+H+'px;z-index:2147483647;'+
  'display:flex;flex-direction:column;'+
  'background:linear-gradient(90deg,#3D1A0A 0%%,#2A1560 40%%,#0A1A6B 70%%,#050E40 100%%);'+
  'border-bottom:1px solid rgba(255,255,255,.1);box-shadow:0 2px 12px rgba(0,0,0,.5)';

/* ── Direct element refs ────────────────────────────────────── */
var _tablist=bar.querySelector('#_gs_tablist');
var _addr=bar.querySelector('#_gs_addr');
var _tabTitle=bar.querySelector('#_gs_tab_title');
var _bmEl=bar.querySelector('#_gs_bm');
var _badgeEl=bar.querySelector('#_gs_badge');

/* ── Tab state ──────────────────────────────────────────────── */
var _T={tabs:[],current:0};var _loaded=false;var _loadQ=[];
function _url(){return window.__ghostPageURL||location.href;}
function _load(cb){
  if(_loaded){if(cb)cb();return;}
  if(cb)_loadQ.push(cb);
  if(_loadQ.length>1)return;
  // Fast path: Go pre-injected tab state into the ghost:// page HTML.
  if(window.__ghostInitTabs&&window.__ghostInitTabs.tabs&&window.__ghostInitTabs.tabs.length){
    _T=window.__ghostInitTabs;
    _loaded=true;var q0=_loadQ.splice(0);q0.forEach(function(f){try{f();}catch(_){}});
    return;
  }
  // Slow path: regular web page — ask Go via callback.
  window.__ghostTabsCb=function(obj){
    _T=(obj&&obj.tabs&&obj.tabs.length)?obj:{tabs:[{id:1,url:_url(),title:'New Tab'}],current:1};
    _loaded=true;var q=_loadQ.splice(0);q.forEach(function(f){try{f();}catch(_){}});
  };
  try{__ghostGetTabs();}catch(e){
    window.__ghostTabsCb=null;
    if(!_T||!_T.tabs||!_T.tabs.length){_T={tabs:[{id:1,url:_url(),title:'New Tab'}],current:1};}
    _loaded=true;var q2=_loadQ.splice(0);q2.forEach(function(f){try{f();}catch(_){}});
  }
}
function _save(){try{__ghostSetTabs(JSON.stringify(_T));}catch(_){}}
function _render(){
  if(!_T.tabs||!_T.tabs.length)return;
  var kids=_tablist.children;
  if(kids.length===_T.tabs.length){
    // Fast path: same number of tabs — update attributes in place, no DOM rebuild.
    for(var i=0;i<_T.tabs.length;i++){
      var t=_T.tabs[i];var el=kids[i];
      var active=t.id===_T.current;
      var wantCls=active?'_gst _gst_a':'_gst';
      if(el.className!==wantCls)el.className=wantCls;
      var sp=el.querySelector('span');var ttl=t.title||'New Tab';
      if(sp&&sp.textContent!==ttl){sp.textContent=ttl;sp.title=ttl;}
      var fav=el.querySelector('._gst_fav');
      if(fav){
        var http=t.url&&(t.url.startsWith('http://')||t.url.startsWith('https://'));
        if(http){try{var h=new URL(t.url).hostname;
          if(fav.getAttribute('data-h')!==h){
            fav.setAttribute('data-h',h);
            fav.src='https://www.google.com/s2/favicons?domain='+h+'&sz=16';
            fav.style.display='';
            fav.onerror=function(){this.style.display='none';};
          }
        }catch(_){fav.style.display='none';}}
        else{fav.style.display='none';}
      }
    }
    return;
  }
  // Full rebuild — only runs when tab count changes (open/close).
  _tablist.innerHTML='';
  _T.tabs.forEach(function(t){
    var el=document.createElement('div');
    el.className=t.id===_T.current?'_gst _gst_a':'_gst';
    var fav=document.createElement('img');fav.className='_gst_fav';
    if(t.url&&(t.url.startsWith('http://')||t.url.startsWith('https://'))){
      try{var h=new URL(t.url).hostname;
        fav.src='https://www.google.com/s2/favicons?domain='+h+'&sz=16';
        fav.setAttribute('data-h',h);
        fav.onerror=function(){fav.style.display='none';};
      }catch(_){fav.style.display='none';}
    }else{fav.style.display='none';}
    var sp=document.createElement('span');sp.textContent=t.title||'New Tab';sp.title=t.title||'';
    var xb=document.createElement('button');xb.className='_gst_x';xb.innerHTML='&#10005;';xb.title='Close';
    el.appendChild(fav);el.appendChild(sp);el.appendChild(xb);
    (function(id){
      el.addEventListener('click',function(e){if(xb.contains(e.target))return;_switchTab(id);});
      xb.addEventListener('click',function(e){e.stopPropagation();_closeTab(id);});
    })(t.id);
    _tablist.appendChild(el);
  });
}
function _updateCur(){
  var u=_url(),ti=document.title||location.hostname||'New Tab';
  for(var i=0;i<_T.tabs.length;i++){
    if(_T.tabs[i].id===_T.current){
      _T.tabs[i].url=u;_T.tabs[i].title=ti;
      break;
    }
  }
}
/* ── Navigation helper: ghost:// goes via Go NavigateToString, http via location ─ */
function _ghostGoTo(url){
  if(url.startsWith('ghost://')){
    try{__ghostGoTo(url,JSON.stringify(_T));}catch(e){console.error('[ghost] __ghostGoTo threw:',e);}
  }else{location.href=url;}
}
function _switchTab(id){
  _updateCur();_T.current=id;_save();_render();
  for(var i=0;i<_T.tabs.length;i++){
    if(_T.tabs[i].id===id){
      if(_T.tabs[i].url===_url())return;
      _ghostGoTo(_T.tabs[i].url);return;
    }
  }
}
window._gsNewTab=function(){
  console.log('[ghost] _gsNewTab _loaded='+_loaded+' tabs='+((_T&&_T.tabs)?_T.tabs.length:0));
  try{__ghostDebug('_gsNewTab loaded='+_loaded+' tabs='+((_T&&_T.tabs)?_T.tabs.length:0));}catch(_){}
  if(!_loaded){_load(function(){window._gsNewTab();});return;}
  try{__ghostDebug('_gsNewTab proceeding to create tab');}catch(_){}
  _updateCur();
  var mx=0;_T.tabs.forEach(function(t){if(t.id>mx)mx=t.id;});
  var id=mx+1;_T.tabs.push({id:id,url:'ghost://newtab',title:'New Tab'});
  _T.current=id;_save();_render();
  _ghostGoTo('ghost://newtab');
};
window._gsCloseCurrentTab=function(){_closeTab(_T.current);};
function _closeTab(id){
  if(_T.tabs.length<=1){try{__ghostClose();}catch(_){}return;}
  var idx=-1;for(var i=0;i<_T.tabs.length;i++){if(_T.tabs[i].id===id){idx=i;break;}}
  if(idx<0)return;
  var wa=(id===_T.current);
  _T.tabs.splice(idx,1);
  if(wa){var ni=Math.min(idx,_T.tabs.length-1);_T.current=_T.tabs[ni].id;_save();_render();_ghostGoTo(_T.tabs[ni].url);}
  else{_save();_render();}
}

/* ── Window-button event listeners ─────────────────────────── */
function _mx(){
  var b=bar.querySelector('#_gs_max');if(!b)return;
  try{__ghostIsMaximized().then(function(m){
    b.innerHTML=m?'&#10064;':'&#9633;';b.title=m?'Restore':'Maximise';
  });}catch(_){}
}
_mx();
window.addEventListener('resize',_mx);
bar.querySelector('#_gs_tabs').addEventListener('mousedown',function(e){
  if(e.button===0&&!e.target.closest('button,input,a,._gst')){try{__ghostStartDrag();}catch(_){}}
});
bar.querySelector('#_gs_back').addEventListener('click',function(){history.back();});
bar.querySelector('#_gs_fwd').addEventListener('click',function(){history.forward();});
bar.querySelector('#_gs_reload').addEventListener('click',function(){location.reload();});
bar.querySelector('#_gs_new_tab').addEventListener('click',function(){
  try{__ghostDebug('new-tab-btn-click');}catch(_){}
  _gsNewTab();
});
bar.querySelector('#_gs_min').addEventListener('click',function(){try{__ghostMinimize();}catch(e){}});
bar.querySelector('#_gs_max').addEventListener('click',function(){_mx();try{__ghostMaximize();}catch(e){}});
bar.querySelector('#_gs_cls').addEventListener('click',function(){try{__ghostClose();}catch(e){}});
_addr.addEventListener('keydown',function(e){
  if(e.key!=='Enter')return;e.preventDefault();
  var r=_addr.value.trim();if(!r)return;var u=r;
  if(!u.match(/^https?:\/\//i)&&!u.startsWith('ghost://')){
    if(u.indexOf('.')>=0&&u.indexOf(' ')<0){u='https://'+u;}
    else{u=_searchURL+encodeURIComponent(u);}
  }
  _ghostGoTo(u);
});
_addr.addEventListener('focus',function(){_addr.select();});

/* ── Bookmarks ──────────────────────────────────────────────── */
function _bmUpdate(){
  var u=_url();
  if(!u||u.startsWith('ghost://')||u.startsWith('data:')){
    _bmEl.textContent='☆';_bmEl.classList.remove('_bm_on');
    _bmEl.style.opacity='0.25';_bmEl.title='Cannot bookmark internal pages';return;
  }
  _bmEl.style.opacity='';_bmEl.title='Bookmark (Ctrl+D)';
  try{__ghostIsBookmarked(u).then(function(yes){
    _bmEl.textContent=yes?'★':'☆';
    if(yes)_bmEl.classList.add('_bm_on');else _bmEl.classList.remove('_bm_on');
  });}catch(_){}
}
function _bmToggle(){
  var u=_url(),ti=document.title||'';
  console.log('[ghost] _bmToggle url='+u);
  if(!u||u.startsWith('ghost://')||u.startsWith('data:'))return;
  if(_bmEl.classList.contains('_bm_on')){
    _bmEl.textContent='☆';_bmEl.classList.remove('_bm_on');
    try{__ghostRemoveBookmark(u);}catch(_){}
  }else{
    _bmEl.textContent='★';_bmEl.classList.add('_bm_on');
    try{__ghostAddBookmark(u,ti);}catch(_){}
  }
}
_bmEl.addEventListener('click',_bmToggle);
window._gsBmToggle=_bmToggle;

/* ── Blocked count badge ────────────────────────────────────── */
function _badgeUpdate(){
  try{__ghostGetBlockedCount().then(function(n){
    _badgeEl.textContent=n||0;
    _badgeEl.title='Blocked trackers: '+(n||0);
  });}catch(_){}
}
_badgeEl.addEventListener('click',function(){console.log('[ghost] badge clicked');_ghostGoTo('ghost://privacy');});
setInterval(_badgeUpdate,2000);

/* ── Title watcher ──────────────────────────────────────────── */
var _titleWatched=false;
function _watchTitle(){
  if(_titleWatched)return;_titleWatched=true;
  var t=document.querySelector('title');
  if(t){new MutationObserver(function(){
    // Only update the tab title — never the URL.
    // Calling _updateCur() here would corrupt the target tab's URL when
    // _T.current was already switched but the old page's title fires late.
    var ti=document.title||location.hostname||'New Tab';
    for(var i=0;i<_T.tabs.length;i++){
      if(_T.tabs[i].id===_T.current){_T.tabs[i].title=ti;break;}
    }
    _render();
  }).observe(t,{childList:true,characterData:true,subtree:true});}
}

/* ── Find bar (created lazily on first Ctrl+F) ──────────────── */
var _fb=null;
function _ensureFb(){
  if(_fb&&document.getElementById('_gs_find'))return;
  _fb=document.createElement('div');
  _fb.id='_gs_find';
  _fb.style.cssText=
    'position:fixed;bottom:0;right:0;z-index:2147483646;'+
    'background:rgba(20,20,50,.95);border:1px solid rgba(255,255,255,.15);'+
    'border-radius:8px 8px 0 0;padding:8px 10px;'+
    'gap:6px;align-items:center;box-shadow:0 -2px 12px rgba(0,0,0,.5)';
  _fb.style.display='none';
  var btnCss='background:transparent;border:1px solid rgba(255,255,255,.2);'+
    'color:rgba(255,255,255,.8);font-size:12px;padding:4px 8px;border-radius:4px;cursor:pointer';
  _fb.innerHTML=
    '<input id="_gs_fi" autocomplete="off" placeholder="Find…" '+
    'style="border:1px solid rgba(255,255,255,.2);border-radius:6px;'+
    'background:rgba(255,255,255,.08);color:#fff;font-size:13px;'+
    'padding:4px 10px;outline:none;width:200px">'+
    '<button id="_gs_fp" title="Previous" style="'+btnCss+'">↑</button>'+
    '<button id="_gs_fn" title="Next" style="'+btnCss+'">↓</button>'+
    '<button id="_gs_fx" title="Close" style="'+btnCss+'">✕</button>';
  try{document.documentElement.appendChild(_fb);}catch(_){_fb=null;return;}
  var _fi=document.getElementById('_gs_fi');
  _fi.addEventListener('input',function(){
    if(_fi.value)window.find(_fi.value,false,false,true,false,false,false);
  });
  _fi.addEventListener('keydown',function(e){
    if(e.key==='Enter'){e.preventDefault();window.find(_fi.value,false,e.shiftKey,true,false,false,false);}
    if(e.key==='Escape'){_fb.style.display='none';}
  });
  document.getElementById('_gs_fp').addEventListener('click',function(){
    var fi=document.getElementById('_gs_fi');
    if(fi)window.find(fi.value,false,true,true,false,false,false);
  });
  document.getElementById('_gs_fn').addEventListener('click',function(){
    var fi=document.getElementById('_gs_fi');
    if(fi)window.find(fi.value,false,false,true,false,false,false);
  });
  document.getElementById('_gs_fx').addEventListener('click',function(){_fb.style.display='none';});
}
function _openFind(){
  _ensureFb();
  if(!_fb)return;
  _fb.style.display='flex';
  setTimeout(function(){var fi=document.getElementById('_gs_fi');if(fi){fi.focus();fi.select();}},50);
}
window._gsOpenFind=_openFind;

/* ── Context menu ───────────────────────────────────────────── */
var _cm=null;
function _ensureCm(){
  if(_cm&&document.getElementById('_gs_ctx'))return;
  _cm=document.createElement('div');
  _cm.id='_gs_ctx';
  _cm.style.cssText=
    'position:fixed;z-index:2147483646;'+
    'background:rgba(20,20,50,.97);border:1px solid rgba(255,255,255,.12);'+
    'border-radius:8px;padding:4px;min-width:160px;'+
    'box-shadow:0 4px 20px rgba(0,0,0,.6);font-family:Segoe UI,system-ui,sans-serif';
  _cm.style.display='none';
  try{document.documentElement.appendChild(_cm);}catch(_){_cm=null;}
}
function _ctxItem(label,fn){
  var d=document.createElement('div');
  d.style.cssText='padding:7px 14px;font-size:13px;color:#E8E8F4;cursor:pointer;border-radius:4px';
  d.textContent=label;
  d.addEventListener('mouseenter',function(){d.style.background='rgba(255,255,255,.1)';});
  d.addEventListener('mouseleave',function(){d.style.background='';});
  d.addEventListener('click',function(){if(_cm)_cm.style.display='none';fn();});
  return d;
}
function _ctxSep(){
  var d=document.createElement('div');
  d.style.cssText='height:1px;background:rgba(255,255,255,.1);margin:3px 6px';
  return d;
}
function _showCtx(e){
  _ensureCm();if(!_cm)return;
  _cm.innerHTML='';
  var tgt=e.target;
  var link=tgt.closest('a[href]');
  var img=tgt.closest('img');
  var sel=window.getSelection?window.getSelection().toString().trim():'';
  _cm.appendChild(_ctxItem('Back',function(){history.back();}));
  _cm.appendChild(_ctxItem('Forward',function(){history.forward();}));
  _cm.appendChild(_ctxItem('Reload',function(){location.reload();}));
  if(link||img||sel)_cm.appendChild(_ctxSep());
  if(link){
    (function(href){
      _cm.appendChild(_ctxItem('Open in new tab',function(){
        window._gsNewTab&&window._gsNewTab();
        setTimeout(function(){location.href=href;},50);
      }));
      _cm.appendChild(_ctxItem('Copy link',function(){try{navigator.clipboard.writeText(href);}catch(_){}}));
    })(link.href);
  }
  if(img){
    (function(src){
      _cm.appendChild(_ctxItem('Open image in new tab',function(){
        window._gsNewTab&&window._gsNewTab();
        setTimeout(function(){location.href=src;},50);
      }));
      _cm.appendChild(_ctxItem('Copy image URL',function(){try{navigator.clipboard.writeText(src);}catch(_){}}));
    })(img.src);
  }
  if(sel){
    var short=sel.length>30?sel.slice(0,30)+'…':sel;
    (function(q){
      _cm.appendChild(_ctxItem('Search “'+short+'”',function(){
        location.href=_searchURL+encodeURIComponent(q);
      }));
    })(sel);
  }
  var x=e.clientX,y=e.clientY;
  _cm.style.left=x+'px';_cm.style.top=y+'px';_cm.style.display='block';
  var rect=_cm.getBoundingClientRect();
  if(rect.right>window.innerWidth)_cm.style.left=(x-rect.width)+'px';
  if(rect.bottom>window.innerHeight)_cm.style.top=(y-rect.height)+'px';
}
document.addEventListener('contextmenu',function(e){
  e.preventDefault();
  if(e.target.closest('#_gs_toolbar'))return;
  _showCtx(e);
},true);
document.addEventListener('click',function(e){
  if(_cm&&!_cm.contains(e.target))_cm.style.display='none';
},true);
document.addEventListener('keydown',function(e){
  if(e.key==='Escape'){
    if(_cm)_cm.style.display='none';
    if(_fb)_fb.style.display='none';
  }
},true);

/* ── Download click intercept ───────────────────────────────── */
document.addEventListener('click',function(e){
  var a=e.target.closest('a[download]');
  if(!a||!a.href)return;
  var fname=a.download||a.href.split('/').pop()||'download';
  try{__ghostDownloadStarted(a.href,fname,0);}catch(_){}
},true);

/* ── Mount: DOM insertion + one-time state init ─────────────── */
var _ready=false;
function mount(){
  if(!document.documentElement)return;
  if(!document.getElementById('_gs_st')){
    try{if(document.head)document.head.appendChild(s);}catch(_){}
  }
  bar.style.setProperty('position','fixed','important');
  bar.style.setProperty('top','0','important');
  bar.style.setProperty('left','0','important');
  bar.style.setProperty('right','0','important');
  bar.style.setProperty('height',H+'px','important');
  bar.style.setProperty('z-index','2147483647','important');
  bar.style.setProperty('display','flex','important');
  bar.style.setProperty('flex-direction','column','important');
  try{
    if(document.body)document.body.style.setProperty('padding-top',H+'px','important');
  }catch(_){}
  if(!document.getElementById('_gs_toolbar')){
    try{document.documentElement.appendChild(bar);}catch(_){return;}
    _addr.value=_url();
    if(!_ready){
      _ready=true;
      if(_tabTitle)_tabTitle.textContent=document.title||location.hostname||_url();
      _load(function(){_updateCur();_render();_save();});
      _watchTitle();
    }else{
      _render();
    }
  }
}

window.addEventListener('load',function(){
  mount();
  _addr.value=_url();
  _load(function(){_updateCur();_render();_save();});
  try{
    var u=_url();
    if(u&&!u.startsWith('data:')&&!u.startsWith('about:'))
      __ghostRecordHistory(u,document.title||'');
  }catch(_){}
  _bmUpdate();
  _badgeUpdate();
});
window.addEventListener('popstate',function(){_addr.value=_url();_updateCur();_save();});

if(document.readyState==='loading'){document.addEventListener('DOMContentLoaded',mount);}
else{mount();}

// Only re-mount if the toolbar was removed by the page; never re-render on a timer.
setInterval(function(){if(!document.getElementById('_gs_toolbar'))mount();},500);
})();`, profileName, p.searchEngineURL)

	p.wv.Init(script)
}

// injectResizeEdges injects a JS listener that detects cursor proximity to
// viewport edges and calls __ghostStartResize.
func (p *WebViewPanel) injectResizeEdges() {
	p.wv.Init(`(function(){
'use strict';
try{if(window!==window.top)return;}catch(e){return;}
var HT={l:10,r:11,t:12,tl:13,tr:14,b:15,bl:16,br:17};
var E=8;
var CS={10:'w-resize',11:'e-resize',12:'n-resize',13:'nw-resize',14:'ne-resize',15:'s-resize',16:'sw-resize',17:'se-resize'};
function getHT(e){
  var x=e.clientX,y=e.clientY,w=window.innerWidth,h=window.innerHeight;
  var L=x<E,R=x>=w-E,T=y<E,B=y>=h-E;
  if(L&&T)return HT.tl;if(R&&T)return HT.tr;
  if(L&&B)return HT.bl;if(R&&B)return HT.br;
  if(T)return HT.t;if(B)return HT.b;
  if(L)return HT.l;if(R)return HT.r;
  return 0;
}
document.addEventListener('mousemove',function(e){
  document.documentElement.style.cursor=CS[getHT(e)]||'';
},true);
document.addEventListener('mousedown',function(e){
  if(e.button!==0)return;
  var h=getHT(e);
  if(h){e.preventDefault();e.stopPropagation();try{__ghostStartResize(h);}catch(_){}}
},true);
})();`)
}

func (p *WebViewPanel) injectKeyboardShortcuts() {
	p.wv.Init(`(function(){
'use strict';
try{if(window!==window.top)return;}catch(e){return;}
window.addEventListener('keydown',function(e){
  if(!e.ctrlKey)return;
  switch(e.key){
    case 't':case 'T':
      e.preventDefault();
      if(typeof _gsNewTab==='function'){_gsNewTab();}else{location.href='ghost://newtab';}
      break;
    case 'w':case 'W':
      e.preventDefault();
      if(typeof _gsCloseCurrentTab==='function'){_gsCloseCurrentTab();}else{try{__ghostClose();}catch(_){}}
      break;
    case 'l':case 'L':
      e.preventDefault();
      var a=document.getElementById('_gs_addr');
      if(a){a.focus();a.select();}
      break;
    case 'r':case 'R':
      e.preventDefault();location.reload();break;
    case 'd':case 'D':
      e.preventDefault();
      if(typeof window._gsBmToggle==='function')window._gsBmToggle();
      break;
    case 'f':case 'F':
      e.preventDefault();
      if(typeof window._gsOpenFind==='function')window._gsOpenFind();
      break;
  }
});
})();`)
}

// ghostPageDataURL builds a ghost:// page, injects the current tab state as
// window.__ghostInitTabs so _load() can restore it synchronously without the
// async __ghostGetTabs round-trip, then returns a base64 data URL.
func (p *WebViewPanel) ghostPageDataURL(url string) string {
	html := p.ghostPageHTML(url)
	if p.tabsJSON != "" {
		script := `<script>window.__ghostInitTabs=` + p.tabsJSON + `;</script>`
		html = strings.Replace(html, "</head>", script+"</head>", 1)
	}
	return base64.StdEncoding.EncodeToString([]byte(html))
}

// ghostPageHTML returns fully self-contained HTML for ghost:// internal URLs.
func (p *WebViewPanel) ghostPageHTML(url string) string {
	page := strings.TrimPrefix(url, "ghost://")

	switch page {
	case "bookmarks":
		return p.ghostBookmarksPage()
	case "history":
		return p.ghostHistoryPage()
	case "downloads":
		return p.ghostDownloadsPage()
	case "privacy":
		return p.ghostPrivacyPage()
	case "profiles":
		return p.ghostProfilesPage()
	case "settings":
		return p.ghostSettingsPage()
	}

	var body string
	switch page {
	case "newtab", "":
		body = "<h2>Ghost-Silicon</h2><p style=\"margin-top:10px;opacity:.7\">Type an address above and press Enter.</p>"
	case "network":
		body = "<h2>Network Monitor</h2><p>Coming soon.</p>"
	case "audit":
		body = "<h2>Audit Log</h2><p>Coming soon.</p>"
	default:
		body = "<h2>" + page + "</h2><p>Page not found.</p>"
	}

	const tmpl = `<!DOCTYPE html><html><head>
<meta charset="utf-8"><title>ghost://GSPAGE</title>
<script>window.__ghostPageURL='ghost://GSPAGE';</script>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Segoe UI',system-ui,sans-serif;
  background:linear-gradient(160deg,#1a0a0a 0%,#0d0d30 50%,#050a28 100%);
  color:#E8E8F4;height:100vh;overflow:hidden}
#content{min-height:100vh;display:flex;align-items:center;
  justify-content:center;text-align:center;padding:20px}
#content h2{font-size:2rem;font-weight:700;
  background:linear-gradient(90deg,#F4A460,#A080FF);
  -webkit-background-clip:text;-webkit-text-fill-color:transparent}
</style></head><body>
<div id="content">GSBODY</div>
</body></html>`

	html := strings.ReplaceAll(tmpl, "GSPAGE", page)
	html = strings.ReplaceAll(html, "GSBODY", body)
	return html
}

// loadTabsJSON reads the saved tab state from the profile directory.
// Silently no-ops if the file does not exist (first launch with this profile).
func (p *WebViewPanel) loadTabsJSON() {
	if p.userDataDir == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(p.userDataDir, "tabs.json"))
	if err == nil && len(data) > 0 {
		p.tabsJSON = string(data)
	}
}

// saveTabsJSON atomically writes the tab state to the profile directory.
func (p *WebViewPanel) saveTabsJSON(j string) {
	if p.userDataDir == "" {
		return
	}
	path := filepath.Join(p.userDataDir, "tabs.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(j), 0o600); err == nil {
		_ = os.Rename(tmp, path)
	}
}

// InitialURL returns the URL the browser should navigate to on startup.
// For session restore: returns the saved current tab's URL if it is a regular
// web URL; falls back to ghost://newtab otherwise.
func (p *WebViewPanel) InitialURL() string {
	var state struct {
		Tabs []struct {
			ID  int    `json:"id"`
			URL string `json:"url"`
		} `json:"tabs"`
		Current int `json:"current"`
	}
	if err := json.Unmarshal([]byte(p.tabsJSON), &state); err != nil {
		return defaultHomeURL
	}
	for _, t := range state.Tabs {
		if t.ID == state.Current {
			if t.URL == "" || strings.HasPrefix(t.URL, "ghost://") || strings.HasPrefix(t.URL, "data:") {
				return defaultHomeURL
			}
			return t.URL
		}
	}
	return defaultHomeURL
}

func ghostPageBase(page, extraCSS, bodyContent string) string {
	return `<!DOCTYPE html><html><head>
<meta charset="utf-8"><title>ghost://` + page + `</title>
<script>window.__ghostPageURL='ghost://` + page + `';</script>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Segoe UI',system-ui,sans-serif;background:linear-gradient(160deg,#1a0a0a 0%,#0d0d30 50%,#050a28 100%);color:#E8E8F4;min-height:100vh;padding-top:90px}
.page{max-width:800px;margin:0 auto;padding:24px}
h1{font-size:1.8rem;font-weight:700;background:linear-gradient(90deg,#F4A460,#A080FF);-webkit-background-clip:text;-webkit-text-fill-color:transparent;margin-bottom:20px}
` + extraCSS + `
</style></head><body>
<div class="page">
` + bodyContent + `
</div>
</body></html>`
}

func (p *WebViewPanel) ghostBookmarksPage() string {
	css := `.toolbar{display:flex;gap:10px;margin-bottom:16px}
#q{flex:1;padding:10px 16px;border:1px solid rgba(255,255,255,.2);border-radius:8px;background:rgba(255,255,255,.08);color:#fff;font-size:14px;outline:none}
#q:focus{border-color:rgba(130,150,255,.7)}
.bm{display:flex;align-items:center;gap:10px;padding:10px 14px;border-radius:8px;background:rgba(255,255,255,.05);margin-bottom:6px}
.bm:hover{background:rgba(255,255,255,.09)}
.bm-info{flex:1;min-width:0;cursor:pointer}
.bm-title{font-size:14px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.bm-url{font-size:11px;color:rgba(255,255,255,.4);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;margin-top:2px}
.bm-del{background:transparent;border:none;color:rgba(255,255,255,.3);font-size:14px;cursor:pointer;padding:4px 6px;border-radius:4px}
.bm-del:hover{background:rgba(220,30,30,.3);color:#fff}
.empty{text-align:center;opacity:.4;padding:40px;font-size:14px}`
	body := `<h1>Bookmarks</h1>
<div class="toolbar"><input id="q" placeholder="Search bookmarks&#8230;" autocomplete="off"/></div>
<div id="list"></div>
<script>
var _all=[];
function esc(s){var d=document.createElement('div');d.textContent=s;return d.innerHTML;}
function render(){
  var q=document.getElementById('q').value.toLowerCase();
  var items=q?_all.filter(function(b){return(b.title||'').toLowerCase().includes(q)||(b.url||'').toLowerCase().includes(q);}):_all;
  var l=document.getElementById('list');
  if(!items.length){l.innerHTML='<div class="empty">'+(q?'No results.':'No bookmarks yet.')+'</div>';return;}
  l.innerHTML='';
  items.forEach(function(b){
    var d=document.createElement('div');d.className='bm';
    var info=document.createElement('div');info.className='bm-info';
    info.innerHTML='<div class="bm-title">'+esc(b.title||b.url)+'</div><div class="bm-url">'+esc(b.url)+'</div>';
    info.onclick=function(){location.href=b.url;};
    var del=document.createElement('button');del.className='bm-del';del.textContent='✕';del.title='Remove';
    del.onclick=function(){__ghostRemoveBookmark(b.url).then(refresh);};
    d.appendChild(info);d.appendChild(del);l.appendChild(d);
  });
}
function refresh(){__ghostGetBookmarks().then(function(j){try{_all=JSON.parse(j)||[];}catch(_){_all=[];}render();});}
document.getElementById('q').addEventListener('input',render);
refresh();
</script>`
	return ghostPageBase("bookmarks", css, body)
}

func (p *WebViewPanel) ghostHistoryPage() string {
	css := `.toolbar{display:flex;gap:10px;margin-bottom:16px;align-items:center}
#q{flex:1;padding:10px 16px;border:1px solid rgba(255,255,255,.2);border-radius:8px;background:rgba(255,255,255,.08);color:#fff;font-size:14px;outline:none}
#q:focus{border-color:rgba(130,150,255,.7)}
.btn{padding:8px 16px;border:1px solid rgba(255,255,255,.2);border-radius:8px;background:rgba(255,255,255,.08);color:#ccc;font-size:13px;cursor:pointer;white-space:nowrap}
.btn:hover{background:rgba(255,255,255,.15);color:#fff}
.date-grp{font-size:11px;text-transform:uppercase;letter-spacing:.06em;color:rgba(255,255,255,.4);margin:16px 0 6px}
.he{display:flex;align-items:center;gap:8px;padding:7px 10px;border-radius:6px}
.he:hover{background:rgba(255,255,255,.07)}
.he-time{font-size:11px;color:rgba(255,255,255,.3);flex-shrink:0;width:50px;text-align:right}
.he-info{flex:1;min-width:0;cursor:pointer}
.he-title{font-size:13px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.he-url{font-size:11px;color:rgba(255,255,255,.4);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.he-del{background:transparent;border:none;color:rgba(255,255,255,.25);font-size:11px;cursor:pointer;padding:4px 6px;border-radius:4px;flex-shrink:0;line-height:1}
.he-del:hover{background:rgba(220,30,30,.35);color:#fff}
.empty{text-align:center;opacity:.4;padding:40px;font-size:14px}`
	body := `<h1>History</h1>
<div class="toolbar">
<input id="q" placeholder="Search history&#8230;" autocomplete="off"/>
<button class="btn" id="clr">Clear All</button>
</div>
<div id="list"></div>
<script>
var _all=[];
function esc(s){var d=document.createElement('div');d.textContent=s;return d.innerHTML;}
function fmt(t){var d=new Date(t);return d.toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'});}
function fmtDate(t){return new Date(t).toLocaleDateString([],{weekday:'long',month:'long',day:'numeric'});}
function render(){
  var q=document.getElementById('q').value.toLowerCase();
  var items=q?_all.filter(function(e){return(e.title||'').toLowerCase().includes(q)||(e.url||'').toLowerCase().includes(q);}):_all;
  var l=document.getElementById('list');
  if(!items.length){l.innerHTML='<div class="empty">'+(q?'No results.':'No history yet.')+'</div>';return;}
  l.innerHTML='';var lastDay='';
  items.forEach(function(e){
    var day=new Date(e.visited_at).toDateString();
    if(day!==lastDay){lastDay=day;var g=document.createElement('div');g.className='date-grp';g.textContent=fmtDate(e.visited_at);l.appendChild(g);}
    var d=document.createElement('div');d.className='he';
    var info=document.createElement('div');info.className='he-info';
    info.innerHTML='<div class="he-title">'+esc(e.title||e.url)+'</div><div class="he-url">'+esc(e.url)+'</div>';
    var del=document.createElement('button');del.className='he-del';del.textContent='✕';del.title='Remove';
    d.innerHTML='<span class="he-time">'+esc(fmt(e.visited_at))+'</span>';
    d.appendChild(info);d.appendChild(del);
    (function(entry,row,btn){
      info.onclick=function(){location.href=entry.url;};
      btn.onclick=function(ev){ev.stopPropagation();row.remove();try{__ghostDeleteHistoryEntry(entry.url);}catch(_){}};
    })(e,d,del);
    l.appendChild(d);
  });
}
document.getElementById('q').addEventListener('input',render);
document.getElementById('clr').onclick=function(){
  if(!confirm('Clear all history?'))return;
  __ghostClearHistory().then(refresh);
};
function refresh(){__ghostGetHistory().then(function(j){try{_all=JSON.parse(j)||[];}catch(_){_all=[];}render();});}
refresh();
</script>`
	return ghostPageBase("history", css, body)
}

func (p *WebViewPanel) ghostDownloadsPage() string {
	css := `.dl{background:rgba(255,255,255,.05);border-radius:8px;padding:14px 16px;margin-bottom:8px}
.dl-name{font-size:14px;font-weight:600;margin-bottom:4px}
.dl-url{font-size:11px;color:rgba(255,255,255,.4);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;margin-bottom:8px}
.dl-bar{height:4px;background:rgba(255,255,255,.1);border-radius:2px;overflow:hidden;margin-bottom:6px}
.dl-fill{height:100%;background:linear-gradient(90deg,#A080FF,#F4A460);border-radius:2px;transition:.3s}
.dl-foot{font-size:12px;color:rgba(255,255,255,.5);display:flex;justify-content:space-between;align-items:center}
.open-btn{background:transparent;border:1px solid rgba(255,255,255,.2);color:#ccc;font-size:11px;padding:3px 8px;border-radius:4px;cursor:pointer}
.open-btn:hover{background:rgba(255,255,255,.1);color:#fff}
.empty{text-align:center;opacity:.4;padding:40px;font-size:14px}`
	body := `<h1>Downloads</h1>
<div id="list"></div>
<script>
var states={0:'Downloading…',1:'Complete',2:'Failed',3:'Cancelled'};
function esc(s){var d=document.createElement('div');d.textContent=s;return d.innerHTML;}
function render(items){
  var l=document.getElementById('list');
  if(!items||!items.length){l.innerHTML='<div class="empty">No downloads yet.</div>';return;}
  l.innerHTML='';
  items.forEach(function(it){
    var pct=it.TotalBytes>0?Math.round(it.RecvBytes/it.TotalBytes*100):0;
    var name=it.Filename||(it.URL?it.URL.split('/').pop():'')||'download';
    var d=document.createElement('div');d.className='dl';
    d.innerHTML='<div class="dl-name">'+esc(name)+'</div>'+
      '<div class="dl-url">'+esc(it.URL||'')+'</div>'+
      '<div class="dl-bar"><div class="dl-fill" style="width:'+pct+'%"></div></div>'+
      '<div class="dl-foot"><span>'+esc(states[it.State]||'Unknown')+
      (it.State===0&&it.TotalBytes>0?' — '+pct+'%':'')+
      '</span>'+
      (it.State===1&&it.Destination?'<button class="open-btn" data-path="'+esc(it.Destination)+'">Open folder</button>':'')+
      '</div>';
    l.appendChild(d);
  });
  l.querySelectorAll('.open-btn').forEach(function(btn){
    btn.onclick=function(){__ghostOpenFolder(btn.dataset.path);};
  });
}
function refresh(){__ghostGetDownloads().then(function(j){try{render(JSON.parse(j));}catch(_){render([]); }});}
refresh();setInterval(refresh,1500);
</script>`
	return ghostPageBase("downloads", css, body)
}

func (p *WebViewPanel) ghostPrivacyPage() string {
	css := `.subtitle{font-size:13px;color:rgba(255,255,255,.45);margin-top:-14px;margin-bottom:24px}
.card{background:rgba(255,255,255,.05);border-radius:10px;padding:16px 20px;margin-bottom:16px}
.card h2{font-size:13px;font-weight:600;color:#A080FF;text-transform:uppercase;letter-spacing:.05em;margin-bottom:12px}
table{width:100%;border-collapse:collapse}
td{padding:5px 0;font-size:13px;vertical-align:top}
td:first-child{color:rgba(255,255,255,.5);width:180px;flex-shrink:0}
td:last-child{font-family:monospace;font-size:12px;word-break:break-all;color:#E8E8F4}
.big-n{font-size:2.5rem;font-weight:700;color:#A080FF;margin:4px 0}
.big-lbl{font-size:12px;color:rgba(255,255,255,.4)}`
	body := `<h1>Privacy Dashboard</h1>
<p class="subtitle" id="pname">Loading&#8230;</p>
<div class="card"><h2>Blocked Trackers</h2>
<div class="big-n" id="bcount">0</div>
<div class="big-lbl">requests blocked this session</div></div>
<div class="card"><h2>Spoofed Identity</h2><table id="id-tbl"></table></div>
<div class="card"><h2>Hardware Fingerprint</h2><table id="hw-tbl"></table></div>
<div class="card"><h2>Noise Seeds</h2><table id="ns-tbl"></table></div>
<script>
function row(k,v){return '<tr><td>'+k+'</td><td>'+v+'</td></tr>';}
function hex16(n){return '0x'+(n>>>0).toString(16).padStart(8,'0');}
function refresh(){
  __ghostGetPrivacyData().then(function(j){
    var d=JSON.parse(j);
    document.getElementById('pname').textContent='Profile: '+d.profileName;
    document.getElementById('bcount').textContent=d.blockedCount||0;
    document.getElementById('id-tbl').innerHTML=
      row('User Agent',d.userAgent)+row('Platform',d.platform)+
      row('Language',d.language)+row('Timezone',d.timezone);
    document.getElementById('hw-tbl').innerHTML=
      row('CPU Cores',d.hardwareConcurrency)+row('Device Memory',d.deviceMemory+' GB')+
      row('GPU Vendor',d.gpuVendor)+row('GPU Renderer',d.gpuRenderer);
    document.getElementById('ns-tbl').innerHTML=
      row('Canvas Seed',hex16(d.canvasSeed))+
      row('Audio Seed',hex16(d.audioSeed))+
      row('WebGL Seed',hex16(d.webglSeed));
  });
}
refresh();setInterval(refresh,2000);
</script>`
	return ghostPageBase("privacy", css, body)
}

func (p *WebViewPanel) ghostProfilesPage() string {
	css := `.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:12px}
.pcard{background:rgba(255,255,255,.05);border:2px solid transparent;border-radius:10px;padding:20px;cursor:pointer;transition:.15s}
.pcard:hover{background:rgba(255,255,255,.09);border-color:rgba(160,128,255,.4)}
.pcard.active{border-color:#A080FF;background:rgba(160,128,255,.1)}
.pcard-name{font-size:15px;font-weight:600;margin-bottom:6px}
.pcard-badge{font-size:11px;color:rgba(255,255,255,.4);background:rgba(255,255,255,.07);padding:2px 8px;border-radius:10px;display:inline-block}
.active-tag{font-size:10px;color:#A080FF;font-weight:600;text-transform:uppercase;letter-spacing:.05em;margin-top:8px}`
	body := `<h1>Profiles</h1>
<div class="grid" id="grid"></div>
<script>
function refresh(){
  __ghostListProfiles().then(function(j){
    var profiles=JSON.parse(j);
    var g=document.getElementById('grid');g.innerHTML='';
    profiles.forEach(function(p){
      var d=document.createElement('div');
      d.className='pcard'+(p.active?' active':'');
      d.innerHTML='<div class="pcard-name">'+p.name+'</div>'+
        '<span class="pcard-badge">'+p.os+' / '+p.browser+'</span>'+
        (p.active?'<div class="active-tag">Active</div>':'');
      if(!p.active)d.onclick=function(){__ghostSwitchProfile(p.id).then(function(){location.reload();});};
      g.appendChild(d);
    });
  });
}
refresh();
</script>`
	return ghostPageBase("profiles", css, body)
}

func (p *WebViewPanel) ghostSettingsPage() string {
	css := `.prof-name{font-size:13px;color:rgba(255,255,255,.45);margin-top:-14px;margin-bottom:24px}
.section{background:rgba(255,255,255,.05);border-radius:10px;padding:16px 20px;margin-bottom:16px}
.section h2{font-size:13px;font-weight:600;color:#A080FF;text-transform:uppercase;letter-spacing:.05em;margin-bottom:14px}
.field{display:flex;align-items:center;margin-bottom:10px}
.field label{width:160px;font-size:13px;color:rgba(255,255,255,.6);flex-shrink:0}
.field input,.field select{flex:1;padding:7px 12px;border:1px solid rgba(255,255,255,.15);border-radius:6px;background:rgba(255,255,255,.07);color:#fff;font-size:13px;outline:none}
.field input:focus,.field select:focus{border-color:rgba(130,150,255,.7)}
.seed-val{font-family:monospace;font-size:12px;color:rgba(255,255,255,.5);flex:1}
.save-row{display:flex;justify-content:flex-end;margin-top:8px}
.save-btn{padding:10px 28px;background:linear-gradient(90deg,#6A40FF,#A080FF);border:none;color:#fff;font-size:14px;font-weight:600;border-radius:8px;cursor:pointer}
.save-btn:hover{opacity:.85}
#toast{position:fixed;bottom:24px;left:50%;transform:translateX(-50%%);background:rgba(160,128,255,.9);color:#fff;padding:10px 24px;border-radius:8px;font-size:13px;display:none;z-index:999}`
	body := `<h1>Settings</h1>
<p class="prof-name" id="prof-name"></p>
<div class="section"><h2>Identity</h2>
<div class="field"><label>User Agent</label><input id="f-ua" type="text"/></div>
<div class="field"><label>Platform</label><input id="f-plat" type="text"/></div>
<div class="field"><label>Language</label><input id="f-lang" type="text"/></div>
</div>
<div class="section"><h2>Hardware</h2>
<div class="field"><label>CPU Cores</label><input id="f-cpu" type="number" min="1" max="64"/></div>
<div class="field"><label>Device Memory</label>
<select id="f-ram"><option>0.25</option><option>0.5</option><option>1</option><option>2</option><option>4</option><option>8</option></select></div>
</div>
<div class="section"><h2>Network</h2>
<div class="field"><label>Timezone</label><input id="f-tz" type="text"/></div>
</div>
<div class="section"><h2>Noise Seeds (read-only)</h2>
<div class="field"><label>Canvas</label><span class="seed-val" id="s-canvas"></span></div>
<div class="field"><label>Audio</label><span class="seed-val" id="s-audio"></span></div>
<div class="field"><label>WebGL</label><span class="seed-val" id="s-webgl"></span></div>
</div>
<div class="save-row"><button class="save-btn" id="save-btn">Save Settings</button></div>
<div id="toast">Settings saved!</div>
<script>
var _p={};
function hex16(n){return '0x'+(n>>>0).toString(16).padStart(8,'0');}
function load(){
  __ghostGetProfile().then(function(j){
    _p=JSON.parse(j);
    document.getElementById('prof-name').textContent='Profile: '+(_p.profileName||'');
    document.getElementById('f-ua').value=_p.userAgent||'';
    document.getElementById('f-plat').value=_p.platform||'';
    document.getElementById('f-lang').value=_p.language||'';
    document.getElementById('f-cpu').value=_p.hardwareConcurrency||4;
    document.getElementById('f-ram').value=String(_p.deviceMemory||4);
    document.getElementById('f-tz').value=_p.timezone||'';
    document.getElementById('s-canvas').textContent=hex16(_p.canvasSeed||0);
    document.getElementById('s-audio').textContent=hex16(_p.audioSeed||0);
    document.getElementById('s-webgl').textContent=hex16(_p.webglSeed||0);
  });
}
document.getElementById('save-btn').onclick=function(){
  var updated=Object.assign({},_p,{
    userAgent:document.getElementById('f-ua').value,
    platform:document.getElementById('f-plat').value,
    language:document.getElementById('f-lang').value,
    hardwareConcurrency:parseInt(document.getElementById('f-cpu').value)||4,
    deviceMemory:parseFloat(document.getElementById('f-ram').value)||4,
    timezone:document.getElementById('f-tz').value,
  });
  __ghostSaveProfile(JSON.stringify(updated)).then(function(){
    var t=document.getElementById('toast');t.style.display='block';
    setTimeout(function(){t.style.display='none';},2500);
  });
};
load();
</script>`
	return ghostPageBase("settings", css, body)
}

// ── event bridge ─────────────────────────────────────────────────────────────

func (p *WebViewPanel) bindEventBridge() {
	p.wv.Bind("__ghostOnNavStart", func(url string) {
		if p.OnLoadStart != nil {
			p.OnLoadStart(url)
		}
	})
	p.wv.Bind("__ghostOnNavDone", func(url string) {
		if p.OnLoadComplete != nil {
			p.OnLoadComplete(url)
		}
	})
	p.wv.Bind("__ghostOnTitle", func(title string) {
		if p.OnTitleChange != nil {
			p.OnTitleChange(title)
		}
	})
	p.wv.Bind("__ghostOnURL", func(url string) {
		if p.OnURLChange != nil {
			p.OnURLChange(url)
		}
	})
	p.wv.Bind("__ghostOnError", func(url, msg string) {
		if p.OnLoadError != nil {
			p.OnLoadError(url, msg)
		}
	})

	p.wv.Init(`(function(){
'use strict';
if(window.navigation){
  window.navigation.addEventListener('navigate',function(e){
    try{__ghostOnNavStart(e.destination.url)}catch(_){}
  });
  window.navigation.addEventListener('navigatesuccess',function(){
    var u=window.location.href;
    try{__ghostOnNavDone(u)}catch(_){}
    try{__ghostOnURL(u)}catch(_){}
  });
}
window.addEventListener('load',function(){
  var u=window.location.href;
  try{__ghostOnNavDone(u)}catch(_){}
  try{__ghostOnURL(u)}catch(_){}
});
function watchTitle(){
  var t=document.querySelector('title');
  if(!t)return;
  try{__ghostOnTitle(document.title)}catch(_){}
  new MutationObserver(function(){try{__ghostOnTitle(document.title)}catch(_){}})
    .observe(t,{childList:true,characterData:true,subtree:true});
}
if(document.readyState==='loading'){
  document.addEventListener('DOMContentLoaded',watchTitle);
}else{
  watchTitle();
}
['pushState','replaceState'].forEach(function(m){
  var o=history[m];
  history[m]=function(){
    o.apply(this,arguments);
    try{__ghostOnURL(window.location.href)}catch(_){}
  };
});
window.addEventListener('popstate',function(){
  try{__ghostOnURL(window.location.href)}catch(_){}
});
})();`)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func extractBrowserName(ua string) string {
	switch {
	case strings.Contains(ua, "Chrome"):
		return "Chrome"
	case strings.Contains(ua, "Firefox"):
		return "Firefox"
	case strings.Contains(ua, "Safari"):
		return "Safari"
	default:
		return "Browser"
	}
}

// ── polyfill ──────────────────────────────────────────────────────────────────

type profileConfig struct {
	UserAgent           string   `json:"userAgent"`
	AppVersion          string   `json:"appVersion"`
	Vendor              string   `json:"vendor"`
	VendorSub           string   `json:"vendorSub"`
	Product             string   `json:"product"`
	ProductSub          string   `json:"productSub"`
	Languages           []string `json:"languages"`
	Language            string   `json:"language"`
	DoNotTrack          string   `json:"doNotTrack"`
	CookieEnabled       bool     `json:"cookieEnabled"`
	HardwareConcurrency int      `json:"hardwareConcurrency"`
	DeviceMemory        float64  `json:"deviceMemory"`
	MaxTouchPoints      int      `json:"maxTouchPoints"`
	Platform            string   `json:"platform"`
	ScreenWidth         int      `json:"screenWidth"`
	ScreenHeight        int      `json:"screenHeight"`
	ScreenAvailWidth    int      `json:"screenAvailWidth"`
	ScreenAvailHeight   int      `json:"screenAvailHeight"`
	ColorDepth          int      `json:"colorDepth"`
	PixelDepth          int      `json:"pixelDepth"`
	DevicePixelRatio    float64  `json:"devicePixelRatio"`
	CanvasSeed          int64    `json:"canvasSeed"`
	AudioSeed           int64    `json:"audioSeed"`
	WebGLSeed           int64    `json:"webglSeed"`
	FontSeed            int64    `json:"fontSeed"`
	GPUVendor           string   `json:"gpuVendor"`
	GPURenderer         string   `json:"gpuRenderer"`
	Timezone            string   `json:"timezone"`
}

func (p *WebViewPanel) buildConfig() *profileConfig {
	prof := p.br.Profile()
	hw := &prof.Hardware
	br := &prof.Browser
	sc := &prof.Screen
	no := &prof.Noise
	langs := br.Languages
	if len(langs) == 0 {
		langs = []string{"en-US", "en"}
	}
	return &profileConfig{
		UserAgent:           br.UserAgent,
		AppVersion:          br.AppVersion,
		Vendor:              br.Vendor,
		VendorSub:           br.VendorSub,
		Product:             br.Product,
		ProductSub:          br.ProductSub,
		Languages:           langs,
		Language:            br.PrimaryLanguage(),
		DoNotTrack:          br.DoNotTrack,
		CookieEnabled:       br.CookieEnabled,
		HardwareConcurrency: hw.CPUCores,
		DeviceMemory:        hw.DeviceMemoryGB(),
		MaxTouchPoints:      hw.MaxTouchPoints,
		Platform:            hw.Platform,
		ScreenWidth:         sc.Width,
		ScreenHeight:        sc.Height,
		ScreenAvailWidth:    sc.AvailWidth,
		ScreenAvailHeight:   sc.AvailHeight,
		ColorDepth:          sc.ColorDepth,
		PixelDepth:          sc.PixelDepth,
		DevicePixelRatio:    sc.DevicePixelRatio,
		CanvasSeed:          no.CanvasSeed,
		AudioSeed:           no.AudioSeed,
		WebGLSeed:           no.WebGLSeed,
		FontSeed:            no.FontSeed,
		GPUVendor:           hw.GPUVendor,
		GPURenderer:         hw.GPURenderer,
		Timezone:            prof.Network.Timezone,
	}
}

func (p *WebViewPanel) injectPolyfill() error {
	cfg := p.buildConfig()
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal profile config: %w", err)
	}
	polyfill := fmt.Sprintf(`(function(){
'use strict';
const _ghost=Object.freeze(%s);
Object.defineProperty(window,'__ghost',{value:_ghost,writable:false});
function def(obj,prop,val){try{Object.defineProperty(obj,prop,{get:function(){return val;},configurable:false,enumerable:true});}catch(e){}}
var nav=window.navigator;
def(nav,'userAgent',_ghost.userAgent);def(nav,'appVersion',_ghost.appVersion);
def(nav,'vendor',_ghost.vendor);def(nav,'vendorSub',_ghost.vendorSub);
def(nav,'product',_ghost.product);def(nav,'productSub',_ghost.productSub);
def(nav,'languages',Object.freeze(_ghost.languages.slice()));
def(nav,'language',_ghost.language);def(nav,'doNotTrack',_ghost.doNotTrack||null);
def(nav,'cookieEnabled',_ghost.cookieEnabled);
def(nav,'hardwareConcurrency',_ghost.hardwareConcurrency);
def(nav,'deviceMemory',_ghost.deviceMemory);
def(nav,'maxTouchPoints',_ghost.maxTouchPoints);
def(nav,'platform',_ghost.platform);
var scr=window.screen;
def(scr,'width',_ghost.screenWidth);def(scr,'height',_ghost.screenHeight);
def(scr,'availWidth',_ghost.screenAvailWidth);def(scr,'availHeight',_ghost.screenAvailHeight);
def(scr,'colorDepth',_ghost.colorDepth);def(scr,'pixelDepth',_ghost.pixelDepth);
def(window,'devicePixelRatio',_ghost.devicePixelRatio);
if(_ghost.canvasSeed!==0){var oTDU=HTMLCanvasElement.prototype.toDataURL;var oGID=CanvasRenderingContext2D.prototype.getImageData;var _s=_ghost.canvasSeed;function lcg(s){return((s*1664525+1013904223)&0xFFFFFFFF)>>>0;}HTMLCanvasElement.prototype.toDataURL=function(type,quality){var ctx=this.getContext('2d');if(ctx){var id=oGID.call(ctx,0,0,this.width,this.height);var s=_s;for(var i=0;i<id.data.length;i+=4){s=lcg(s);id.data[i]^=(s&0x01);id.data[i+1]^=((s>>1)&0x01);id.data[i+2]^=((s>>2)&0x01);}ctx.putImageData(id,0,0);}return oTDU.call(this,type,quality);};}
var oGP=WebGLRenderingContext.prototype.getParameter;WebGLRenderingContext.prototype.getParameter=function(p){var e=this.getExtension('WEBGL_debug_renderer_info');if(e){if(p===e.UNMASKED_VENDOR_WEBGL)return _ghost.gpuVendor;if(p===e.UNMASKED_RENDERER_WEBGL)return _ghost.gpuRenderer;}return oGP.call(this,p);};
if(_ghost.timezone){var oRO=Intl.DateTimeFormat.prototype.resolvedOptions;Intl.DateTimeFormat.prototype.resolvedOptions=function(){var o=oRO.call(this);o.timeZone=_ghost.timezone;return o;};}
})();`, string(cfgJSON))
	p.wv.Init(polyfill)
	p.log.Info("polyfill injected", "profile_id", p.br.Profile().ID)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── frameless WndProc subclass ────────────────────────────────────────────────

func (p *WebViewPanel) subclassFrameless(hwnd win.HWND) {
	const (
		gwlpWndProc   = ^uintptr(3)
		wmNcCalcSize  = uintptr(0x0083)
		wmNcHitTest   = uintptr(0x0084)
		htClient      = uintptr(1)
		htLeft        = uintptr(10)
		htRight       = uintptr(11)
		htTop         = uintptr(12)
		htTopLeft     = uintptr(13)
		htTopRight    = uintptr(14)
		htBottom      = uintptr(15)
		htBottomLeft  = uintptr(16)
		htBottomRight = uintptr(17)
		resizeBorder  = int32(8)
	)

	user32 := syscall.NewLazyDLL("user32.dll")
	getWndLongPtr := user32.NewProc("GetWindowLongPtrW")
	setWndLongPtr := user32.NewProc("SetWindowLongPtrW")
	callWndProc := user32.NewProc("CallWindowProcW")

	origProc, _, _ := getWndLongPtr.Call(uintptr(hwnd), gwlpWndProc)

	cb := syscall.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
		switch msg {
		case wmNcCalcSize:
			if wp != 0 {
				return 0
			}
		case wmNcHitTest:
			var wr win.RECT
			win.GetWindowRect(win.HWND(h), &wr)
			x := int32(int16(lp & 0xFFFF))
			y := int32(int16((lp >> 16) & 0xFFFF))
			cx := x - wr.Left
			cy := y - wr.Top
			w := wr.Right - wr.Left
			ht := wr.Bottom - wr.Top
			rb := resizeBorder
			switch {
			case cx < rb && cy < rb:
				return htTopLeft
			case cx >= w-rb && cy < rb:
				return htTopRight
			case cx < rb && cy >= ht-rb:
				return htBottomLeft
			case cx >= w-rb && cy >= ht-rb:
				return htBottomRight
			case cy < rb:
				return htTop
			case cy >= ht-rb:
				return htBottom
			case cx < rb:
				return htLeft
			case cx >= w-rb:
				return htRight
			default:
				return htClient
			}
		}
		r, _, _ := callWndProc.Call(origProc, h, msg, wp, lp)
		return r
	})

	p.wndProcCb = cb
	setWndLongPtr.Call(uintptr(hwnd), gwlpWndProc, cb)
}

// ── resize ────────────────────────────────────────────────────────────────────

func (p *WebViewPanel) onResize() {
	if p.wv == nil || p.mainWindow == nil {
		return
	}
	b := p.mainWindow.ClientBounds()
	if b.Width > 0 && b.Height > 0 {
		p.log.Info("webview2 resize", "w", b.Width, "h", b.Height)
		p.wv.SetSize(b.Width, b.Height, webview2.HintNone)
	}
}

func (p *WebViewPanel) ForceResize() { p.onResize() }

// ── ghost:// scheme ───────────────────────────────────────────────────────────

func (p *WebViewPanel) handleGhostScheme(url string) string {
	return fmt.Sprintf(`<p>ghost: %s</p>`, url)
}

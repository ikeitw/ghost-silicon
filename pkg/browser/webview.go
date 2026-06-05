//go:build windows

// Package browser вЂ" WebView2 embed and browser chrome.
//
// Architecture (HTML chrome):
//
//	go-webview2 top-level window (frameless, subclassed WndProc)
//	в""в"Ђв"Ђ WebView2 controller (fills entire client area)
//	    в""в"Ђв"Ђ HTML chrome overlay (position:fixed, z-index max)
//	        вЂ" address bar, nav buttons, tab strip вЂ" all rendered as HTML
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
	"reflect"
	"strings"
	"syscall"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/pkg/edge"
	"github.com/lxn/walk"
	"github.com/lxn/win"

	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/bridge"
	"ghost-silicon/pkg/identity"
)

// wvBrowserSlot mirrors the first 4 words of go-webview2's unexported webview
// struct (commit dc24628cff85) so we can access the browser interface field.
// Layout: hwnd uintptr | mainthread uintptr | browser.itab uintptr | browser.data uintptr
type wvBrowserSlot struct {
	_    uintptr // hwnd
	_    uintptr // mainthread
	_    uintptr // browser interface: type/itab pointer
	data uintptr // browser interface: data pointer в†’ *edge.Chromium
}

// extractChromium reads the *edge.Chromium that go-webview2 stores inside the
// unexported webview.browser interface field.  Safe only for pinned go-webview2
// commit dc24628cff85 вЂ" the struct layout is stable for that revision.
func extractChromium(wv webview2.WebView) *edge.Chromium {
	slot := (*wvBrowserSlot)(unsafe.Pointer(reflect.ValueOf(wv).Pointer()))
	if slot.data == 0 {
		return nil
	}
	return (*edge.Chromium)(unsafe.Pointer(slot.data))
}

// openDevToolsVtblIdx is the 0-based index of OpenDevToolsWindow in
// ICoreWebView2's vtable, counted from iCoreWebView2Vtbl in go-webview2
// commit dc24628cff85: 3 IUnknown slots + 48 ICoreWebView2 slots before it.
const openDevToolsVtblIdx = uintptr(51)

// openDevToolsWindow calls ICoreWebView2.OpenDevToolsWindow via direct vtable
// invocation.  Synthetic DOM F12 events are not intercepted by WebView2 at the
// browser level, so this is the only reliable way to open DevTools from code.
// Must be called on the UI/WebView2 thread.
func openDevToolsWindow(c *edge.Chromium) {
	if c == nil {
		return
	}
	// Locate edge.Chromium's unexported webview *ICoreWebView2 field by name
	// so the offset is resolved at runtime rather than hard-coded.
	t := reflect.TypeOf(*c)
	var wvOff uintptr
	found := false
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Name == "webview" {
			wvOff = t.Field(i).Offset
			found = true
			break
		}
	}
	if !found {
		return
	}
	wvPtr := *(*uintptr)(unsafe.Pointer(uintptr(unsafe.Pointer(c)) + wvOff))
	if wvPtr == 0 {
		return
	}
	// vtable_ptr is the first word of the COM object; each slot is 8 bytes.
	vtbl := *(*uintptr)(unsafe.Pointer(wvPtr))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + openDevToolsVtblIdx*8))
	syscall.SyscallN(fn, wvPtr)
}

// toolbarH is the pixel height of the dedicated toolbar WebView2 window.
// Content WebView2 rendering always starts at y=toolbarH.
const toolbarH = 82

// в"Ђв"Ђ WebViewPanel в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ

// WebViewPanel wraps a go-webview2 WebView that occupies the entire Walk
// MainWindow client area.  The browser chrome (toolbar, tabs, address bar) is
// rendered in a separate toolbar WebView2 pinned above the content WebView2,
// isolating it from page JavaScript that could hide or break a position:fixed overlay.
type WebViewPanel struct {
	mainWindow *walk.MainWindow
	wv         webview2.WebView

	br          *bridge.Bridge
	log         *logging.Logger
	userDataDir string

	wvHWND          win.HWND // top-level window created by go-webview2
	toolbarWv       webview2.WebView
	toolbarHWND     win.HWND // toolbar child window reparented into wvHWND
	curURL          string   // last URL reported by the content WebView2
	devToolsHWND    win.HWND // non-zero once DevTools is reparented into wvHWND
	devToolsPending bool     // true while the goroutine is waiting for the DevTools HWND

	wndProcCb       uintptr // keeps subclassed WndProc callback alive (GC guard)
	tabsJSON        string  // JSON tab state persisted across navigations
	searchEngineURL string  // URL prefix for address-bar text searches

	// Feature stores wired to JS bindings.
	bookmarks     *BookmarkStore
	browsingHist  *BrowsingHistoryStore
	dlMgr         *DownloadManager
	blocker       *Blocker
	netLog        *NetworkLog
	auditLog      *AuditLog
	identityStore identity.Store
	rotState      *identity.RotationState
	rotPolicy     identity.RotationPolicy

	// Callbacks set by Window after construction.
	OnTitleChange  func(title string)
	OnURLChange    func(url string)
	OnLoadStart    func(url string)
	OnLoadComplete func(url string)
	OnLoadError    func(url, errMsg string)
}

// в"Ђв"Ђ data types for JS bindings в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ

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
	identityStore identity.Store,
	rotPolicy *identity.RotationPolicy,
) (*WebViewPanel, error) {
	if searchEngineURL == "" {
		searchEngineURL = "https://duckduckgo.com/?q="
	}
	var rp identity.RotationPolicy
	if rotPolicy != nil {
		rp = *rotPolicy
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
		netLog:          newNetworkLog(),
		auditLog:        newAuditLog(),
		identityStore:   identityStore,
		rotPolicy:       rp,
	}
	// Set up rotation state if a non-never policy was given.
	if rotPolicy != nil && rotPolicy.Trigger != identity.RotationNever {
		if rs, err := identity.NewRotationState(br.Profile(), *rotPolicy); err == nil {
			p.rotState = rs
			if rotPolicy.Trigger == identity.RotationOnSession {
				if newProf, rotated := rs.NotifyNewSession(); rotated {
					br.UpdateProfile(newProf)
					p.auditLog.record("rotation", "session rotation → "+newProf.Name)
				}
			} else if rotPolicy.Trigger == identity.RotationOnInterval {
				go func() {
					ticker := time.NewTicker(10 * time.Second)
					defer ticker.Stop()
					for range ticker.C {
						if newProf, rotated := rs.CheckInterval(); rotated {
							mw.Synchronize(func() {
								br.UpdateProfile(newProf)
								_ = p.injectPolyfill()
								p.auditLog.record("rotation", "interval rotation → "+newProf.Name)
							})
						}
					}
				}()
			}
		}
	}
	p.auditLog.record("start", "browser started with profile "+br.Profile().Name)

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
		return nil, fmt.Errorf("webview2: failed to create instance - ensure the WebView2 Runtime (Edge) is installed")
	}
	p.wv = wv
	p.log.Info("webview2 initialised")

	// в"Ђв"Ђ Ad/tracker blocker в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	// Wire Blocker.ShouldBlock to WebResourceRequested so the badge counter
	// reflects real blocked requests and resources are suppressed.
	if c := extractChromium(p.wv); c != nil {
		blocker := p.blocker
		nl := p.netLog
		al := p.auditLog
		c.WebResourceRequestedCallback = func(
			req *edge.ICoreWebView2WebResourceRequest,
			args *edge.ICoreWebView2WebResourceRequestedEventArgs,
		) {
			uri, err := req.GetUri()
			if err != nil {
				return
			}
			host := extractHost(uri)
			blocked := blocker.ShouldBlock(uri)
			nl.record(host, uri, blocked)
			if blocked {
				al.record("blocked", host)
				if env := c.Environment(); env != nil {
					if resp, e2 := env.CreateWebResourceResponse(nil, 200, "OK", ""); e2 == nil {
						_ = args.PutResponse(resp)
					}
				}
			}
			// on_request_count rotation: fire on the UI thread when threshold hit.
			if p.rotState != nil {
				if newProf, rotated := p.rotState.IncrementRequests(); rotated {
					mw := p.mainWindow
					mw.Synchronize(func() {
						p.br.UpdateProfile(newProf)
						_ = p.injectPolyfill()
						p.auditLog.record("rotation", "request-count rotation → "+newProf.Name)
					})
				}
			}
		}
		c.AddWebResourceRequestedFilter("*", edge.COREWEBVIEW2_WEB_RESOURCE_CONTEXT_ALL)
		p.log.Info("ad/tracker blocker wired to WebResourceRequested")
	}

	// в"Ђв"Ђ Download-shelf push в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	// Whenever the download list changes, push updated JSON to the JS shelf.
	dlMgr.onChange = func() {
		data, _ := json.Marshal(dlMgr.All())
		js := "if(typeof _dlPush==='function')_dlPush(" + string(data) + ");"
		mw.Synchronize(func() { wv.Eval(js) })
	}

	// в"Ђв"Ђ Frameless chrome в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	wvHWND := win.HWND(uintptr(p.wv.Window()))
	p.wvHWND = wvHWND
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

	// в"Ђв"Ђ Toolbar WebView2 (isolated 82 px window above content) в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	// Created separately from the content WebView2 so page scripts can never
	// interfere with the tab strip or address bar.
	tbarWv := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:    false,
		DataPath: filepath.Join(userDataDir, ".toolbar"),
		WindowOptions: webview2.WindowOptions{
			Width:  defaultWindowW,
			Height: defaultWindowH,
		},
	})
	if tbarWv == nil {
		return nil, fmt.Errorf("webview2: failed to create toolbar instance")
	}
	p.toolbarWv = tbarWv
	tbHWND := win.HWND(uintptr(tbarWv.Window()))
	p.toolbarHWND = tbHWND

	// Reparent toolbar window as WS_CHILD of wvHWND (same technique as DevTools).
	tbStyle := uint32(win.GetWindowLong(tbHWND, win.GWL_STYLE))
	tbStyle = (tbStyle &^ uint32(win.WS_POPUP) &^ uint32(win.WS_CAPTION) &^
		uint32(win.WS_THICKFRAME) &^ uint32(win.WS_SYSMENU)) | uint32(win.WS_CHILD)
	win.SetWindowLong(tbHWND, win.GWL_STYLE, int32(tbStyle))
	const wsExAppWindow = 0x00040000
	tbExStyle := win.GetWindowLong(tbHWND, win.GWL_EXSTYLE)
	win.SetWindowLong(tbHWND, win.GWL_EXSTYLE, tbExStyle&^int32(wsExAppWindow))
	win.SetParent(tbHWND, wvHWND)
	win.SetWindowPos(tbHWND, 0, 0, 0, 0, 0,
		win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)
	var initCR win.RECT
	win.GetClientRect(wvHWND, &initCR)
	win.MoveWindow(tbHWND, 0, 0, initCR.Right, int32(toolbarH), true)
	win.SetWindowPos(tbHWND, win.HWND_TOP, 0, 0, 0, 0,
		win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)
	win.ShowWindow(tbHWND, win.SW_SHOW)

	// в"Ђв"Ђ Window management bindings в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	const swMaximize = 3

	// в"Ђв"Ђ Debug helper в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	p.wv.Bind("__ghostDebug", func(msg string) {
		p.log.Info("JS-DEBUG: " + msg)
	})

	// в"Ђв"Ђ Tab state (persistence only вЂ" live state is owned by toolbar WebView2) в"Ђ
	p.loadTabsJSON()

	// в"Ђв"Ђ Resize (bottom/side edges вЂ" top edge handled by toolbar WebView2) в"Ђв"Ђв"Ђв"Ђв"Ђ
	p.wv.Bind("__ghostStartResize", func(ht int) {
		var pt win.POINT
		win.GetCursorPos(&pt)
		lp := uintptr(pt.Y)<<16 | uintptr(uint16(pt.X))
		win.ReleaseCapture()
		win.PostMessage(wvHWND, win.WM_NCLBUTTONDOWN, uintptr(ht), lp)
	})

	// в"Ђв"Ђ Content в†’ toolbar forwarding bindings в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	// These let keyboard shortcuts and the context menu in the content WebView2
	// reach the toolbar WebView2 through Go without any JS-to-JS cross-frame call.
	p.wv.Bind("__ghostNewTab", func() {
		p.mainWindow.Synchronize(func() {
			if p.toolbarWv != nil {
				p.toolbarWv.Eval("if(typeof _gsNewTab==='function')_gsNewTab();")
			}
		})
	})
	p.wv.Bind("__ghostCloseTab", func() {
		p.mainWindow.Synchronize(func() {
			if p.toolbarWv != nil {
				p.toolbarWv.Eval("if(typeof _gsCloseCurrentTab==='function')_gsCloseCurrentTab();")
			}
		})
	})
	p.wv.Bind("__ghostOpenInNewTab", func(url string) {
		if url == "" || strings.HasPrefix(url, "javascript:") {
			return
		}
		urlJSON, _ := json.Marshal(url)
		p.mainWindow.Synchronize(func() {
			if p.toolbarWv != nil {
				p.toolbarWv.Eval("if(typeof _gsOpenInNewTab==='function')_gsOpenInNewTab(" + string(urlJSON) + ");")
			}
		})
	})
	p.wv.Bind("__ghostFocusAddr", func() {
		p.mainWindow.Synchronize(func() {
			if p.toolbarWv != nil {
				p.toolbarWv.Eval("(function(){var a=document.getElementById('_gs_addr');if(a){a.focus();a.select();}})()")
			}
		})
	})
	p.wv.Bind("__ghostToggleBm", func() {
		p.mainWindow.Synchronize(func() {
			if p.toolbarWv != nil {
				p.toolbarWv.Eval("if(typeof _bmToggle==='function')_bmToggle();")
			}
		})
	})

	// в"Ђв"Ђ Toolbar WebView2 bindings в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	p.toolbarWv.Bind("__tbMinimize", func() {
		win.PostMessage(wvHWND, win.WM_SYSCOMMAND, win.SC_MINIMIZE, 0)
	})
	p.toolbarWv.Bind("__tbMaximize", func() {
		var wp win.WINDOWPLACEMENT
		wp.Length = uint32(unsafe.Sizeof(wp))
		win.GetWindowPlacement(wvHWND, &wp)
		if wp.ShowCmd == swMaximize {
			win.PostMessage(wvHWND, win.WM_SYSCOMMAND, win.SC_RESTORE, 0)
		} else {
			win.PostMessage(wvHWND, win.WM_SYSCOMMAND, win.SC_MAXIMIZE, 0)
		}
	})
	p.toolbarWv.Bind("__tbClose", func() {
		win.PostMessage(wvHWND, win.WM_CLOSE, 0, 0)
	})
	p.toolbarWv.Bind("__tbStartDrag", func() {
		var pt win.POINT
		win.GetCursorPos(&pt)
		lp := uintptr(pt.Y)<<16 | uintptr(uint16(pt.X))
		win.ReleaseCapture()
		win.PostMessage(wvHWND, win.WM_NCLBUTTONDOWN, win.HTCAPTION, lp)
	})
	p.toolbarWv.Bind("__tbIsMaximized", func() bool {
		var wp win.WINDOWPLACEMENT
		wp.Length = uint32(unsafe.Sizeof(wp))
		win.GetWindowPlacement(wvHWND, &wp)
		return wp.ShowCmd == swMaximize
	})
	p.toolbarWv.Bind("__tbStartResize", func(ht int) {
		var pt win.POINT
		win.GetCursorPos(&pt)
		lp := uintptr(pt.Y)<<16 | uintptr(uint16(pt.X))
		win.ReleaseCapture()
		win.PostMessage(wvHWND, win.WM_NCLBUTTONDOWN, uintptr(ht), lp)
	})
	p.toolbarWv.Bind("__tbBack", func() {
		p.mainWindow.Synchronize(func() { p.wv.Eval("history.back()") })
	})
	p.toolbarWv.Bind("__tbFwd", func() {
		p.mainWindow.Synchronize(func() { p.wv.Eval("history.forward()") })
	})
	p.toolbarWv.Bind("__tbReload", func() {
		p.mainWindow.Synchronize(func() { p.wv.Eval("location.reload()") })
	})
	p.toolbarWv.Bind("__tbGetTabs", func() {
		data := p.tabsJSON
		p.mainWindow.Synchronize(func() {
			p.toolbarWv.Eval("if(window.__tbTabsCb){var _f=window.__tbTabsCb;window.__tbTabsCb=null;_f(" + data + ");}")
		})
	})
	p.toolbarWv.Bind("__tbSetTabs", func(j string) {
		p.tabsJSON = j
		go p.saveTabsJSON(j)
	})
	p.toolbarWv.Bind("__tbGoTo", func(url, tabsJSON string) {
		if tabsJSON == "" {
			tabsJSON = p.tabsJSON
		}
		if strings.HasPrefix(url, "ghost://") {
			html := p.ghostPageHTML(url)
			if tabsJSON != "" {
				html = strings.Replace(html, "</head>",
					`<script>window.__ghostInitTabs=`+tabsJSON+`;</script></head>`, 1)
			}
			dataURL := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))
			p.mainWindow.Synchronize(func() {
				p.tabsJSON = tabsJSON
				p.wv.Navigate(dataURL)
			})
		} else {
			p.mainWindow.Synchronize(func() {
				p.tabsJSON = tabsJSON
				p.wv.Navigate(url)
			})
		}
	})
	p.toolbarWv.Bind("__tbAddBookmark", func(url, title string) {
		_, _ = p.bookmarks.Add(url, title)
	})
	p.toolbarWv.Bind("__tbRemoveBookmark", func(url string) {
		_ = p.bookmarks.RemoveByURL(url)
	})
	p.toolbarWv.Bind("__tbGetBookmarks", func() string {
		data, _ := json.Marshal(p.bookmarks.All())
		return string(data)
	})
	p.toolbarWv.Bind("__tbIsBookmarked", func(url string) bool {
		return p.bookmarks.Has(url)
	})
	p.toolbarWv.Bind("__tbGetBlockedCount", func() int64 {
		return p.blocker.BlockedCount()
	})
	p.toolbarWv.Bind("__tbOpenDevTools", func() {
		p.mainWindow.Synchronize(func() { p.EmbedDevToolsToggle() })
	})
	p.toolbarWv.Bind("__tbToggleDownloads", func() {
		p.mainWindow.Synchronize(func() {
			p.wv.Eval("if(typeof _gsDlToggle==='function')_gsDlToggle();")
		})
	})
	p.toolbarWv.Bind("__tbOnLoad", func() {
		if p.curURL == "" {
			return
		}
		url := p.curURL
		p.mainWindow.Synchronize(func() {
			urlJSON, _ := json.Marshal(url)
			p.toolbarWv.Eval("if(typeof _tbNavUpdate==='function')_tbNavUpdate(" + string(urlJSON) + ")")
		})
	})
	p.toolbarWv.Bind("__tbDebug", func(msg string) {
		p.log.Info("TB-DEBUG: " + msg)
	})

	// в"Ђв"Ђ Bookmark bindings в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	p.wv.Bind("__ghostAddBookmark", func(url, title string) {
		_, _ = p.bookmarks.Add(url, title)
		p.mainWindow.Synchronize(func() {
			p.wv.Eval("if(typeof _bmBarUpdate==='function')_bmBarUpdate();")
		})
	})
	p.wv.Bind("__ghostRemoveBookmark", func(url string) {
		_ = p.bookmarks.RemoveByURL(url)
		p.mainWindow.Synchronize(func() {
			p.wv.Eval("if(typeof _bmBarUpdate==='function')_bmBarUpdate();")
		})
	})
	p.wv.Bind("__ghostGetBookmarks", func() string {
		data, _ := json.Marshal(p.bookmarks.All())
		return string(data)
	})
	p.wv.Bind("__ghostIsBookmarked", func(url string) bool {
		return p.bookmarks.Has(url)
	})

	// в"Ђв"Ђ History bindings в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
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

	// в"Ђв"Ђ Download bindings в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
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

	// в"Ђв"Ђ Blocker bindings в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	p.wv.Bind("__ghostGetBlockedCount", func() int64 {
		return p.blocker.BlockedCount()
	})

	// в"Ђв"Ђ DevTools binding в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	// Called from the HTML context menu and the F12 action.
	// JS bindings run in a goroutine, so Synchronize onto the UI thread first.
	p.wv.Bind("__ghostOpenDevTools", func() {
		p.mainWindow.Synchronize(func() {
			p.EmbedDevToolsToggle()
		})
	})

	// в"Ђв"Ђ Privacy bindings в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
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

	// в"Ђв"Ђ Settings bindings в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
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
		p.auditLog.record("settings-save", "profile settings updated for "+prof.Name)
		if err := p.injectPolyfill(); err != nil {
			p.log.Warn("polyfill re-injection after settings save failed", "error", err.Error())
		}
	})

	// в"Ђв"Ђ Profile switcher bindings в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	p.wv.Bind("__ghostListProfiles", func() string {
		activeID := p.br.Profile().ID
		var items []profileListItem
		if p.identityStore != nil {
			if metas, err := p.identityStore.List(); err == nil {
				for _, m := range metas {
					item := profileListItem{ID: m.ID, Name: m.Name, Active: m.ID == activeID}
					if full, err2 := p.identityStore.Load(m.ID); err2 == nil {
						item.OS = full.Hardware.Platform
						item.Browser = extractBrowserName(full.Browser.UserAgent)
					}
					items = append(items, item)
				}
			}
		}
		if len(items) == 0 {
			prof := p.br.Profile()
			items = []profileListItem{{
				ID: prof.ID, Name: prof.Name,
				OS: prof.Hardware.Platform, Browser: extractBrowserName(prof.Browser.UserAgent),
				Active: true,
			}}
		}
		b, _ := json.Marshal(items)
		return string(b)
	})
	p.wv.Bind("__ghostSwitchProfile", func(id string) {
		if p.identityStore == nil {
			return
		}
		newProf, err := p.identityStore.Load(id)
		if err != nil {
			p.log.Warn("switch profile: load failed", "id", id, "error", err.Error())
			return
		}
		p.br.UpdateProfile(newProf)
		if p.rotState != nil {
			if rs, e2 := identity.NewRotationState(newProf, p.rotPolicy); e2 == nil {
				p.rotState = rs
			}
		}
		if err2 := p.injectPolyfill(); err2 != nil {
			p.log.Warn("polyfill re-injection after profile switch failed", "error", err2.Error())
		}
		p.auditLog.record("profile-switch", "switched to "+newProf.Name)
		p.log.Info("profile switched", "id", id, "name", newProf.Name)
	})

	// в"Ђв"Ђ Navigate binding в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	p.wv.Bind("__ghostNavigate", p.handleGhostScheme)

	// ── Network log / Audit log / Blocklist bindings ──────────────────────────
	p.wv.Bind("__ghostGetNetworkLog", func() string {
		b, _ := json.Marshal(p.netLog.snapshot())
		return string(b)
	})
	p.wv.Bind("__ghostClearNetworkLog", func() { p.netLog.clear() })
	p.wv.Bind("__ghostGetAuditLog", func() string {
		b, _ := json.Marshal(p.auditLog.snapshot())
		return string(b)
	})
	p.wv.Bind("__ghostFetchBlocklist", func(url string) string {
		if err := p.blocker.FetchAndReload(url); err != nil {
			return err.Error()
		}
		p.auditLog.record("blocklist", "reloaded from "+url)
		return ""
	})

	// setupContentView registers all content bindings on p.wv with the
	// active-tab guards and injects all persistent polyfill/overlay scripts.
	// It runs after the inline bindings above so its versions win.
	p.setupContentView(p.wv)

	// в"Ђв"Ђ Initial layout: content below toolbar в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	{
		var cr win.RECT
		win.GetClientRect(wvHWND, &cr)
		p.setWebViewBounds(0, toolbarH, int(cr.Right), int(cr.Bottom))
	}

	// в"Ђв"Ђ Navigate toolbar to its standalone HTML page в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
	{
		html := p.ghostToolbarPage()
		if p.tabsJSON != "" {
			html = strings.Replace(html, "</head>",
				`<script>window.__tbInitTabs=`+p.tabsJSON+`;</script></head>`, 1)
		}
		p.toolbarWv.Navigate("data:text/html;base64," +
			base64.StdEncoding.EncodeToString([]byte(html)))
	}

	return p, nil
}

// в"Ђв"Ђ navigation в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ

func (p *WebViewPanel) Navigate(url string) {
	if p.wv == nil {
		return
	}
	p.log.Info("navigate", "url", url)
	if strings.HasPrefix(url, "ghost://") {
		p.wv.Navigate("data:text/html;base64," + p.ghostPageDataURL(url))
		// Push the pretty ghost:// URL to the toolbar address bar immediately,
		// because the content WebView2 will only report a data: URL via __ghostOnURL.
		if p.toolbarWv != nil {
			urlJSON, _ := json.Marshal(url)
			p.mainWindow.Synchronize(func() {
				p.toolbarWv.Eval("if(typeof _tbNavUpdate==='function')_tbNavUpdate(" + string(urlJSON) + ")")
			})
		}
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

// в"Ђв"Ђ Minimal content overlay в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ
// The browser chrome (toolbar, tabs, address bar) now lives in a dedicated
// toolbar WebView2.  This overlay only provides: context menu, window.open /
// target=_blank intercepts, download shelf, and the find bar.

func (p *WebViewPanel) injectChromeOverlay() {
	script := fmt.Sprintf(`(function(){
'use strict';
try{if(window!==window.top)return;}catch(e){return;}
var _searchURL=%q;

/* в"Ђв"Ђ Download shelf в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ */
var _dlShelf=document.createElement('div');_dlShelf.id='_gs_dl_shelf';
_dlShelf.style.cssText=
  'position:fixed;bottom:0;left:0;right:0;z-index:2147483646;'+
  'background:rgba(18,18,44,.97);border-top:1px solid rgba(255,255,255,.1);'+
  'transform:translateY(100%%);transition:transform .22s cubic-bezier(0.4,0,0.2,1);'+
  'padding:10px 16px 14px;display:flex;flex-direction:column;gap:6px;'+
  'max-height:220px;overflow-y:auto;font-family:Segoe UI,system-ui,sans-serif;'+
  'font-size:12px;color:#E8E8F4;box-sizing:border-box';
_dlShelf.innerHTML=
  '<div id="_gs_dl_hdr_row" style="display:flex;justify-content:space-between;align-items:center;padding-bottom:6px;border-bottom:1px solid rgba(255,255,255,.08);margin-bottom:4px">'+
  '<span style="font-size:12px;font-weight:600;color:rgba(255,255,255,.7)">Downloads</span>'+
  '<button id="_gs_dl_x" style="background:transparent;border:none;color:rgba(255,255,255,.4);font-size:13px;cursor:pointer;padding:0 4px;border-radius:3px">&#10005;</button></div>';
function _dlMountShelf(){
  if(!document.getElementById('_gs_dl_shelf')){
    try{document.documentElement.appendChild(_dlShelf);}catch(_){return;}
    var x=document.getElementById('_gs_dl_x');
    if(x)x.addEventListener('click',function(){_dlClose();});
  }
}
function _dlOpen(){_dlMountShelf();_dlShelf.style.transform='translateY(0)';}
function _dlClose(){_dlShelf.style.transform='translateY(100%%)';}
window._gsDlClose=_dlClose;
window._gsDlToggle=function(){
  _dlMountShelf();
  if(_dlShelf.style.transform==='translateY(0)'){_dlClose();}
  else{try{__ghostGetDownloads().then(function(j){_dlPush(JSON.parse(j));});}catch(_){_dlOpen();}}
};
window._dlPush=function _dlPush(items){
  _dlMountShelf();
  if(!items||!items.length){setTimeout(_dlClose,1500);return;}
  var hdr=document.getElementById('_gs_dl_hdr_row');
  while(_dlShelf.lastChild&&_dlShelf.lastChild!==hdr)_dlShelf.removeChild(_dlShelf.lastChild);
  var hasActive=false;
  items.forEach(function(it){
    if(it.State===0)hasActive=true;
    var row=document.createElement('div');
    row.style.cssText='display:flex;flex-direction:column;gap:2px;padding:2px 0;border-bottom:1px solid rgba(255,255,255,.05)';
    var pct=it.TotalBytes>0?Math.round(it.RecvBytes/it.TotalBytes*100):0;
    var fname=it.Filename||(it.URL||'').split('/').pop()||'download';
    var st=it.State===0?(pct+'%%'):it.State===1?'Done':it.State===2?'Failed':'Cancelled';
    row.innerHTML=
      '<div style="display:flex;align-items:center;gap:6px">'+
      '<span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1">'+fname+'</span>'+
      '<span style="font-size:11px;color:rgba(255,255,255,.45);flex-shrink:0">'+st+'</span></div>'+
      (it.State===0?
        '<div style="height:3px;background:rgba(255,255,255,.1);border-radius:2px;overflow:hidden;margin-top:3px">'+
        '<div style="height:100%%;background:linear-gradient(90deg,#7B61FF,#B89CFF);width:'+pct+'%%"></div></div>':
        '');
    _dlShelf.appendChild(row);
  });
  _dlOpen();
  if(!hasActive){setTimeout(_dlClose,4000);}
};

/* в"Ђв"Ђ Find bar в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ */
var _fb=null;
function _ensureFb(){
  if(_fb&&document.getElementById('_gs_find'))return;
  _fb=document.createElement('div');_fb.id='_gs_find';
  _fb.style.cssText=
    'position:fixed;bottom:0;right:0;z-index:2147483646;'+
    'background:rgba(20,20,50,.95);border:1px solid rgba(255,255,255,.15);'+
    'border-radius:8px 8px 0 0;padding:8px 10px;gap:6px;align-items:center;'+
    'box-shadow:0 -2px 12px rgba(0,0,0,.5);font-family:Segoe UI,system-ui,sans-serif';
  _fb.style.display='none';
  var btnCss='background:transparent;border:1px solid rgba(255,255,255,.2);color:rgba(255,255,255,.8);font-size:12px;padding:4px 8px;border-radius:4px;cursor:pointer';
  _fb.innerHTML=
    '<input id="_gs_fi" autocomplete="off" placeholder="FindвЂ¦" '+
    'style="border:1px solid rgba(255,255,255,.2);border-radius:6px;background:rgba(255,255,255,.08);color:#fff;font-size:13px;padding:4px 10px;outline:none;width:200px">'+
    '<button id="_gs_fp" title="Previous" style="'+btnCss+'">&#8593;</button>'+
    '<button id="_gs_fn" title="Next" style="'+btnCss+'">&#8595;</button>'+
    '<button id="_gs_fx" title="Close" style="'+btnCss+'">&#10005;</button>';
  try{document.documentElement.appendChild(_fb);}catch(_){_fb=null;return;}
  var _fi=document.getElementById('_gs_fi');
  _fi.addEventListener('input',function(){if(_fi.value)window.find(_fi.value,false,false,true,false,false,false);});
  _fi.addEventListener('keydown',function(e){
    if(e.key==='Enter'){e.preventDefault();window.find(_fi.value,false,e.shiftKey,true,false,false,false);}
    if(e.key==='Escape'){_fb.style.display='none';}
  });
  document.getElementById('_gs_fp').addEventListener('click',function(){var fi=document.getElementById('_gs_fi');if(fi)window.find(fi.value,false,true,true,false,false,false);});
  document.getElementById('_gs_fn').addEventListener('click',function(){var fi=document.getElementById('_gs_fi');if(fi)window.find(fi.value,false,false,true,false,false,false);});
  document.getElementById('_gs_fx').addEventListener('click',function(){_fb.style.display='none';});
}
window._gsOpenFind=function(){_ensureFb();if(!_fb)return;_fb.style.display='flex';setTimeout(function(){var fi=document.getElementById('_gs_fi');if(fi){fi.focus();fi.select();}},50);};
document.addEventListener('keydown',function(e){if(e.key==='Escape'){if(_fb)_fb.style.display='none';}},true);

/* в"Ђв"Ђ Context menu в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ */
var _cm=null;
function _ensureCm(){
  if(_cm&&document.getElementById('_gs_ctx'))return;
  _cm=document.createElement('div');_cm.id='_gs_ctx';
  _cm.style.cssText=
    'position:fixed;z-index:2147483646;background:rgba(20,20,50,.97);'+
    'border:1px solid rgba(255,255,255,.12);border-radius:8px;padding:4px;min-width:160px;'+
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
  var d=document.createElement('div');d.style.cssText='height:1px;background:rgba(255,255,255,.1);margin:3px 6px';return d;
}
function _showCtx(e){
  _ensureCm();if(!_cm)return;_cm.innerHTML='';
  var link=e.target.closest('a[href]');var img=e.target.closest('img');
  var sel=window.getSelection?window.getSelection().toString().trim():'';
  _cm.appendChild(_ctxItem('Back',function(){history.back();}));
  _cm.appendChild(_ctxItem('Forward',function(){history.forward();}));
  _cm.appendChild(_ctxItem('Reload',function(){location.reload();}));
  if(link||img||sel)_cm.appendChild(_ctxSep());
  if(link){
    (function(href){
      _cm.appendChild(_ctxItem('Open in new tab',function(){try{__ghostOpenInNewTab(href);}catch(_){}}));
      _cm.appendChild(_ctxItem('Copy link',function(){try{navigator.clipboard.writeText(href);}catch(_){}}));
    })(link.href);
  }
  if(img){
    (function(src){
      _cm.appendChild(_ctxItem('Open image in new tab',function(){try{__ghostOpenInNewTab(src);}catch(_){}}));
      _cm.appendChild(_ctxItem('Copy image URL',function(){try{navigator.clipboard.writeText(src);}catch(_){}}));
    })(img.src);
  }
  if(sel){
    var short=sel.length>30?sel.slice(0,30)+'вЂ¦':sel;
    (function(q){_cm.appendChild(_ctxItem('Search "'+short+'"',function(){location.href=_searchURL+encodeURIComponent(q);}));})(sel);
  }
  _cm.appendChild(_ctxSep());
  _cm.appendChild(_ctxItem('Developer Tools',function(){try{__ghostOpenDevTools();}catch(_){}}));
  var x=e.clientX,y=e.clientY;
  _cm.style.left=x+'px';_cm.style.top=y+'px';_cm.style.display='block';
  var rect=_cm.getBoundingClientRect();
  if(rect.right>window.innerWidth)_cm.style.left=(x-rect.width)+'px';
  if(rect.bottom>window.innerHeight)_cm.style.top=(y-rect.height)+'px';
}
document.addEventListener('contextmenu',function(e){e.preventDefault();_showCtx(e);},true);
document.addEventListener('click',function(e){if(_cm&&!_cm.contains(e.target))_cm.style.display='none';},true);

/* в"Ђв"Ђ New-window / target=_blank intercepts в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ */
(function(){
  var _wo=window.open;
  window.open=function(url,name,feat){
    if(url&&typeof url==='string'&&!url.startsWith('javascript:')){
      try{__ghostOpenInNewTab(url);}catch(_){}return window;
    }
    return _wo?_wo.apply(this,arguments):null;
  };
})();
document.addEventListener('click',function(e){
  var a=e.target.closest('a[target]');
  if(!a||!a.href)return;
  var t=(a.getAttribute('target')||'').toLowerCase();
  if(t!=='_blank'&&t!=='_new'&&t!=='blank')return;
  try{if(new URL(a.href).pathname===location.pathname&&a.href.includes('#'))return;}catch(_){}
  e.preventDefault();e.stopPropagation();
  try{__ghostOpenInNewTab(a.href);}catch(_){}
},true);

/* в"Ђв"Ђ Download click intercept в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ */
document.addEventListener('click',function(e){
  var a=e.target.closest('a[download]');
  if(!a||!a.href)return;
  var fname=a.download||a.href.split('/').pop()||'download';
  try{__ghostDownloadStarted(a.href,fname,0);}catch(_){}
},true);

/* в"Ђв"Ђ Record history on page load в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ */
window.addEventListener('load',function(){
  try{
    var u=window.location.href;
    if(u&&!u.startsWith('data:')&&!u.startsWith('about:'))__ghostRecordHistory(u,document.title||'');
  }catch(_){}
});
})();`, p.searchEngineURL)

	p.wv.Init(script)
}

func (p *WebViewPanel) injectChromeOverlayOn(wv webview2.WebView) {
	orig := p.wv
	p.wv = wv
	p.injectChromeOverlay()
	p.wv = orig
}

// injectResizeEdges injects a JS listener that detects cursor proximity to
// viewport edges and calls __ghostStartResize.
func (p *WebViewPanel) injectResizeEdgesOn(wv webview2.WebView) {
	wv.Init(`(function(){
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

func (p *WebViewPanel) injectResizeEdges() { p.injectResizeEdgesOn(p.wv) }

func (p *WebViewPanel) injectKeyboardShortcutsOn(wv webview2.WebView) {
	wv.Init(`(function(){
'use strict';
try{if(window!==window.top)return;}catch(e){return;}
window.addEventListener('keydown',function(e){
  if(!e.ctrlKey)return;
  switch(e.key){
    case 't':case 'T':
      e.preventDefault();try{__ghostNewTab();}catch(_){}break;
    case 'w':case 'W':
      e.preventDefault();try{__ghostCloseTab();}catch(_){}break;
    case 'l':case 'L':
      e.preventDefault();try{__ghostFocusAddr();}catch(_){}break;
    case 'r':case 'R':
      e.preventDefault();location.reload();break;
    case 'd':case 'D':
      e.preventDefault();try{__ghostToggleBm();}catch(_){}break;
    case 'f':case 'F':
      e.preventDefault();if(typeof window._gsOpenFind==='function')window._gsOpenFind();break;
  }
});
})();`)
}

func (p *WebViewPanel) injectKeyboardShortcuts() { p.injectKeyboardShortcutsOn(p.wv) }

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
	case "network":
		return p.ghostNetworkPage()
	case "audit":
		return p.ghostAuditPage()
	}

	var body string
	switch page {
	case "newtab", "":
		body = "<h2>Ghost-Silicon</h2><p style=\"margin-top:10px;opacity:.7\">Type an address above and press Enter.</p>"
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
body{font-family:'Segoe UI',system-ui,sans-serif;background:linear-gradient(160deg,#1a0a0a 0%,#0d0d30 50%,#050a28 100%);color:#E8E8F4;min-height:100vh}
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
    var del=document.createElement('button');del.className='bm-del';del.textContent='вњ•';del.title='Remove';
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
    var del=document.createElement('button');del.className='he-del';del.textContent='вњ•';del.title='Remove';
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
var states={0:'DownloadingвЂ¦',1:'Complete',2:'Failed',3:'Cancelled'};
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
      (it.State===0&&it.TotalBytes>0?' вЂ" '+pct+'%':'')+
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

func (p *WebViewPanel) ghostNetworkPage() string {
	css := `.subtitle{font-size:13px;color:rgba(255,255,255,.45);margin-top:-14px;margin-bottom:20px}
.toolbar{display:flex;align-items:center;gap:10px;margin-bottom:14px}
.btn{padding:6px 16px;border:1px solid rgba(255,255,255,.2);border-radius:6px;background:rgba(255,255,255,.07);color:#E8E8F4;font-size:12px;cursor:pointer}
.btn:hover{background:rgba(255,255,255,.13)}
table{width:100%;border-collapse:collapse;font-size:12px}
th{text-align:left;padding:6px 8px;color:rgba(255,255,255,.4);font-weight:500;border-bottom:1px solid rgba(255,255,255,.1)}
td{padding:5px 8px;vertical-align:top;border-bottom:1px solid rgba(255,255,255,.05);font-family:monospace;word-break:break-all}
tr.blocked td{color:#FF6B6B}
tr.allowed td{color:rgba(255,255,255,.75)}
.badge{display:inline-block;padding:1px 6px;border-radius:4px;font-size:10px;font-weight:600}
.b-block{background:rgba(255,80,80,.25);color:#FF8080}
.b-allow{background:rgba(80,200,120,.15);color:#80C880}
.stat{font-size:22px;font-weight:700;color:#A080FF;display:inline-block;margin-right:6px}`
	body := `<h1>Network Monitor</h1>
<p class="subtitle">Live request log — last ` + fmt.Sprintf("%d", netLogCap) + ` requests</p>
<div class="toolbar">
  <span id="stats"></span>
  <button class="btn" onclick="__ghostClearNetworkLog().then(refresh)">Clear</button>
</div>
<table>
  <thead><tr><th>Time</th><th>Host</th><th>Status</th><th>URL</th></tr></thead>
  <tbody id="tbody"></tbody>
</table>
<script>
function fmt(ts){var d=new Date(ts);return d.toLocaleTimeString();}
function trunc(s,n){return s.length>n?s.slice(0,n)+'…':s;}
function refresh(){
  __ghostGetNetworkLog().then(function(j){
    var rows=JSON.parse(j);
    var blocked=rows.filter(function(r){return r.blocked;}).length;
    document.getElementById('stats').innerHTML=
      '<span class="stat">'+rows.length+'</span>requests&nbsp;&nbsp;'+
      '<span class="stat" style="color:#FF8080">'+blocked+'</span>blocked';
    var tb=document.getElementById('tbody');
    tb.innerHTML=rows.map(function(r){
      var cls=r.blocked?'blocked':'allowed';
      var badge=r.blocked?'<span class="badge b-block">BLOCKED</span>':'<span class="badge b-allow">OK</span>';
      return '<tr class="'+cls+'"><td>'+fmt(r.ts)+'</td><td>'+r.host+'</td><td>'+badge+'</td><td>'+trunc(r.url,120)+'</td></tr>';
    }).join('');
  });
}
refresh();setInterval(refresh,1500);
</script>`
	return ghostPageBase("network", css, body)
}

func (p *WebViewPanel) ghostAuditPage() string {
	css := `.subtitle{font-size:13px;color:rgba(255,255,255,.45);margin-top:-14px;margin-bottom:20px}
table{width:100%;border-collapse:collapse;font-size:12px}
th{text-align:left;padding:6px 8px;color:rgba(255,255,255,.4);font-weight:500;border-bottom:1px solid rgba(255,255,255,.1)}
td{padding:5px 8px;vertical-align:top;border-bottom:1px solid rgba(255,255,255,.05)}
td:last-child{font-family:monospace;font-size:11px;word-break:break-all;color:rgba(255,255,255,.65)}
.tag{display:inline-block;padding:1px 7px;border-radius:4px;font-size:10px;font-weight:600}
.t-navigate{background:rgba(80,140,255,.2);color:#80AAFF}
.t-profile-switch{background:rgba(255,160,80,.2);color:#FFA050}
.t-settings-save{background:rgba(80,200,120,.2);color:#80C880}
.t-rotation{background:rgba(160,80,255,.2);color:#C080FF}
.t-blocked{background:rgba(255,80,80,.2);color:#FF8080}
.t-blocklist{background:rgba(80,200,200,.2);color:#80DDDD}
.t-start{background:rgba(200,200,80,.2);color:#D0D050}
.t-default{background:rgba(255,255,255,.1);color:rgba(255,255,255,.6)}`
	body := `<h1>Audit Log</h1>
<p class="subtitle">Browser events this session</p>
<table>
  <thead><tr><th>Time</th><th>Event</th><th>Detail</th></tr></thead>
  <tbody id="tbody"></tbody>
</table>
<script>
var colorMap={navigate:'t-navigate','profile-switch':'t-profile-switch','settings-save':'t-settings-save',
  rotation:'t-rotation',blocked:'t-blocked',blocklist:'t-blocklist',start:'t-start'};
function fmt(ts){var d=new Date(ts);return d.toLocaleTimeString();}
function refresh(){
  __ghostGetAuditLog().then(function(j){
    var rows=JSON.parse(j);
    var tb=document.getElementById('tbody');
    tb.innerHTML=rows.map(function(r){
      var cls=colorMap[r.type]||'t-default';
      return '<tr><td>'+fmt(r.ts)+'</td>'+
        '<td><span class="tag '+cls+'">'+r.type+'</span></td>'+
        '<td>'+r.detail+'</td></tr>';
    }).join('');
  });
}
refresh();setInterval(refresh,2000);
</script>`
	return ghostPageBase("audit", css, body)
}

// в"Ђв"Ђ event bridge в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ

// ghostToolbarPage returns the standalone 82 px HTML page loaded into the
// toolbar WebView2.  All chrome interactions use __tb* Go bindings.
func (p *WebViewPanel) ghostToolbarPage() string {
	searchURL := p.searchEngineURL
	if searchURL == "" {
		searchURL = "https://duckduckgo.com/?q="
	}
	searchJSON, _ := json.Marshal(searchURL)
	return `<!DOCTYPE html><html><head>
<meta charset="utf-8">
<script>window.__tbSearchURL=` + string(searchJSON) + `;</script>
<style>
*{box-sizing:border-box;margin:0;padding:0}
html,body{width:100%;height:82px;overflow:hidden;
  background:linear-gradient(90deg,#3D1A0A 0%,#2A1560 40%,#0A1A6B 70%,#050E40 100%);
  color:#E8E8F4;font-family:'Segoe UI',system-ui,sans-serif;user-select:none;-webkit-user-select:none}
#tabs-row{height:38px;display:flex;align-items:flex-end;padding:0 0 0 8px;overflow:hidden}
#tablist{display:flex;align-items:flex-end;gap:2px;overflow:hidden;flex:1;min-width:0}
.tab{background:rgba(255,255,255,.10);border:1px solid rgba(255,255,255,.12);border-bottom:none;
  border-radius:8px 8px 0 0;padding:0 4px 0 10px;height:30px;display:flex;align-items:center;
  font-size:12px;color:rgba(255,255,255,.6);max-width:180px;min-width:80px;cursor:pointer;flex-shrink:0;
  transition:background .18s,color .18s}
.tab.active{background:rgba(255,255,255,.2);color:#fff;border-color:rgba(255,255,255,.28)}
.tab:hover:not(.active){background:rgba(255,255,255,.15);color:rgba(255,255,255,.9)}
.tab span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1}
.tab-fav{width:14px;height:14px;border-radius:2px;margin-right:4px;flex-shrink:0;object-fit:contain}
.tab-x{background:transparent;border:none;color:rgba(255,255,255,.35);font-size:11px;
  padding:0 3px;margin-left:2px;border-radius:3px;flex-shrink:0;cursor:pointer;line-height:1.5}
.tab-x:hover{background:rgba(255,255,255,.18);color:#fff}
#new-tab{background:transparent;border:none;color:rgba(255,255,255,.55);font-size:20px;
  padding:0 8px;border-radius:50%;align-self:center;cursor:pointer;flex-shrink:0;line-height:1}
#new-tab:hover{background:rgba(255,255,255,.14);color:#fff}
#wm-btns{display:flex;align-items:stretch;height:38px;margin-left:auto}
.wm-btn{background:transparent;border:none;width:46px;height:100%;
  color:rgba(255,255,255,.8);font-size:13px;cursor:pointer;
  display:flex;align-items:center;justify-content:center;transition:background .15s}
.wm-btn:hover{background:rgba(255,255,255,.18)}
#cls:hover{background:#E81123;color:#fff}
#nav-row{height:44px;display:flex;align-items:center;padding:0 10px;gap:6px}
.nav-btn{background:transparent;border:none;width:28px;height:28px;border-radius:50%;
  font-size:16px;color:rgba(255,255,255,.7);cursor:pointer;
  display:flex;align-items:center;justify-content:center;flex-shrink:0;transition:background .15s,color .15s}
.nav-btn:hover{background:rgba(255,255,255,.14);color:#fff}
#addr{flex:1;height:30px;border:1px solid rgba(255,255,255,.18);border-radius:15px;
  padding:0 14px;font-size:13px;background:rgba(255,255,255,.10);color:#fff;outline:none;
  transition:border-color .2s,background .2s}
#addr::placeholder{color:rgba(255,255,255,.35)}
#addr:focus{border-color:rgba(130,150,255,.75);background:rgba(255,255,255,.16)}
#bm{background:transparent;border:none;width:28px;height:28px;border-radius:50%;
  font-size:16px;color:rgba(255,255,255,.4);cursor:pointer;
  display:flex;align-items:center;justify-content:center;flex-shrink:0;transition:background .15s,color .15s}
#bm:hover{background:rgba(255,255,255,.13);color:#F4A460}
#bm.bm-on{color:#F4A460}
#badge{min-width:28px;height:22px;border-radius:11px;background:rgba(244,164,96,.10);
  border:1px solid rgba(244,164,96,.22);color:rgba(244,164,96,.65);font-size:11px;font-weight:600;
  display:flex;align-items:center;justify-content:center;padding:0 5px;cursor:pointer;
  flex-shrink:0;user-select:none;transition:background .15s,color .15s}
#badge:hover{background:rgba(244,164,96,.2);color:#F4A460}
#dl-btn{background:transparent;border:none;width:28px;height:28px;border-radius:50%;
  font-size:14px;color:rgba(255,255,255,.6);cursor:pointer;
  display:flex;align-items:center;justify-content:center;flex-shrink:0;transition:background .15s,color .15s}
#dl-btn:hover{background:rgba(255,255,255,.14);color:#fff}
</style>
</head>
<body>
<div id="tabs-row">
  <div id="tablist"></div>
  <button id="new-tab" title="New Tab (Ctrl+T)">+</button>
  <div id="wm-btns">
    <button class="wm-btn" id="min" title="Minimise">&#8212;</button>
    <button class="wm-btn" id="max" title="Maximise/Restore">&#9633;</button>
    <button class="wm-btn" id="cls" title="Close">&#10005;</button>
  </div>
</div>
<div id="nav-row">
  <button class="nav-btn" id="back" title="Back">&#8592;</button>
  <button class="nav-btn" id="fwd" title="Forward">&#8594;</button>
  <button class="nav-btn" id="reload" title="Reload">&#8635;</button>
  <input id="addr" type="text" spellcheck="false" placeholder="Search or enter address"/>
  <button id="bm" title="Bookmark">&#9734;</button>
  <div id="badge" title="Blocked trackers">0</div>
  <button id="dl-btn" title="Downloads">&#8595;</button>
</div>
<script>
(function(){
'use strict';
var _S=window.__tbSearchURL||'https://duckduckgo.com/?q=';
var _T={tabs:[],current:0};
var _loaded=false,_loadQ=[];
var _curURL='';
var _tablist=document.getElementById('tablist');

/* ── Load tab state ─────────────────────────────────────────── */
function _load(cb){
  if(_loaded){if(cb)cb();return;}
  if(cb)_loadQ.push(cb);
  if(_loadQ.length>1)return;
  if(window.__tbInitTabs&&window.__tbInitTabs.tabs&&window.__tbInitTabs.tabs.length){
    _T=window.__tbInitTabs;_loaded=true;
    var q=_loadQ.splice(0);q.forEach(function(f){try{f();}catch(_){}});return;
  }
  window.__tbTabsCb=function(obj){
    _T=(obj&&obj.tabs&&obj.tabs.length)?obj:{tabs:[{id:1,url:'ghost://newtab',title:'New Tab'}],current:1};
    _loaded=true;var q=_loadQ.splice(0);q.forEach(function(f){try{f();}catch(_){}});
  };
  try{__tbGetTabs();}catch(e){
    window.__tbTabsCb=null;
    _T={tabs:[{id:1,url:'ghost://newtab',title:'New Tab'}],current:1};
    _loaded=true;var q=_loadQ.splice(0);q.forEach(function(f){try{f();}catch(_){}});
  }
}
function _save(){try{__tbSetTabs(JSON.stringify(_T));}catch(_){}}

/* ── Render tabs ────────────────────────────────────────────── */
function _render(){
  if(!_T.tabs||!_T.tabs.length)return;
  var kids=_tablist.children;
  if(kids.length===_T.tabs.length){
    for(var i=0;i<_T.tabs.length;i++){
      var t=_T.tabs[i];var el=kids[i];
      var wantCls=t.id===_T.current?'tab active':'tab';
      if(el.className!==wantCls)el.className=wantCls;
      var sp=el.querySelector('span');var ttl=t.title||'New Tab';
      if(sp&&sp.textContent!==ttl){sp.textContent=ttl;sp.title=ttl;}
      var fav=el.querySelector('.tab-fav');
      if(fav){
        var http=t.url&&(t.url.startsWith('http://')||t.url.startsWith('https://'));
        if(http){try{var h=new URL(t.url).hostname;
          if(fav.getAttribute('data-h')!==h){fav.setAttribute('data-h',h);
            fav.src='https://www.google.com/s2/favicons?domain='+h+'&sz=16';
            fav.style.display='';fav.onerror=function(){this.style.display='none';};}}
          catch(_){fav.style.display='none';}}
        else{fav.style.display='none';}
      }
    }
    return;
  }
  _tablist.innerHTML='';
  _T.tabs.forEach(function(t){
    var el=document.createElement('div');
    el.className=t.id===_T.current?'tab active':'tab';
    var fav=document.createElement('img');fav.className='tab-fav';
    if(t.url&&(t.url.startsWith('http://')||t.url.startsWith('https://'))){
      try{var h=new URL(t.url).hostname;
        fav.src='https://www.google.com/s2/favicons?domain='+h+'&sz=16';
        fav.setAttribute('data-h',h);fav.onerror=function(){fav.style.display='none';};
      }catch(_){fav.style.display='none';}
    }else{fav.style.display='none';}
    var sp=document.createElement('span');sp.textContent=t.title||'New Tab';sp.title=t.title||'';
    var xb=document.createElement('button');xb.className='tab-x';xb.innerHTML='&#10005;';xb.title='Close';
    el.appendChild(fav);el.appendChild(sp);el.appendChild(xb);
    (function(id){
      el.addEventListener('click',function(e){if(xb.contains(e.target))return;_switchTab(id);});
      xb.addEventListener('click',function(e){e.stopPropagation();_closeTab(id);});
    })(t.id);
    _tablist.appendChild(el);
  });
}

/* ── Navigation ─────────────────────────────────────────────── */
function _updateCurURL(){
  if(!_curURL)return;
  for(var i=0;i<_T.tabs.length;i++){
    if(_T.tabs[i].id===_T.current){_T.tabs[i].url=_curURL;break;}
  }
}
function _goTo(url){
  try{__tbGoTo(url,JSON.stringify(_T));}catch(e){console.error('[tb] __tbGoTo:',e);}
}
function _switchTab(id){
  _updateCurURL();_T.current=id;_save();_render();
  for(var i=0;i<_T.tabs.length;i++){
    if(_T.tabs[i].id===id){var u=_T.tabs[i].url||'ghost://newtab';if(u!==_curURL)_goTo(u);return;}
  }
}

/* ── Tab operations (called from content WebView2 via Go bindings) ── */
window._gsNewTab=function(){
  _load(function(){
    _updateCurURL();
    var mx=0;_T.tabs.forEach(function(t){if(t.id>mx)mx=t.id;});
    var id=mx+1;_T.tabs.push({id:id,url:'ghost://newtab',title:'New Tab'});
    _T.current=id;_save();_render();_goTo('ghost://newtab');
  });
};
window._gsCloseCurrentTab=function(){_closeTab(_T.current);};
window._gsOpenInNewTab=function(url){
  if(!url||url.startsWith('javascript:'))return;
  _load(function(){
    _updateCurURL();
    var mx=0;_T.tabs.forEach(function(t){if(t.id>mx)mx=t.id;});
    var id=mx+1;_T.tabs.push({id:id,url:url,title:'New Tab'});
    _T.current=id;_save();_render();_goTo(url);
  });
};
function _closeTab(id){
  if(_T.tabs.length<=1){try{__tbClose();}catch(_){}return;}
  var idx=-1;for(var i=0;i<_T.tabs.length;i++){if(_T.tabs[i].id===id){idx=i;break;}}
  if(idx<0)return;
  var wa=id===_T.current;
  _T.tabs.splice(idx,1);
  if(wa){var ni=Math.min(idx,_T.tabs.length-1);_T.current=_T.tabs[ni].id;_save();_render();
    _goTo(_T.tabs[ni].url||'ghost://newtab');}
  else{_save();_render();}
}

/* ── Address bar ────────────────────────────────────────────── */
var _addr=document.getElementById('addr');
_addr.addEventListener('keydown',function(e){
  if(e.key!=='Enter')return;e.preventDefault();
  var r=_addr.value.trim();if(!r)return;
  var u=r;
  if(!u.match(/^https?:\/\//i)&&!u.startsWith('ghost://')){
    if(u.indexOf('.')>=0&&u.indexOf(' ')<0){u='https://'+u;}
    else{u=_S+encodeURIComponent(u);}
  }
  _load(function(){
    _updateCurURL();
    for(var i=0;i<_T.tabs.length;i++){if(_T.tabs[i].id===_T.current){_T.tabs[i].url=u;break;}}
    _save();_goTo(u);
  });
});
_addr.addEventListener('focus',function(){_addr.select();});

/* ── Callbacks from Go ──────────────────────────────────────── */
window._tbNavUpdate=function(url){
  _curURL=url;_addr.value=url;
  for(var i=0;i<_T.tabs.length;i++){if(_T.tabs[i].id===_T.current){_T.tabs[i].url=url;break;}}
  _save();_bmUpdate();_badgeUpdate();
};
window._tbTitleUpdate=function(title){
  for(var i=0;i<_T.tabs.length;i++){if(_T.tabs[i].id===_T.current){_T.tabs[i].title=title||'New Tab';break;}}
  _render();
};

/* ── Window controls ────────────────────────────────────────── */
function _updateMaxBtn(){
  var b=document.getElementById('max');if(!b)return;
  try{__tbIsMaximized().then(function(m){b.innerHTML=m?'&#10064;':'&#9633;';b.title=m?'Restore':'Maximise';});}catch(_){}
}
_updateMaxBtn();window.addEventListener('resize',_updateMaxBtn);
document.getElementById('tabs-row').addEventListener('mousedown',function(e){
  if(e.button===0&&!e.target.closest('button,.tab')){try{__tbStartDrag();}catch(_){}}
});
document.getElementById('min').addEventListener('click',function(){try{__tbMinimize();}catch(_){}});
document.getElementById('max').addEventListener('click',function(){_updateMaxBtn();try{__tbMaximize();}catch(_){}});
document.getElementById('cls').addEventListener('click',function(){try{__tbClose();}catch(_){}});
document.getElementById('new-tab').addEventListener('click',function(){window._gsNewTab();});
document.getElementById('back').addEventListener('click',function(){try{__tbBack();}catch(_){}});
document.getElementById('fwd').addEventListener('click',function(){try{__tbFwd();}catch(_){}});
document.getElementById('reload').addEventListener('click',function(){try{__tbReload();}catch(_){}});
document.getElementById('dl-btn').addEventListener('click',function(){try{__tbToggleDownloads();}catch(_){}});
document.getElementById('badge').addEventListener('click',function(){_goTo('ghost://privacy');});

/* ── Bookmarks ──────────────────────────────────────────────── */
var _bmEl=document.getElementById('bm');
function _bmUpdate(){
  var u=_curURL;
  if(!u||u.startsWith('ghost://')||u.startsWith('data:')){
    _bmEl.textContent='☆';_bmEl.classList.remove('bm-on');
    _bmEl.style.opacity='0.25';_bmEl.title='Cannot bookmark internal pages';return;
  }
  _bmEl.style.opacity='';_bmEl.title='Bookmark (Ctrl+D)';
  try{__tbIsBookmarked(u).then(function(yes){
    _bmEl.textContent=yes?'★':'☆';
    if(yes)_bmEl.classList.add('bm-on');else _bmEl.classList.remove('bm-on');
  });}catch(_){}
}
window._bmToggle=function(){
  var u=_curURL;
  if(!u||u.startsWith('ghost://')||u.startsWith('data:'))return;
  if(_bmEl.classList.contains('bm-on')){
    _bmEl.textContent='☆';_bmEl.classList.remove('bm-on');
    try{__tbRemoveBookmark(u);}catch(_){}
  }else{
    _bmEl.textContent='★';_bmEl.classList.add('bm-on');
    try{__tbAddBookmark(u,u);}catch(_){}
  }
};
_bmEl.addEventListener('click',window._bmToggle);

/* ── Blocked count badge ────────────────────────────────────── */
var _badgeEl=document.getElementById('badge');
function _badgeUpdate(){
  try{__tbGetBlockedCount().then(function(n){
    _badgeEl.textContent=n||0;_badgeEl.title='Blocked trackers: '+(n||0);
  });}catch(_){}
}
setInterval(_badgeUpdate,2000);

/* ── Top-edge resize detection ──────────────────────────────── */
document.addEventListener('mousemove',function(e){
  if(e.clientY<8){var x=e.clientX,w=window.innerWidth;
    document.documentElement.style.cursor=x<8?'nw-resize':x>=w-8?'ne-resize':'n-resize';
  }else{document.documentElement.style.cursor='';}
},true);
document.addEventListener('mousedown',function(e){
  if(e.button!==0||e.target.closest('button,input')||e.clientY>=8)return;
  e.preventDefault();e.stopPropagation();
  var x=e.clientX,w=window.innerWidth;
  try{__tbStartResize(x<8?13:x>=w-8?14:12);}catch(_){}
},true);

/* ── Tab drag-and-drop ───────────────────────────────────────── */
var _dg={on:false,el:null,ghost:null,fi:0,ti:0,moved:false,startX:0,pid:0};
_tablist.addEventListener('pointerdown',function(e){
  if(e.button!==0)return;
  var el=e.target.closest('.tab');if(!el||e.target.closest('.tab-x'))return;
  var kids=Array.from(_tablist.children);var fi=kids.indexOf(el);if(fi<0)return;
  _dg.on=true;_dg.el=el;_dg.fi=fi;_dg.ti=fi;_dg.moved=false;_dg.startX=e.clientX;_dg.pid=e.pointerId;
},true);
_tablist.addEventListener('pointermove',function(e){
  if(!_dg.on||!_dg.el)return;
  if(!_dg.moved&&Math.abs(e.clientX-_dg.startX)<5)return;
  if(!_dg.moved){
    _dg.moved=true;_dg.el.setPointerCapture(_dg.pid);
    var r=_dg.el.getBoundingClientRect();
    var g=_dg.el.cloneNode(true);
    g.style.cssText+=';position:fixed;z-index:99;opacity:.8;pointer-events:none;'+
      'width:'+r.width+'px;left:'+(e.clientX-r.width/2)+'px;top:'+r.top+'px;margin:0';
    document.body.appendChild(g);_dg.ghost=g;_dg.el.style.opacity='0.3';
  }
  if(_dg.ghost)_dg.ghost.style.left=(e.clientX-_dg.ghost.offsetWidth/2)+'px';
  var kids=Array.from(_tablist.children);
  for(var i=0;i<kids.length;i++){
    if(kids[i]===_dg.el)continue;
    var r2=kids[i].getBoundingClientRect();
    if(e.clientX>=r2.left&&e.clientX<r2.right&&i!==_dg.ti){
      _tablist.insertBefore(_dg.el,i<_dg.ti?kids[i]:kids[i].nextSibling||null);_dg.ti=i;break;
    }
  }
},true);
function _dgEnd(){
  if(!_dg.on)return;
  var moved=_dg.moved,fi=_dg.fi,ti=_dg.ti;
  _dg.on=false;_dg.moved=false;
  if(_dg.ghost){try{_dg.ghost.remove();}catch(_){}_dg.ghost=null;}
  if(_dg.el){_dg.el.style.opacity='';_dg.el=null;}
  if(moved&&fi!==ti){var t=_T.tabs.splice(fi,1)[0];_T.tabs.splice(ti,0,t);_render();_save();}
}
_tablist.addEventListener('pointerup',_dgEnd,true);
_tablist.addEventListener('pointercancel',function(){
  _dg.on=false;_dg.moved=false;
  if(_dg.ghost){try{_dg.ghost.remove();}catch(_){}_dg.ghost=null;}
  if(_dg.el){_dg.el.style.opacity='';_dg.el=null;}
},true);

/* ── Init ───────────────────────────────────────────────────── */
_load(function(){_render();});
try{__tbOnLoad();}catch(_){}
})();
</script>
</body>
</html>`
}

// setupContentView registers all content-side Go bindings and injects all
// persistent JS scripts into wv.  Call once per content WebView2 instance
// (the initial p.wv and every pool slot).
func (p *WebViewPanel) setupContentView(wv webview2.WebView) {
	wv.Bind("__ghostAddBookmark", func(url, title string) {
		_, _ = p.bookmarks.Add(url, title)
		p.mainWindow.Synchronize(func() {
			p.wv.Eval("if(typeof _bmBarUpdate==='function')_bmBarUpdate();")
		})
	})
	wv.Bind("__ghostRemoveBookmark", func(url string) {
		_ = p.bookmarks.RemoveByURL(url)
		p.mainWindow.Synchronize(func() {
			p.wv.Eval("if(typeof _bmBarUpdate==='function')_bmBarUpdate();")
		})
	})
	wv.Bind("__ghostGetBookmarks", func() string {
		data, _ := json.Marshal(p.bookmarks.All())
		return string(data)
	})
	wv.Bind("__ghostIsBookmarked", func(url string) bool { return p.bookmarks.Has(url) })
	wv.Bind("__ghostRecordHistory", func(url, title string) { p.browsingHist.Record(url, title) })
	wv.Bind("__ghostGetHistory", func() string {
		data, _ := json.Marshal(p.browsingHist.All())
		return string(data)
	})
	wv.Bind("__ghostDeleteHistoryEntry", func(url string) { p.browsingHist.DeleteByURL(url) })
	wv.Bind("__ghostClearHistory", func() { p.browsingHist.Clear() })
	wv.Bind("__ghostGetDownloads", func() string {
		data, _ := json.Marshal(p.dlMgr.All())
		return string(data)
	})
	wv.Bind("__ghostDownloadStarted", func(url, filename string, total int64) string {
		dir := filepath.Join(p.userDataDir, "Downloads")
		return p.dlMgr.Start(url, filename, filepath.Join(dir, filename), total)
	})
	wv.Bind("__ghostOpenFolder", func(path string) {
		if path != "" {
			_ = exec.Command("explorer", "/select,", path).Start()
		}
	})
	wv.Bind("__ghostGetBlockedCount", func() int64 { return p.blocker.BlockedCount() })
	wv.Bind("__ghostOpenDevTools", func() {
		p.mainWindow.Synchronize(func() { p.EmbedDevToolsToggle() })
	})
	wv.Bind("__ghostGetPrivacyData", func() string {
		cfg := p.buildConfig()
		prof := p.br.Profile()
		d := privacyData{
			ProfileName: prof.Name, BlockedCount: p.blocker.BlockedCount(),
			UserAgent: cfg.UserAgent, Platform: cfg.Platform, Language: cfg.Language,
			Timezone: cfg.Timezone, HardwareConcurrency: cfg.HardwareConcurrency,
			DeviceMemory: cfg.DeviceMemory, GPUVendor: cfg.GPUVendor,
			GPURenderer: cfg.GPURenderer, CanvasSeed: cfg.CanvasSeed,
			AudioSeed: cfg.AudioSeed, WebGLSeed: cfg.WebGLSeed,
		}
		b, _ := json.Marshal(d)
		return string(b)
	})
	wv.Bind("__ghostGetProfile", func() string {
		cfg := p.buildConfig()
		prof := p.br.Profile()
		d := profileDataFull{
			ProfileName: prof.Name, UserAgent: cfg.UserAgent, Platform: cfg.Platform,
			Language: cfg.Language, Languages: cfg.Languages, Timezone: cfg.Timezone,
			HardwareConcurrency: cfg.HardwareConcurrency, DeviceMemory: cfg.DeviceMemory,
			GPUVendor: cfg.GPUVendor, GPURenderer: cfg.GPURenderer,
			CanvasSeed: cfg.CanvasSeed, AudioSeed: cfg.AudioSeed,
			WebGLSeed: cfg.WebGLSeed, FontSeed: cfg.FontSeed,
		}
		b, _ := json.Marshal(d)
		return string(b)
	})
	wv.Bind("__ghostSaveProfile", func(jsonStr string) {
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
		p.auditLog.record("settings-save", "profile settings updated for "+prof.Name)
		if err := p.injectPolyfill(); err != nil {
			p.log.Warn("polyfill re-injection after settings save failed", "error", err.Error())
		}
	})
	wv.Bind("__ghostListProfiles", func() string {
		activeID := p.br.Profile().ID
		var items []profileListItem
		if p.identityStore != nil {
			if metas, err := p.identityStore.List(); err == nil {
				for _, m := range metas {
					item := profileListItem{ID: m.ID, Name: m.Name, Active: m.ID == activeID}
					if full, err2 := p.identityStore.Load(m.ID); err2 == nil {
						item.OS = full.Hardware.Platform
						item.Browser = extractBrowserName(full.Browser.UserAgent)
					}
					items = append(items, item)
				}
			}
		}
		if len(items) == 0 {
			prof := p.br.Profile()
			items = []profileListItem{{
				ID: prof.ID, Name: prof.Name,
				OS: prof.Hardware.Platform, Browser: extractBrowserName(prof.Browser.UserAgent),
				Active: true,
			}}
		}
		b, _ := json.Marshal(items)
		return string(b)
	})
	wv.Bind("__ghostSwitchProfile", func(id string) {
		if p.identityStore == nil {
			return
		}
		newProf, err := p.identityStore.Load(id)
		if err != nil {
			p.log.Warn("switch profile: load failed", "id", id, "error", err.Error())
			return
		}
		p.br.UpdateProfile(newProf)
		if p.rotState != nil {
			if rs, e2 := identity.NewRotationState(newProf, p.rotPolicy); e2 == nil {
				p.rotState = rs
			}
		}
		if err2 := p.injectPolyfill(); err2 != nil {
			p.log.Warn("polyfill re-injection after profile switch failed", "error", err2.Error())
		}
		p.auditLog.record("profile-switch", "switched to "+newProf.Name)
		p.log.Info("profile switched", "id", id, "name", newProf.Name)
	})
	wv.Bind("__ghostNavigate", p.handleGhostScheme)
	wv.Bind("__ghostGetNetworkLog", func() string {
		b, _ := json.Marshal(p.netLog.snapshot())
		return string(b)
	})
	wv.Bind("__ghostClearNetworkLog", func() { p.netLog.clear() })
	wv.Bind("__ghostGetAuditLog", func() string {
		b, _ := json.Marshal(p.auditLog.snapshot())
		return string(b)
	})
	wv.Bind("__ghostFetchBlocklist", func(url string) string {
		if err := p.blocker.FetchAndReload(url); err != nil {
			return err.Error()
		}
		p.auditLog.record("blocklist", "reloaded from "+url)
		return ""
	})
	// Inject persistent scripts.
	if err := p.injectPolyfillOn(wv); err != nil {
		p.log.Warn("polyfill injection failed for slot", "error", err.Error())
	}
	p.injectChromeOverlayOn(wv)
	p.injectResizeEdgesOn(wv)
	p.injectKeyboardShortcutsOn(wv)
	p.bindEventBridgeOn(wv)
}

func (p *WebViewPanel) bindEventBridge() { p.bindEventBridgeOn(p.wv) }

func (p *WebViewPanel) bindEventBridgeOn(wv webview2.WebView) {
	thisWV := wv // captured for active-tab guard
	wv.Bind("__ghostOnNavStart", func(url string) {
		if thisWV != p.wv {
			return
		}
		if p.OnLoadStart != nil {
			p.OnLoadStart(url)
		}
	})
	wv.Bind("__ghostOnNavDone", func(url string) {
		if thisWV != p.wv {
			return
		}
		if p.OnLoadComplete != nil {
			p.OnLoadComplete(url)
		}
	})
	wv.Bind("__ghostOnTitle", func(title string) {
		if thisWV != p.wv {
			return
		}
		if p.toolbarWv != nil {
			titleJSON, _ := json.Marshal(title)
			p.mainWindow.Synchronize(func() {
				p.toolbarWv.Eval("if(typeof _tbTitleUpdate==='function')_tbTitleUpdate(" + string(titleJSON) + ")")
			})
		}
		if p.OnTitleChange != nil {
			p.OnTitleChange(title)
		}
	})
	wv.Bind("__ghostOnURL", func(url string) {
		if thisWV != p.wv {
			return
		}
		// Skip data: URLs — the Navigate() method already pushed the ghost:// URL.
		if !strings.HasPrefix(url, "data:") {
			p.curURL = url
			p.auditLog.record("navigate", url)
			if p.toolbarWv != nil {
				urlJSON, _ := json.Marshal(url)
				p.mainWindow.Synchronize(func() {
					p.toolbarWv.Eval("if(typeof _tbNavUpdate==='function')_tbNavUpdate(" + string(urlJSON) + ")")
				})
			}
		}
		if p.OnURLChange != nil {
			p.OnURLChange(url)
		}
	})
	wv.Bind("__ghostOnError", func(url, msg string) {
		if thisWV != p.wv {
			return
		}
		if p.OnLoadError != nil {
			p.OnLoadError(url, msg)
		}
	})

	wv.Init(`(function(){
'use strict';
function _pageURL(){return window.__ghostPageURL||window.location.href;}
if(window.navigation){
  window.navigation.addEventListener('navigate',function(e){
    try{__ghostOnNavStart(e.destination.url)}catch(_){}
  });
  window.navigation.addEventListener('navigatesuccess',function(){
    var u=_pageURL();
    try{__ghostOnNavDone(u)}catch(_){}
    try{__ghostOnURL(u)}catch(_){}
  });
}
window.addEventListener('load',function(){
  var u=_pageURL();
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
    try{__ghostOnURL(_pageURL())}catch(_){}
  };
});
window.addEventListener('popstate',function(){
  try{__ghostOnURL(_pageURL())}catch(_){}
});
})();`)
}

// в"Ђв"Ђ helpers в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ

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

// в"Ђв"Ђ polyfill в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ

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
if(_ghost.timezone){var oRO=Intl.DateTimeFormat.prototype.resolvedOptions;Intl.DateTimeFormat.prototype.resolvedOptions=function(){var o=oRO.call(this);o.timeZone=_ghost.timezone;return o;};var _tzOff=(function(){try{var now=Date.now();var parts=new Intl.DateTimeFormat('en-US',{timeZone:_ghost.timezone,year:'numeric',month:'numeric',day:'numeric',hour:'numeric',minute:'numeric',second:'numeric',hour12:false}).formatToParts(new Date(now));var v={};parts.forEach(function(p){if(p.type!=='literal')v[p.type]=parseInt(p.value,10);});var tzMs=Date.UTC(v.year,v.month-1,v.day,v.hour%%24,v.minute,v.second);return Math.round((now-tzMs)/60000);}catch(e){return 0;}})();Date.prototype.getTimezoneOffset=function(){return _tzOff;};}
if(typeof navigator.getBattery==='function'){try{Object.defineProperty(navigator,'getBattery',{value:function(){return Promise.resolve({charging:true,chargingTime:0,dischargingTime:Infinity,level:1.0,addEventListener:function(){},removeEventListener:function(){},dispatchEvent:function(){return true;}});},configurable:false,writable:false,enumerable:true});}catch(e){}}
try{var _ep=Object.setPrototypeOf([],PluginArray.prototype);var _em=Object.setPrototypeOf([],MimeTypeArray.prototype);Object.defineProperty(navigator,'plugins',{get:function(){return _ep;},configurable:false,enumerable:true});Object.defineProperty(navigator,'mimeTypes',{get:function(){return _em;},configurable:false,enumerable:true});try{Object.defineProperty(navigator,'pdfViewerEnabled',{get:function(){return false;},configurable:false,enumerable:true});}catch(e){}}catch(e){}
(function(){var _mm=window.matchMedia;window.matchMedia=function(q){if(typeof q==='string'&&/prefers-color-scheme/.test(q)){var mql={matches:false,media:q,onchange:null};mql.addListener=mql.removeListener=mql.addEventListener=mql.removeEventListener=function(){};mql.dispatchEvent=function(){return false;};return mql;}return _mm.call(window,q);};})();
if(_ghost.fontSeed!==0){var _origMT=CanvasRenderingContext2D.prototype.measureText;var _fSeed=_ghost.fontSeed>>>0;CanvasRenderingContext2D.prototype.measureText=function(text){var r=_origMT.call(this,text);var h=_fSeed;for(var i=0;i<text.length;i++){h=(Math.imul(h^text.charCodeAt(i),0x9e3779b9))>>>0;}var noise=(h/0xFFFFFFFF)*0.02;var out={width:r.width+noise};['actualBoundingBoxLeft','actualBoundingBoxRight','fontBoundingBoxAscent','fontBoundingBoxDescent','actualBoundingBoxAscent','actualBoundingBoxDescent'].forEach(function(k){if(k in r)out[k]=r[k];});return out;};}
if(_ghost.canvasSeed!==0){var _oTB=HTMLCanvasElement.prototype.toBlob;HTMLCanvasElement.prototype.toBlob=function(cb,type,quality){var ctx=this.getContext('2d');if(ctx){var id=oGID.call(ctx,0,0,this.width,this.height);var s2=_s;for(var i=0;i<id.data.length;i+=4){s2=lcg(s2);id.data[i]^=(s2&0x01);id.data[i+1]^=((s2>>1)&0x01);id.data[i+2]^=((s2>>2)&0x01);}ctx.putImageData(id,0,0);}_oTB.call(this,cb,type,quality);};}
if(typeof WebGL2RenderingContext!=='undefined'){var _oGP2=WebGL2RenderingContext.prototype.getParameter;WebGL2RenderingContext.prototype.getParameter=function(p){var e=this.getExtension('WEBGL_debug_renderer_info');if(e){if(p===e.UNMASKED_VENDOR_WEBGL)return _ghost.gpuVendor;if(p===e.UNMASKED_RENDERER_WEBGL)return _ghost.gpuRenderer;}return _oGP2.call(this,p);};}
if(_ghost.audioSeed!==0&&typeof AudioBuffer!=='undefined'){var _aGCD=AudioBuffer.prototype.getChannelData;var _aSeed=_ghost.audioSeed;AudioBuffer.prototype.getChannelData=function(ch){var arr=_aGCD.call(this,ch);var s=(_aSeed^(ch*0x9e3779b9))>>>0;for(var i=0;i<arr.length;i+=100){s=(Math.imul(s,1664525)+1013904223)>>>0;arr[i]+=(s/0xFFFFFFFF-0.5)*1e-7;}return arr;};}
if(window.performance&&performance.now){var _oPNow=performance.now.bind(performance);performance.now=function(){return Math.floor(_oPNow()*10)/10;};}
try{if('connection' in navigator){Object.defineProperty(navigator,'connection',{get:function(){return undefined;},configurable:false,enumerable:true});}}catch(e){}
})();`, string(cfgJSON))
	p.wv.Init(polyfill)
	p.log.Info("polyfill injected", "profile_id", p.br.Profile().ID)
	return nil
}

func (p *WebViewPanel) injectPolyfillOn(wv webview2.WebView) error {
	orig := p.wv
	p.wv = wv
	err := p.injectPolyfill()
	p.wv = orig
	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// в"Ђв"Ђ frameless WndProc subclass в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ

func (p *WebViewPanel) subclassFrameless(hwnd win.HWND) {
	const (
		gwlpWndProc   = ^uintptr(3)
		wmSize        = uintptr(0x0005)
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
		case wmSize:
			r, _, _ := callWndProc.Call(origProc, h, msg, wp, lp)
			cw := int(lp & 0xFFFF)
			ch := int((lp >> 16) & 0xFFFF)
			// Resize toolbar HWND to match new window width.
			if p.toolbarHWND != 0 {
				win.MoveWindow(p.toolbarHWND, 0, 0, int32(cw), int32(toolbarH), true)
				win.SetWindowPos(p.toolbarHWND, win.HWND_TOP, 0, 0, 0, 0,
					win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)
			}
			// Content WebView2 always starts below the toolbar.
			if p.devToolsHWND != 0 && win.IsWindowVisible(p.devToolsHWND) {
				p.adjustDevToolsSplit(cw, ch)
			} else {
				p.setWebViewBounds(0, toolbarH, cw, ch)
			}
			return r
		}
		r, _, _ := callWndProc.Call(origProc, h, msg, wp, lp)
		return r
	})

	p.wndProcCb = cb
	setWndLongPtr.Call(uintptr(hwnd), gwlpWndProc, cb)
}

// в"Ђв"Ђ resize в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ

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

// в"Ђв"Ђ DevTools embedding в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ

// EmbedDevToolsToggle opens DevTools docked to the right of the page on the
// first call, then toggles its visibility on subsequent calls.
// Must be called on the UI thread.
func (p *WebViewPanel) EmbedDevToolsToggle() {
	if p.devToolsHWND != 0 {
		var cr win.RECT
		win.GetClientRect(p.wvHWND, &cr)
		if win.IsWindowVisible(p.devToolsHWND) {
			win.ShowWindow(p.devToolsHWND, win.SW_HIDE)
			// Restore content WebView2 to fill below toolbar.
			p.setWebViewBounds(0, toolbarH, int(cr.Right), int(cr.Bottom))
		} else {
			p.adjustDevToolsSplit(int(cr.Right), int(cr.Bottom))
		}
		return
	}
	if p.devToolsPending {
		return
	}
	c := extractChromium(p.wv)
	if c == nil {
		return
	}
	// Snapshot existing top-level windows so we can identify the new DevTools one.
	before := make(map[uintptr]bool)
	user32 := syscall.NewLazyDLL("user32.dll")
	enumWindows := user32.NewProc("EnumWindows")
	snapCb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		before[hwnd] = true
		return 1
	})
	enumWindows.Call(snapCb, 0)
	openDevToolsWindow(c)
	p.devToolsPending = true
	go p.awaitAndEmbedDevTools(before)
}

// awaitAndEmbedDevTools polls top-level windows until the DevTools window
// appears, then reparents it into the browser window.
func (p *WebViewPanel) awaitAndEmbedDevTools(before map[uintptr]bool) {
	user32 := syscall.NewLazyDLL("user32.dll")
	enumWindows := user32.NewProc("EnumWindows")
	getWindowText := user32.NewProc("GetWindowTextW")

	var foundHWND uintptr
	searchCb := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		if before[hwnd] {
			return 1
		}
		buf := make([]uint16, 256)
		getWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
		title := syscall.UTF16ToString(buf)
		if strings.Contains(title, "DevTools") {
			foundHWND = hwnd
			return 0
		}
		return 1
	})

	var devHWND win.HWND
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		foundHWND = 0
		enumWindows.Call(searchCb, 0)
		if foundHWND != 0 {
			devHWND = win.HWND(foundHWND)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	p.mainWindow.Synchronize(func() {
		p.devToolsPending = false
		if devHWND == 0 {
			return
		}
		p.devToolsHWND = devHWND

		// Convert from top-level to child window: add WS_CHILD, strip
		// WS_POPUP/WS_CAPTION/WS_THICKFRAME/WS_SYSMENU so there is no title bar.
		style := uint32(win.GetWindowLong(devHWND, win.GWL_STYLE))
		style = (style &^ uint32(win.WS_POPUP) &^ uint32(win.WS_CAPTION) &
			^uint32(win.WS_THICKFRAME) &^ uint32(win.WS_SYSMENU)) | uint32(win.WS_CHILD)
		win.SetWindowLong(devHWND, win.GWL_STYLE, int32(style))

		// Remove WS_EX_APPWINDOW so it no longer appears in the taskbar.
		const wsExAppWindow = 0x00040000
		exStyle := win.GetWindowLong(devHWND, win.GWL_EXSTYLE)
		win.SetWindowLong(devHWND, win.GWL_EXSTYLE, exStyle&^int32(wsExAppWindow))

		win.SetParent(devHWND, p.wvHWND)
		win.SetWindowPos(devHWND, 0, 0, 0, 0, 0,
			win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|
				win.SWP_NOZORDER|win.SWP_NOACTIVATE)

		var r win.RECT
		win.GetClientRect(p.wvHWND, &r)
		p.adjustDevToolsSplit(int(r.Right), int(r.Bottom))
	})
}

// adjustDevToolsSplit divides the browser window into a left content area
// (WebView2) and a right DevTools panel, then shows DevTools.
// Must be called after go-webview2's own WM_SIZE/Resize so our PutBounds
// overrides the auto-fill that Resize performs.
func (p *WebViewPanel) adjustDevToolsSplit(winW, winH int) {
	if p.devToolsHWND == 0 {
		return
	}
	dtW := winW * 2 / 5 // ~40% for DevTools, like Chrome's default
	if dtW < 320 {
		dtW = 320
	}
	if dtW > winW-200 {
		dtW = winW - 200
	}
	wvW := winW - dtW
	contentH := winH - toolbarH
	p.setWebViewBounds(0, toolbarH, wvW, winH)
	win.MoveWindow(p.devToolsHWND, int32(wvW), int32(toolbarH), int32(dtW), int32(contentH), true)
	win.ShowWindow(p.devToolsHWND, win.SW_SHOW)
	win.SetWindowPos(p.devToolsHWND, win.HWND_TOP, 0, 0, 0, 0,
		win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)
}

// setWebViewBounds calls ICoreWebView2Controller.PutBounds directly via vtable
// so the WebView2 rendering surface occupies exactly the specified rect within
// the parent window, rather than auto-filling the entire client area.
//
// PutBounds is vtable slot 6 in ICoreWebView2Controller (3 IUnknown +
// GetIsVisible + PutIsVisible + GetBounds + PutBounds).
func (p *WebViewPanel) setWebViewBounds(left, top, right, bottom int) {
	ctrl := p.getWebViewController()
	if ctrl == nil {
		return
	}
	rect := win.RECT{
		Left:   int32(left),
		Top:    int32(top),
		Right:  int32(right),
		Bottom: int32(bottom),
	}
	const putBoundsVtblIdx = uintptr(6)
	ctrlPtr := uintptr(unsafe.Pointer(ctrl))
	vtbl := *(*uintptr)(unsafe.Pointer(ctrlPtr))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + putBoundsVtblIdx*8))
	syscall.SyscallN(fn, ctrlPtr, uintptr(unsafe.Pointer(&rect)))
}

// getWebViewController extracts the *edge.ICoreWebView2Controller from the
// unexported controller field of edge.Chromium using reflection.
// setWVVisibleFor calls ICoreWebView2Controller.PutIsVisible (vtable slot 4)
// to show or hide the WebView2 rendering surface without touching the HWND.
func setWVVisibleFor(wv webview2.WebView, visible bool) {
	c := extractChromium(wv)
	if c == nil {
		return
	}
	ctrl := getCtrlFor(c)
	if ctrl == nil {
		return
	}
	const putIsVisibleIdx = uintptr(4)
	boolVal := uintptr(0)
	if visible {
		boolVal = 1
	}
	ctrlPtr := uintptr(unsafe.Pointer(ctrl))
	vtbl := *(*uintptr)(unsafe.Pointer(ctrlPtr))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + putIsVisibleIdx*8))
	syscall.SyscallN(fn, ctrlPtr, boolVal)
}

// setWVBoundsFor calls ICoreWebView2Controller.PutBounds on an arbitrary
// WebView2 instance (not necessarily p.wv).
func setWVBoundsFor(wv webview2.WebView, left, top, right, bottom int) {
	c := extractChromium(wv)
	if c == nil {
		return
	}
	ctrl := getCtrlFor(c)
	if ctrl == nil {
		return
	}
	rect := win.RECT{Left: int32(left), Top: int32(top), Right: int32(right), Bottom: int32(bottom)}
	const putBoundsVtblIdx = uintptr(6)
	ctrlPtr := uintptr(unsafe.Pointer(ctrl))
	vtbl := *(*uintptr)(unsafe.Pointer(ctrlPtr))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + putBoundsVtblIdx*8))
	syscall.SyscallN(fn, ctrlPtr, uintptr(unsafe.Pointer(&rect)))
}

// getCtrlFor extracts the ICoreWebView2Controller from any Chromium instance.
func getCtrlFor(c *edge.Chromium) *edge.ICoreWebView2Controller {
	t := reflect.TypeOf(*c)
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Name == "controller" {
			off := t.Field(i).Offset
			ptr := *(*uintptr)(unsafe.Pointer(uintptr(unsafe.Pointer(c)) + off))
			if ptr == 0 {
				return nil
			}
			return (*edge.ICoreWebView2Controller)(unsafe.Pointer(ptr))
		}
	}
	return nil
}

func (p *WebViewPanel) getWebViewController() *edge.ICoreWebView2Controller {
	c := extractChromium(p.wv)
	if c == nil {
		return nil
	}
	t := reflect.TypeOf(*c)
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Name == "controller" {
			off := t.Field(i).Offset
			ptr := *(*uintptr)(unsafe.Pointer(uintptr(unsafe.Pointer(c)) + off))
			if ptr == 0 {
				return nil
			}
			return (*edge.ICoreWebView2Controller)(unsafe.Pointer(ptr))
		}
	}
	return nil
}

// в"Ђв"Ђ ghost:// scheme в"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђв"Ђ

func (p *WebViewPanel) handleGhostScheme(url string) string {
	return fmt.Sprintf(`<p>ghost: %s</p>`, url)
}

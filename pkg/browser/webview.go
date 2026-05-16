// pkg/browser/webview.go
//go:build windows

// Package browser — WebView2 embed and browser chrome.
//
// Architecture (HTML chrome):
//
//	Walk MainWindow (full client area)
//	└── WebView2 controller (fills entire client area)
//	    └── HTML chrome overlay (position:fixed, z-index max)
//	        — address bar, nav buttons, tab strip — all rendered as HTML
//
// The browser chrome (toolbar, tabs, address bar) is implemented as a
// position:fixed HTML overlay injected into every page via
// AddScriptToExecuteOnDocumentCreated.  This eliminates the Win32
// child-window Z-order / WndProc conflicts that prevented the Walk-based
// toolbar from rendering.
package browser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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

	wndProcCb uintptr // keeps subclassed WndProc callback alive (GC guard)

	// Callbacks set by Window after construction.
	OnTitleChange  func(title string)
	OnURLChange    func(url string)
	OnLoadStart    func(url string)
	OnLoadComplete func(url string)
	OnLoadError    func(url, errMsg string)
}

// NewWebViewPanel creates WebView2 as a child of mw, filling the entire
// client area.  The browser chrome is injected as HTML.
func NewWebViewPanel(
	mw *walk.MainWindow,
	br *bridge.Bridge,
	userDataDir string,
	log *logging.Logger,
) (*WebViewPanel, error) {
	p := &WebViewPanel{
		mainWindow:  mw,
		br:          br,
		log:         log.WithComponent("webview"),
		userDataDir: userDataDir,
	}

	hwnd := unsafe.Pointer(uintptr(mw.Handle()))
	wv := webview2.NewWithOptions(webview2.WebViewOptions{
		Window:    hwnd,
		Debug:     false,
		DataPath:  userDataDir,
		AutoFocus: true,
	})
	if wv == nil {
		return nil, fmt.Errorf("webview2: failed to create instance — " +
			"ensure the WebView2 Runtime (Edge) is installed")
	}
	p.wv = wv
	p.log.Info("webview2 initialised")

	// ── Frameless chrome ──────────────────────────────────────────────────
	// go-webview2 creates its window as WS_OVERLAPPEDWINDOW (native title bar
	// + border). We strip WS_CAPTION and subclass the WndProc to:
	//   • WM_NCCALCSIZE: return 0 so the entire window rect is client area,
	//     eliminating the non-client border strip completely.
	//   • WM_NCHITTEST: return resize hit-values for the 8-px edge zone;
	//     return HTCLIENT for everything else so mouse events reach WebView2,
	//     which handles tab-bar dragging internally via -webkit-app-region:drag.
	// DwmSetWindowAttribute restores the DWM shadow and Windows 11 rounded
	// corners that WS_CAPTION removal would otherwise kill.
	wvHWND := win.HWND(uintptr(p.wv.Window()))
	wvStyle := win.GetWindowLong(wvHWND, win.GWL_STYLE)
	win.SetWindowLong(wvHWND, win.GWL_STYLE, wvStyle&^win.WS_CAPTION|win.WS_CLIPCHILDREN)
	win.SetWindowPos(wvHWND, 0, 0, 0, 0, 0,
		win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)
	p.subclassFrameless(wvHWND)
	dwmapi := syscall.NewLazyDLL("dwmapi.dll")
	// Restore DWM shadow (disabled when WS_CAPTION is removed).
	ncPolicy := uint32(2) // DWMNCRP_ENABLED
	dwmapi.NewProc("DwmSetWindowAttribute").Call(
		uintptr(wvHWND), 2, uintptr(unsafe.Pointer(&ncPolicy)), 4)
	// Windows 11: keep rounded corners.
	cornerPref := uint32(2) // DWMWCP_ROUND
	dwmapi.NewProc("DwmSetWindowAttribute").Call(
		uintptr(wvHWND), 33, uintptr(unsafe.Pointer(&cornerPref)), 4)

	// ── Window management bindings ────────────────────────────────────────
	// go-webview2 always creates its own top-level window regardless of the
	// Window option passed to NewWithOptions.  wv.Window() is that window's
	// HWND — the one the user actually sees and interacts with.  All min/max/
	// close operations must target it, not Walk's MainWindow HWND.
	const swMaximize = 3 // SW_SHOWMAXIMIZED — used to detect current state

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

	if err := p.injectPolyfill(); err != nil {
		p.log.Warn("polyfill injection failed", "error", err.Error())
	}
	p.injectChromeOverlay()
	p.bindEventBridge()
	p.wv.Bind("__ghostNavigate", p.handleGhostScheme)

	mw.SizeChanged().Attach(p.onResize)
	return p, nil
}

// ── navigation ────────────────────────────────────────────────────────────────

func (p *WebViewPanel) Navigate(url string) {
	if p.wv == nil {
		return
	}
	p.log.Info("navigate", "url", url)
	if strings.HasPrefix(url, "ghost://") {
		html := p.ghostPageHTML(url)
		encoded := base64.StdEncoding.EncodeToString([]byte(html))
		p.wv.Navigate("data:text/html;base64," + encoded)
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

// injectChromeOverlay injects a position:fixed browser chrome into every
// page via AddScriptToExecuteOnDocumentCreated.  The overlay contains the
// address bar, navigation buttons, and a single-tab strip.
//
// Ghost:// internal pages (served as data: URLs) skip the overlay because
// they include the chrome in their own HTML.
func (p *WebViewPanel) injectChromeOverlay() {
	profileName := p.br.Profile().Name

	// %q embeds the profile name safely; %% produces a literal % in CSS.
	script := fmt.Sprintf(`(function(){
'use strict';
if(window.location.href.startsWith('data:')||window.location.href==='about:blank')return;
if(document.getElementById('_gs_toolbar'))return;
var CHROME_H=82;
var profile=%q;
var styleEl=document.createElement('style');
styleEl.textContent=[
  '#_gs_toolbar{position:fixed;top:0;left:0;right:0;height:'+CHROME_H+'px;',
  'background:linear-gradient(90deg,#3D1A0A 0%%,#2A1560 40%%,#0A1A6B 70%%,#050E40 100%%);',
  'border-bottom:1px solid rgba(255,255,255,.1);z-index:2147483647;',
  'display:flex;flex-direction:column;box-shadow:0 2px 12px rgba(0,0,0,.5);}',
  '#_gs_tabs{height:38px;display:flex;align-items:flex-end;padding:0 0 0 8px;gap:2px;',
  '-webkit-app-region:drag;}',  /* drag the window by the tab bar background */
  /* interactive children must opt out of dragging */
  '#_gs_tab,#_gs_new_tab,#_gs_wm_btns,._gs_wm_btn{-webkit-app-region:no-drag;}',
  '#_gs_tab{background:rgba(255,255,255,.18);border:1px solid rgba(255,255,255,.2);',
  'border-bottom:none;border-radius:8px 8px 0 0;padding:0 12px;height:30px;',
  'display:flex;align-items:center;font-size:12px;color:white;',
  'max-width:220px;white-space:nowrap;overflow:hidden;}',
  '#_gs_tab_title{overflow:hidden;text-overflow:ellipsis;flex:1;}',
  '#_gs_new_tab{background:transparent;border:none;color:rgba(255,255,255,.6);',
  'font-size:20px;cursor:pointer;padding:0 8px;border-radius:50%;align-self:center;line-height:1;}',
  '#_gs_new_tab:hover{background:rgba(255,255,255,.15);color:white;}',
  '#_gs_nav{height:44px;display:flex;align-items:center;padding:0 10px;gap:6px;}',
  '._gs_btn{width:28px;height:28px;border:none;background:transparent;',
  'border-radius:50%%;cursor:pointer;font-size:16px;color:rgba(255,255,255,.8);',
  'display:flex;align-items:center;justify-content:center;}',
  '._gs_btn:hover{background:rgba(255,255,255,.15);color:white;}',
  '#_gs_addr{flex:1;height:30px;border:1px solid rgba(255,255,255,.2);',
  'border-radius:15px;padding:0 14px;font-size:13px;',
  'background:rgba(255,255,255,.12);color:white;outline:none;}',
  '#_gs_addr::placeholder{color:rgba(255,255,255,.4);}',
  '#_gs_addr:focus{border-color:rgba(130,150,255,.8);background:rgba(255,255,255,.18);}',
  '#_gs_badge{width:10px;height:10px;border-radius:50%%;background:#F4A460;',
  'flex-shrink:0;box-shadow:0 0 4px rgba(244,164,96,.6);}',
  /* Window control buttons (close / maximise / minimise) */
  '#_gs_wm_btns{display:flex;align-items:stretch;margin-left:auto;height:38px;-webkit-app-region:no-drag;}',
  '._gs_wm_btn{width:46px;height:100%%;border:none;background:transparent;',
  'color:rgba(255,255,255,.85);font-size:13px;cursor:pointer;',
  'display:flex;align-items:center;justify-content:center;}',
  '._gs_wm_btn:hover{background:rgba(255,255,255,.2);}',
  '#_gs_cls:hover{background:#E81123!important;color:#fff;}',
  'body{padding-top:'+CHROME_H+'px!important;}'
].join('');
var bar=document.createElement('div');
bar.id='_gs_toolbar';
bar.innerHTML=
  '<div id="_gs_tabs">'+
    '<div id="_gs_tab"><span id="_gs_tab_title">Loading\u2026</span></div>'+
    '<button id="_gs_new_tab" title="New Tab">+</button>'+
    '<div id="_gs_wm_btns">'+
      '<button class="_gs_wm_btn" id="_gs_min" title="Minimise">&#8212;</button>'+
      '<button class="_gs_wm_btn" id="_gs_max" title="Maximise">&#9633;</button>'+
      '<button class="_gs_wm_btn" id="_gs_cls" title="Close">&#10005;</button>'+
    '</div>'+
  '</div>'+
  '<div id="_gs_nav">'+
    '<button class="_gs_btn" id="_gs_back" title="Back">&#8592;</button>'+
    '<button class="_gs_btn" id="_gs_fwd" title="Forward">&#8594;</button>'+
    '<button class="_gs_btn" id="_gs_reload" title="Reload">&#8635;</button>'+
    '<input id="_gs_addr" type="text" spellcheck="false" placeholder="Search or enter address"/>'+
    '<div id="_gs_badge" title="Profile: '+profile+'"></div>'+
  '</div>';
function mount(){
  if(document.getElementById('_gs_toolbar'))return;
  document.head.appendChild(styleEl);
  document.body.insertAdjacentElement('afterbegin',bar);
  var addr=document.getElementById('_gs_addr');
  addr.value=location.href;
  function updateTitle(){
    var t=document.getElementById('_gs_tab_title');
    if(t)t.textContent=document.title||location.hostname||'New Tab';
  }
  updateTitle();
  new MutationObserver(updateTitle).observe(document.querySelector('title')||document.head,{childList:true,characterData:true,subtree:true});
  document.getElementById('_gs_tabs').addEventListener('mousedown',function(e){
    if(e.button===0&&!e.target.closest('button,input,a')){try{__ghostStartDrag();}catch(_){}}
  });
  document.getElementById('_gs_back').addEventListener('click',function(){history.back();});
  document.getElementById('_gs_fwd').addEventListener('click',function(){history.forward();});
  document.getElementById('_gs_reload').addEventListener('click',function(){location.reload();});
  document.getElementById('_gs_min').addEventListener('click',function(){try{__ghostMinimize();}catch(e){}});
  document.getElementById('_gs_max').addEventListener('click',function(){try{__ghostMaximize();}catch(e){}});
  document.getElementById('_gs_cls').addEventListener('click',function(){try{__ghostClose();}catch(e){}});
  addr.addEventListener('keydown',function(e){
    if(e.key!=='Enter')return;
    e.preventDefault();
    var raw=addr.value.trim();if(!raw)return;
    var url=raw;
    if(!url.match(/^https?:\/\//i)&&!url.startsWith('ghost://')){
      if(url.indexOf('.')>=0&&url.indexOf(' ')<0){url='https://'+url;}
      else{url='https://search.brave.com/search?q='+encodeURIComponent(url);}
    }
    location.href=url;
  });
  addr.addEventListener('focus',function(){addr.select();});
  window.addEventListener('load',function(){addr.value=location.href;updateTitle();});
  window.addEventListener('popstate',function(){addr.value=location.href;});
  // New tab navigates the current page (single WebView2 instance)
  document.getElementById('_gs_new_tab').addEventListener('click',function(){
    location.href='ghost://newtab';
  });
}
if(document.readyState==='loading'){document.addEventListener('DOMContentLoaded',mount);}
else{mount();}
})();`, profileName)

	p.wv.Init(script)
}

// ghostPageHTML returns fully self-contained HTML for ghost:// internal URLs.
// Uses strings.ReplaceAll to avoid fmt.Sprintf misinterpreting CSS/JS percent signs.
func (p *WebViewPanel) ghostPageHTML(url string) string {
	page := strings.TrimPrefix(url, "ghost://")
	profileName := p.br.Profile().Name

	var body string
	switch page {
	case "settings":
		body = "<h2>Settings</h2><p>Settings UI - Phase 2.</p>"
	case "newtab", "":
		body = "<h2>Ghost-Silicon</h2><p style=\"margin-top:10px;opacity:.7\">Type an address above and press Enter.</p>"
	case "network":
		body = "<h2>Network Monitor</h2><p>Coming in Phase 2.</p>"
	case "audit":
		body = "<h2>Audit Log</h2><p>Coming in Phase 2.</p>"
	default:
		body = "<h2>" + page + "</h2><p>Page not found.</p>"
	}

	// NOTE: % characters in the template are literal CSS/JS — safe because
	// we use strings.ReplaceAll, not fmt.Sprintf.
	const tmpl = `<!DOCTYPE html><html><head>
<meta charset="utf-8"><title>ghost://GSPAGE</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Segoe UI',system-ui,sans-serif;
  background:linear-gradient(160deg,#1a0a0a 0%,#0d0d30 50%,#050a28 100%);
  color:#E8E8F4;min-height:100vh}
#chrome{position:fixed;top:0;left:0;right:0;height:82px;
  background:linear-gradient(90deg,#3D1A0A 0%,#2A1560 40%,#0A1A6B 70%,#050E40 100%);
  border-bottom:1px solid rgba(255,255,255,.1);z-index:9999;
  display:flex;flex-direction:column;box-shadow:0 2px 12px rgba(0,0,0,.5)}
#tabs{height:38px;display:flex;align-items:flex-end;padding:0 0 0 8px;gap:2px;
  -webkit-app-region:drag}
.tab,.ntbtn,#wm{-webkit-app-region:no-drag}
.tab{background:rgba(255,255,255,.18);border:1px solid rgba(255,255,255,.2);
  border-bottom:none;border-radius:8px 8px 0 0;padding:0 12px;height:30px;
  display:flex;align-items:center;font-size:12px;color:white;
  max-width:220px;white-space:nowrap;overflow:hidden}
.ntbtn{background:transparent;border:none;color:rgba(255,255,255,.6);
  font-size:20px;cursor:pointer;padding:0 8px;border-radius:50%;
  align-self:center;line-height:1}
.ntbtn:hover{background:rgba(255,255,255,.15);color:white}
#wm{display:flex;align-items:stretch;margin-left:auto;height:38px}
.wbtn{width:46px;height:100%;border:none;background:transparent;
  color:rgba(255,255,255,.85);font-size:13px;cursor:pointer;
  display:flex;align-items:center;justify-content:center}
.wbtn:hover{background:rgba(255,255,255,.2)}
#wcls:hover{background:#E81123!important;color:#fff}
#nav{height:44px;display:flex;align-items:center;padding:0 10px;gap:6px}
.btn{width:28px;height:28px;border:none;background:transparent;border-radius:50%;
  cursor:pointer;font-size:16px;color:rgba(255,255,255,.8);
  display:flex;align-items:center;justify-content:center}
.btn:hover{background:rgba(255,255,255,.15);color:white}
#addr{flex:1;height:30px;border:1px solid rgba(255,255,255,.2);border-radius:15px;
  padding:0 14px;font-size:13px;background:rgba(255,255,255,.12);color:white;outline:none}
#addr:focus{border-color:rgba(130,150,255,.8);background:rgba(255,255,255,.18)}
#badge{width:10px;height:10px;border-radius:50%;background:#F4A460;flex-shrink:0;
  box-shadow:0 0 4px rgba(244,164,96,.6)}
#content{padding-top:82px;min-height:100vh;display:flex;align-items:center;
  justify-content:center;text-align:center;padding:82px 20px 20px}
#content h2{font-size:2rem;font-weight:700;
  background:linear-gradient(90deg,#F4A460,#A080FF);
  -webkit-background-clip:text;-webkit-text-fill-color:transparent}
</style></head><body>
<div id="chrome">
  <div id="tabs" onmousedown="if(event.button===0&&!event.target.closest('button,input,a')){try{__ghostStartDrag();}catch(_){}}">
    <div class="tab"><span>ghost://GSPAGE</span></div>
    <button class="ntbtn" title="New Tab" onclick="location.href='ghost://newtab'">+</button>
    <div id="wm">
      <button class="wbtn" title="Minimise" onclick="try{__ghostMinimize();}catch(e){}">&#8212;</button>
      <button class="wbtn" title="Maximise" onclick="try{__ghostMaximize();}catch(e){}">&#9633;</button>
      <button class="wbtn" id="wcls" title="Close" onclick="try{__ghostClose();}catch(e){}">&#10005;</button>
    </div>
  </div>
  <div id="nav">
    <button class="btn" onclick="history.back()" title="Back">&#8592;</button>
    <button class="btn" onclick="history.forward()" title="Forward">&#8594;</button>
    <button class="btn" onclick="location.reload()" title="Reload">&#8635;</button>
    <input id="addr" type="text" spellcheck="false"
      placeholder="Search or enter address" value="ghost://GSPAGE"
      onkeydown="if(event.key==='Enter'){var u=this.value.trim();if(!u)return;if(!u.match(/^https?:\/\//i)&&!u.startsWith('ghost://')){if(u.indexOf('.')>=0&&u.indexOf(' ')<0){u='https://'+u;}else{u='https://search.brave.com/search?q='+encodeURIComponent(u);}}location.href=u;}"
    />
    <div id="badge" title="Profile: GSPROFILE"></div>
  </div>
</div>
<div id="content">GSBODY</div>
</body></html>`

	html := strings.ReplaceAll(tmpl, "GSPAGE", page)
	html = strings.ReplaceAll(html, "GSPROFILE", profileName)
	html = strings.ReplaceAll(html, "GSBODY", body)
	return html
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

// subclassFrameless installs a WndProc on the go-webview2 host window that
// eliminates the non-client border strip and provides correct resize hit
// targets, while leaving HTCLIENT for the interior so WebView2 receives all
// mouse events and can handle -webkit-app-region:drag internally.
func (p *WebViewPanel) subclassFrameless(hwnd win.HWND) {
	const (
		gwlpWndProc   = ^uintptr(3) // -4: index for the window procedure
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
				// Claim the entire window rect as client area — this removes
				// the leftover non-client strip that WS_THICKFRAME adds after
				// WS_CAPTION is stripped.
				return 0
			}
		case wmNcHitTest:
			var wr win.RECT
			win.GetWindowRect(win.HWND(h), &wr)
			// lp = MAKELONG(xScreen, yScreen)
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

// ForceResize explicitly sizes WebView2 after Walk's first layout pass.
func (p *WebViewPanel) ForceResize() { p.onResize() }

// ── ghost:// scheme ───────────────────────────────────────────────────────────

// handleGhostScheme is bound to JavaScript as __ghostNavigate.
func (p *WebViewPanel) handleGhostScheme(url string) string {
	return fmt.Sprintf(`<p>ghost: %s</p>`, url)
}

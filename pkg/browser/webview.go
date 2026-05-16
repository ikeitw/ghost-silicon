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
	tabsJSON  string  // JSON tab state persisted across navigations

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
	// go-webview2 creates its window as WS_OVERLAPPEDWINDOW (native title bar
	// + border). We strip WS_CAPTION and subclass the WndProc to:
	//   • WM_NCCALCSIZE: return 0 so the entire window rect is client area,
	//     eliminating the non-client border strip completely.
	//   • WM_NCHITTEST: return resize hit-values for the 8-px edge zone;
	//     return HTCLIENT for everything else so mouse events reach WebView2.
	// DwmSetWindowAttribute restores the DWM shadow and Windows 11 rounded
	// corners that WS_CAPTION removal would otherwise kill.
	wvHWND := win.HWND(uintptr(p.wv.Window()))
	wvStyle := win.GetWindowLong(wvHWND, win.GWL_STYLE)
	win.SetWindowLong(wvHWND, win.GWL_STYLE, wvStyle&^win.WS_CAPTION|win.WS_CLIPCHILDREN)
	win.SetWindowPos(wvHWND, 0, 0, 0, 0, 0,
		win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)
	p.subclassFrameless(wvHWND)
	dwmapi := syscall.NewLazyDLL("dwmapi.dll")
	ncPolicy := uint32(2) // DWMNCRP_ENABLED — restore DWM shadow
	dwmapi.NewProc("DwmSetWindowAttribute").Call(
		uintptr(wvHWND), 2, uintptr(unsafe.Pointer(&ncPolicy)), 4)
	cornerPref := uint32(2) // DWMWCP_ROUND — Windows 11 rounded corners
	dwmapi.NewProc("DwmSetWindowAttribute").Call(
		uintptr(wvHWND), 33, uintptr(unsafe.Pointer(&cornerPref)), 4)

	// ── Window management bindings ────────────────────────────────────────
	const swMaximize = 3 // SW_SHOWMAXIMIZED

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
	// Tab state — persisted in Go so it survives page navigations.
	p.tabsJSON = `{"tabs":[{"id":1,"url":"ghost://newtab","title":"New Tab"}],"current":1}`
	p.wv.Bind("__ghostGetTabs", func() string {
		return p.tabsJSON
	})
	p.wv.Bind("__ghostSetTabs", func(json string) {
		p.tabsJSON = json
	})
	// Resize — WebView2's child HWND covers the entire frame, so WM_NCHITTEST
	// in the host WndProc never fires for border regions.  The JS overlay
	// detects cursor proximity to viewport edges and calls this binding to
	// start a native resize via WM_NCLBUTTONDOWN.
	p.wv.Bind("__ghostStartResize", func(ht int) {
		var pt win.POINT
		win.GetCursorPos(&pt)
		lp := uintptr(pt.Y)<<16 | uintptr(uint16(pt.X))
		win.ReleaseCapture()
		win.PostMessage(wvHWND, win.WM_NCLBUTTONDOWN, uintptr(ht), lp)
	})

	if err := p.injectPolyfill(); err != nil {
		p.log.Warn("polyfill injection failed", "error", err.Error())
	}
	p.injectChromeOverlay()
	p.injectResizeEdges()
	p.injectKeyboardShortcuts()
	p.bindEventBridge()
	p.wv.Bind("__ghostNavigate", p.handleGhostScheme)

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

// injectChromeOverlay injects a position:fixed browser chrome into every page
// (including ghost:// data: pages) via AddScriptToExecuteOnDocumentCreated.
// The overlay manages a real multi-tab strip backed by Go-side persisted state.
//
// Robustness design: event listeners are attached directly to the bar element
// once (they persist through DOM removal/re-insertion). mount() only handles
// DOM insertion. A 500 ms heartbeat re-inserts the toolbar if a page's JS
// removes it (e.g. YouTube SPA hydration).
func (p *WebViewPanel) injectChromeOverlay() {
	profileName := p.br.Profile().Name

	// %q embeds the profile name safely.
	// CSS percent signs must be written as %% so fmt.Sprintf passes them through.
	script := fmt.Sprintf(`(function(){
'use strict';
try{if(window!==window.top)return;}catch(e){return;}
var H=82,profile=%q;

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
  '._gst{background:rgba(255,255,255,.12);border:1px solid rgba(255,255,255,.15);border-bottom:none;'+
  'border-radius:8px 8px 0 0;padding:0 4px 0 10px;height:30px;display:flex;align-items:center;'+
  'font-size:12px;color:rgba(255,255,255,.75);max-width:180px;min-width:80px;cursor:pointer;'+
  '-webkit-app-region:no-drag;flex-shrink:0}'+
  '._gst._gst_a{background:rgba(255,255,255,.2);color:#fff;border-color:rgba(255,255,255,.25)}'+
  '._gst:hover{background:rgba(255,255,255,.16)}'+
  '._gst>span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1}'+
  '._gst_x{background:transparent;border:none;color:rgba(255,255,255,.4);font-size:11px;'+
  'cursor:pointer;padding:0 3px;margin-left:2px;border-radius:3px;flex-shrink:0;line-height:1.5}'+
  '._gst_x:hover{background:rgba(255,255,255,.2);color:#fff}'+
  '#_gs_new_tab{background:transparent;border:none;color:rgba(255,255,255,.6);'+
  'font-size:20px;cursor:pointer;padding:0 8px;border-radius:50%%;align-self:center;line-height:1;flex-shrink:0}'+
  '#_gs_new_tab:hover{background:rgba(255,255,255,.15);color:#fff}'+
  '#_gs_nav{height:44px;display:flex;align-items:center;padding:0 10px;gap:6px}'+
  '._gs_btn{width:28px;height:28px;border:none;background:transparent;border-radius:50%%;cursor:pointer;'+
  'font-size:16px;color:rgba(255,255,255,.8);display:flex;align-items:center;justify-content:center}'+
  '._gs_btn:hover{background:rgba(255,255,255,.15);color:#fff}'+
  '#_gs_addr{flex:1;height:30px;border:1px solid rgba(255,255,255,.2);border-radius:15px;'+
  'padding:0 14px;font-size:13px;background:rgba(255,255,255,.12);color:#fff;outline:none}'+
  '#_gs_addr::placeholder{color:rgba(255,255,255,.4)}'+
  '#_gs_addr:focus{border-color:rgba(130,150,255,.8);background:rgba(255,255,255,.18)}'+
  '#_gs_badge{width:10px;height:10px;border-radius:50%%;background:#F4A460;'+
  'flex-shrink:0;box-shadow:0 0 4px rgba(244,164,96,.6)}'+
  '#_gs_wm_btns{display:flex;align-items:stretch;margin-left:auto;height:38px;-webkit-app-region:no-drag}'+
  '._gs_wm_btn{width:46px;height:100%%;border:none;background:transparent;'+
  'color:rgba(255,255,255,.85);font-size:13px;cursor:pointer;'+
  'display:flex;align-items:center;justify-content:center}'+
  '._gs_wm_btn:hover{background:rgba(255,255,255,.2)}'+
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
  '<div id="_gs_badge" title="Profile: '+profile+'"></div></div>';
/* Apply critical layout styles directly — bypasses any page Content-Security-Policy
   that might block our <style> element injection. */
bar.style.cssText=
  'position:fixed;top:0;left:0;right:0;'+
  'height:'+H+'px;z-index:2147483647;'+
  'display:flex;flex-direction:column;'+
  'background:linear-gradient(90deg,#3D1A0A 0%%,#2A1560 40%%,#0A1A6B 70%%,#050E40 100%%);'+
  'border-bottom:1px solid rgba(255,255,255,.1);box-shadow:0 2px 12px rgba(0,0,0,.5)';

/* ── Direct element refs from bar (valid even when bar is detached) ── */
var _tablist=bar.querySelector('#_gs_tablist');
var _addr=bar.querySelector('#_gs_addr');
var _tabTitle=bar.querySelector('#_gs_tab_title');

/* ── Tab state ──────────────────────────────────────────────── */
var _T={tabs:[],current:0};
function _url(){return window.__ghostPageURL||location.href;}
function _load(cb){
  try{__ghostGetTabs().then(function(j){
    try{_T=JSON.parse(j);}catch(_){_T=null;}
    if(!_T||!_T.tabs||!_T.tabs.length){_T={tabs:[{id:1,url:_url(),title:'New Tab'}],current:1};}
    if(cb)cb();
  });}catch(e){
    if(!_T||!_T.tabs||!_T.tabs.length){_T={tabs:[{id:1,url:_url(),title:'New Tab'}],current:1};}
    if(cb)cb();
  }
}
function _save(){try{__ghostSetTabs(JSON.stringify(_T));}catch(_){}}
function _render(){
  if(!_T.tabs||!_T.tabs.length)return;
  _tablist.innerHTML='';
  _T.tabs.forEach(function(t){
    var el=document.createElement('div');
    el.className=t.id===_T.current?'_gst _gst_a':'_gst';
    var sp=document.createElement('span');sp.textContent=t.title||'New Tab';sp.title=t.title||'';
    var xb=document.createElement('button');xb.className='_gst_x';xb.innerHTML='&#10005;';xb.title='Close';
    el.appendChild(sp);el.appendChild(xb);
    (function(id){
      el.addEventListener('click',function(e){if(xb.contains(e.target))return;_switchTab(id);});
      xb.addEventListener('click',function(e){e.stopPropagation();_closeTab(id);});
    })(t.id);
    _tablist.appendChild(el);
  });
}
function _updateCur(){
  var u=_url(),ti=document.title||location.hostname||'New Tab';
  for(var i=0;i<_T.tabs.length;i++){if(_T.tabs[i].id===_T.current){_T.tabs[i].url=u;_T.tabs[i].title=ti;break;}}
}
function _switchTab(id){
  _updateCur();_T.current=id;_save();
  for(var i=0;i<_T.tabs.length;i++){if(_T.tabs[i].id===id){location.href=_T.tabs[i].url;return;}}
}
window._gsNewTab=function(){
  _updateCur();
  var mx=0;_T.tabs.forEach(function(t){if(t.id>mx)mx=t.id;});
  var id=mx+1;_T.tabs.push({id:id,url:'ghost://newtab',title:'New Tab'});
  _T.current=id;_save();location.href='ghost://newtab';
};
window._gsCloseCurrentTab=function(){_closeTab(_T.current);};
function _closeTab(id){
  if(_T.tabs.length<=1){try{__ghostClose();}catch(_){}return;}
  var idx=-1;for(var i=0;i<_T.tabs.length;i++){if(_T.tabs[i].id===id){idx=i;break;}}
  if(idx<0)return;
  var wa=(id===_T.current);
  _T.tabs.splice(idx,1);
  if(wa){var ni=Math.min(idx,_T.tabs.length-1);_T.current=_T.tabs[ni].id;_save();location.href=_T.tabs[ni].url;}
  else{_save();_render();}
}

/* ── Wire all event listeners onto bar elements once ────────── */
/* These survive DOM removal/re-insertion since they live on the element. */
function _mx(){
  var b=bar.querySelector('#_gs_max');if(!b)return;
  try{__ghostIsMaximized().then(function(m){
    b.innerHTML=m?'❐':'□';b.title=m?'Restore':'Maximise';
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
bar.querySelector('#_gs_new_tab').addEventListener('click',function(){_gsNewTab();});
bar.querySelector('#_gs_min').addEventListener('click',function(){try{__ghostMinimize();}catch(e){}});
bar.querySelector('#_gs_max').addEventListener('click',function(){_mx();try{__ghostMaximize();}catch(e){}});
bar.querySelector('#_gs_cls').addEventListener('click',function(){try{__ghostClose();}catch(e){}});
_addr.addEventListener('keydown',function(e){
  if(e.key!=='Enter')return;e.preventDefault();
  var r=_addr.value.trim();if(!r)return;var u=r;
  if(!u.match(/^https?:\/\//i)&&!u.startsWith('ghost://')){
    if(u.indexOf('.')>=0&&u.indexOf(' ')<0){u='https://'+u;}
    else{u='https://duckduckgo.com/?q='+encodeURIComponent(u);}
  }
  location.href=u;
});
_addr.addEventListener('focus',function(){_addr.select();});

/* ── Title watcher (set up once) ────────────────────────────── */
var _titleWatched=false;
function _watchTitle(){
  if(_titleWatched)return;_titleWatched=true;
  var t=document.querySelector('title');
  if(t){new MutationObserver(function(){_updateCur();_render();_save();})
    .observe(t,{childList:true,characterData:true,subtree:true});}
}

/* ── Mount: only DOM insertion + one-time state init ────────── */
var _ready=false;
function mount(){
  if(!document.documentElement)return;
  /* Ensure style is in <head> (supplements inline styles with hover/focus rules). */
  if(!document.getElementById('_gs_st')){
    try{if(document.head)document.head.appendChild(s);}catch(_){}
  }
  /* Always re-assert critical inline styles — if the page reset them. */
  bar.style.setProperty('position','fixed','important');
  bar.style.setProperty('top','0','important');
  bar.style.setProperty('left','0','important');
  bar.style.setProperty('right','0','important');
  bar.style.setProperty('height',H+'px','important');
  bar.style.setProperty('z-index','2147483647','important');
  bar.style.setProperty('display','flex','important');
  bar.style.setProperty('flex-direction','column','important');
  /* Body padding via inline JS — bypasses page CSP and any author !important. */
  try{
    if(document.body)document.body.style.setProperty('padding-top',H+'px','important');
  }catch(_){}
  /* Re-insert toolbar if missing (handles YouTube-style body replacement). */
  if(!document.getElementById('_gs_toolbar')){
    try{document.documentElement.appendChild(bar);}catch(_){return;}
    _addr.value=_url();
    if(!_ready){
      _ready=true;
      if(_tabTitle)_tabTitle.textContent=document.title||location.hostname||_url();
      _load(function(){_updateCur();_render();_save();});
      _watchTitle();
    }else{
      /* Re-render existing tab state after re-insertion. */
      _render();
    }
  }
}

window.addEventListener('load',function(){
  mount();
  _addr.value=_url();
  _load(function(){_updateCur();_render();_save();});
});
window.addEventListener('popstate',function(){_addr.value=_url();_updateCur();_save();});

if(document.readyState==='loading'){document.addEventListener('DOMContentLoaded',mount);}
else{mount();}

/* Heartbeat: re-inject toolbar if the page removes it (YouTube, SPAs, etc.) */
setInterval(function(){mount();},500);
})();`, profileName)

	p.wv.Init(script)
}

// injectResizeEdges injects a JS listener that detects cursor proximity to
// viewport edges and calls __ghostStartResize, bypassing the WndProc
// WM_NCHITTEST path which WebView2's child HWND blocks.
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
  }
});
})();`)
}

// ghostPageHTML returns fully self-contained HTML for ghost:// internal URLs.
// The chrome overlay (toolbar, tabs, address bar) is injected separately by
// injectChromeOverlay, so these pages only contain page content and styling.
func (p *WebViewPanel) ghostPageHTML(url string) string {
	page := strings.TrimPrefix(url, "ghost://")

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

	// __ghostPageURL is read by the chrome overlay to display the ghost://
	// URL in the address bar instead of the raw data: URL.
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
// mouse events.  Actual border resizing is handled by injectResizeEdges via
// JS, since WebView2's child HWND intercepts the border mouse events before
// the host WndProc sees them.
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

// ── resize (legacy — now superseded by injectResizeEdges) ────────────────────

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

func (p *WebViewPanel) handleGhostScheme(url string) string {
	return fmt.Sprintf(`<p>ghost: %s</p>`, url)
}

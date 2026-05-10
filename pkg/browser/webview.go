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
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"github.com/lxn/walk"

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

	script := fmt.Sprintf(`(function(){
'use strict';

// Skip injection on our own internal pages.
if(window.location.href.startsWith('data:')||window.location.href==='about:blank')return;

// Avoid double-injection on SPA navigations.
if(document.getElementById('_gs_toolbar'))return;

var CHROME_H = 82;
var profile  = %q;

// ── Styles ─────────────────────────────────────────────────────────────────
var css = [
  '#_gs_toolbar{position:fixed;top:0;left:0;right:0;height:'+CHROME_H+'px;',
  'background:#EDEEF2;border-bottom:1px solid #C8C8D2;z-index:2147483647;',
  'display:flex;flex-direction:column;font-family:"Segoe UI",system-ui,sans-serif;',
  'box-shadow:0 1px 3px rgba(0,0,0,.12);}',

  '#_gs_tabs{height:38px;display:flex;align-items:flex-end;padding:0 8px 0 8px;gap:2px;}',

  '#_gs_tab{background:#fff;border:1px solid #C8C8D2;border-bottom:none;',
  'border-radius:8px 8px 0 0;padding:0 12px;height:30px;display:flex;',
  'align-items:center;font-size:12px;max-width:200px;gap:6px;',
  'white-space:nowrap;overflow:hidden;}',

  '#_gs_tab_title{overflow:hidden;text-overflow:ellipsis;flex:1;}',

  '#_gs_new_tab{width:24px;height:24px;border:none;background:transparent;',
  'border-radius:50%;cursor:pointer;font-size:18px;line-height:24px;',
  'text-align:center;color:#666;margin-left:4px;align-self:center;}',
  '#_gs_new_tab:hover{background:#D8D8E0;}',

  '#_gs_nav{height:44px;display:flex;align-items:center;padding:0 10px;gap:6px;}',

  '._gs_btn{width:28px;height:28px;border:none;background:transparent;',
  'border-radius:50%;cursor:pointer;font-size:15px;display:flex;',
  'align-items:center;justify-content:center;color:#333;}',
  '._gs_btn:hover{background:#D0D0DA;}',
  '._gs_btn:disabled{opacity:.35;cursor:default;}',

  '#_gs_addr{flex:1;height:30px;border:1px solid #C0C0CA;border-radius:15px;',
  'padding:0 14px;font-size:13px;background:#fff;outline:none;}',
  '#_gs_addr:focus{border-color:#4285F4;box-shadow:0 0 0 2px rgba(66,133,244,.2);}',

  '#_gs_badge{width:10px;height:10px;border-radius:50%;background:#32B264;',
  'flex-shrink:0;cursor:default;margin-left:2px;}',

  'body{padding-top:'+CHROME_H+'px!important;}'
].join('');

var styleEl = document.createElement('style');
styleEl.textContent = css;

// ── HTML ────────────────────────────────────────────────────────────────────
var bar = document.createElement('div');
bar.id  = '_gs_toolbar';
bar.innerHTML =
  '<div id="_gs_tabs">' +
    '<div id="_gs_tab">' +
      '<span id="_gs_tab_title">Loading…</span>' +
    '</div>' +
    '<button id="_gs_new_tab" title="New Tab">+</button>' +
  '</div>' +
  '<div id="_gs_nav">' +
    '<button class="_gs_btn" id="_gs_back"    title="Back">&#8592;</button>' +
    '<button class="_gs_btn" id="_gs_fwd"     title="Forward">&#8594;</button>' +
    '<button class="_gs_btn" id="_gs_reload"  title="Reload">&#8635;</button>' +
    '<input  id="_gs_addr" type="text" spellcheck="false" placeholder="Search or enter address"/>' +
    '<div id="_gs_badge" title="Profile: '+profile+'"></div>' +
  '</div>';

// ── Mount ───────────────────────────────────────────────────────────────────
function mount(){
  if(document.getElementById('_gs_toolbar'))return;
  document.head.appendChild(styleEl);
  document.body.insertAdjacentElement('afterbegin', bar);

  // Populate address bar.
  var addr = document.getElementById('_gs_addr');
  addr.value = location.href;

  // Tab title.
  function updateTitle(){
    var t = document.getElementById('_gs_tab_title');
    if(t) t.textContent = document.title || location.hostname || 'New Tab';
  }
  updateTitle();
  new MutationObserver(updateTitle).observe(document.querySelector('title')||document.head,
    {childList:true,characterData:true,subtree:true});

  // ── Navigation buttons ─────────────────────────────────────────────────
  document.getElementById('_gs_back').addEventListener('click',function(){history.back();});
  document.getElementById('_gs_fwd' ).addEventListener('click',function(){history.forward();});
  document.getElementById('_gs_reload').addEventListener('click',function(){location.reload();});

  // ── Address bar ────────────────────────────────────────────────────────
  addr.addEventListener('keydown',function(e){
    if(e.key!=='Enter')return;
    e.preventDefault();
    var raw = addr.value.trim();
    if(!raw)return;
    var url = raw;
    if(url.startsWith('ghost://')){
      // let the Go side handle it
    } else if(!url.match(/^https?:\/\//i)){
      if(url.indexOf('.')!==-1 && url.indexOf(' ')===-1){
        url = 'https://'+url;
      } else {
        url = 'https://search.brave.com/search?q='+encodeURIComponent(url);
      }
    }
    location.href = url;
  });
  addr.addEventListener('focus',function(){addr.select();});

  // ── Update address on navigation ───────────────────────────────────────
  window.addEventListener('load',function(){
    addr.value = location.href;
    updateTitle();
  });
  window.addEventListener('popstate',function(){addr.value=location.href;});

  // ── New tab button ─────────────────────────────────────────────────────
  document.getElementById('_gs_new_tab').addEventListener('click',function(){
    try{ window.open('ghost://newtab','_blank'); }catch(e){}
  });
}

if(document.readyState==='loading'){
  document.addEventListener('DOMContentLoaded',mount);
}else{
  mount();
}

})();`, profileName)

	p.wv.Init(script)
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
	return fmt.Sprintf(`<h2>ghost://</h2><p>%s</p>`, url)
}

// ghostPageHTML returns fully self-contained HTML for ghost:// internal URLs.
// These pages include the browser chrome inline (no overlay injection needed).
func (p *WebViewPanel) ghostPageHTML(url string) string {
	page := strings.TrimPrefix(url, "ghost://")
	profileName := p.br.Profile().Name

	var body string
	switch page {
	case "settings":
		body = `<h2>⚙ Settings</h2><p>Settings UI coming in Phase 2.</p>`
	case "newtab":
		body = `<h2>👻 Ghost-Silicon</h2><p style="color:#666;margin-top:8px;">Type an address in the bar above and press Enter.</p>`
	case "network":
		body = `<h2>🌐 Network Monitor</h2><p>Coming in Phase 2.</p>`
	case "audit":
		body = `<h2>📋 Audit Log</h2><p>Coming in Phase 2.</p>`
	default:
		body = fmt.Sprintf(`<h2>%s</h2><p>Page not found.</p>`, page)
	}

	return fmt.Sprintf(`<!DOCTYPE html><html><head>
<meta charset="utf-8"><title>ghost://%s</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Segoe UI',system-ui,sans-serif;background:#F8F8FA;color:#191923}
#chrome{position:fixed;top:0;left:0;right:0;height:82px;background:#EDEEF2;
  border-bottom:1px solid #C8C8D2;z-index:9999;display:flex;flex-direction:column;
  box-shadow:0 1px 3px rgba(0,0,0,.1)}
#tabs{height:38px;display:flex;align-items:flex-end;padding:0 8px;gap:2px}
.tab{background:#fff;border:1px solid #C8C8D2;border-bottom:none;
  border-radius:8px 8px 0 0;padding:0 14px;height:30px;display:flex;
  align-items:center;font-size:12px;gap:8px}
#nav{height:44px;display:flex;align-items:center;padding:0 10px;gap:6px}
.btn{width:28px;height:28px;border:none;background:transparent;border-radius:50%;
  cursor:pointer;font-size:15px;color:#333}
.btn:hover{background:#D0D0DA}
#addr{flex:1;height:30px;border:1px solid #C0C0CA;border-radius:15px;
  padding:0 14px;font-size:13px;background:#fff;outline:none}
#addr:focus{border-color:#4285F4;box-shadow:0 0 0 2px rgba(66,133,244,.2)}
#badge{width:10px;height:10px;border-radius:50%;background:#32B264;flex-shrink:0}
#content{padding-top:82px;min-height:100vh;display:flex;align-items:center;
  justify-content:center;text-align:center}
#content h2{font-size:1.8rem;font-weight:700;margin-bottom:.6rem}
</style>
</head><body>
<div id="chrome">
  <div id="tabs">
    <div class="tab"><span>ghost://%s</span></div>
    <button class="btn" style="margin-left:4px;font-size:18px;align-self:center" title="New Tab">+</button>
  </div>
  <div id="nav">
    <button class="btn" onclick="history.back()" title="Back">&#8592;</button>
    <button class="btn" onclick="history.forward()" title="Forward">&#8594;</button>
    <button class="btn" onclick="location.reload()" title="Reload">&#8635;</button>
    <input id="addr" type="text" spellcheck="false"
      placeholder="Search or enter address"
      value="ghost://%s"
      onkeydown="if(event.key==='Enter'){var u=this.value.trim();if(u)location.href=(u.match(/^https?:\/\//i)||u.startsWith('ghost://'))?u:(u.includes('.')&&!u.includes(' '))?'https://'+u:'https://search.brave.com/search?q='+encodeURIComponent(u)}"
    />
    <div id="badge" title="Profile: %s"></div>
  </div>
</div>
<div id="content">%s</div>
</body></html>`,
		page, page, page, page, profileName, body)
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

// pkg/browser/webview.go
//go:build windows

// Package browser — WebView2 embed and bridge wiring.
//
// WebViewPanel embeds WebView2 as a direct child of the Walk MainWindow.
// The toolbar and tab strip are created AFTER WebView2 in Open(), so Walk
// assigns them a higher Z-order — they paint on top of the WebView2 area.
// WebView2 fills the full client area; the toolbar pixels it loses at the
// top are hidden under the opaque Walk toolbar widgets.
//
// Navigate, Eval, and Reload are called directly on the UI thread (no
// Dispatch). Walk's Synchronize() guarantees all calls originate there.
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

// WebViewPanel wraps a go-webview2 WebView using the main window as parent.
type WebViewPanel struct {
	mainWindow *walk.MainWindow // Walk main window — WebView2 is its direct child
	wv         webview2.WebView // the actual WebView2 instance

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

// NewWebViewPanel creates WebView2 as a child of mw (the Walk MainWindow).
// WebView2 sits at the bottom of mw's Z-order; the toolbar and tab strip
// created afterwards will render on top of it automatically.
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

	// Embed WebView2 as a direct child of the main window HWND.
	// Using the main window (not a CustomWidget) avoids the Z-order and
	// WndProc conflicts that arise when Walk's layout competes with WebView2
	// for control of the same child-window handle.
	hwnd := unsafe.Pointer(uintptr(mw.Handle()))
	wv := webview2.NewWithOptions(webview2.WebViewOptions{
		Window:    hwnd,
		Debug:     false,
		DataPath:  userDataDir,
		AutoFocus: true,
	})
	if wv == nil {
		return nil, fmt.Errorf("webview2: failed to create instance — " +
			"ensure the WebView2 Runtime (Edge) is installed on this machine")
	}
	p.wv = wv
	p.log.Info("webview2 initialised")

	if err := p.injectPolyfill(); err != nil {
		p.log.Warn("polyfill injection failed", "error", err.Error())
	}

	p.bindEventBridge()
	p.wv.Bind("__ghostNavigate", p.handleGhostScheme)

	// Resize WebView2 whenever the main window is resized.
	mw.SizeChanged().Attach(p.onResize)

	return p, nil
}

// Navigate loads url in the embedded WebView2.
// Must be called on the UI thread (Walk's Synchronize guarantees this).
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

// Reload reloads the current page.
func (p *WebViewPanel) Reload() {
	if p.wv == nil {
		return
	}
	p.wv.Eval(`window.location.reload()`)
}

// Stop halts an in-progress page load.
func (p *WebViewPanel) Stop() {
	if p.wv == nil {
		return
	}
	p.wv.Eval(`window.stop()`)
}

// GoBack navigates back in the WebView2 history.
func (p *WebViewPanel) GoBack() {
	if p.wv == nil {
		return
	}
	p.wv.Eval(`window.history.back()`)
}

// GoForward navigates forward in the WebView2 history.
func (p *WebViewPanel) GoForward() {
	if p.wv == nil {
		return
	}
	p.wv.Eval(`window.history.forward()`)
}

// SetZoom sets the page zoom level. 1.0 = 100 %.
func (p *WebViewPanel) SetZoom(factor float64) {
	if p.wv == nil {
		return
	}
	p.wv.Eval(fmt.Sprintf(`document.body.style.zoom=%f`, factor))
}

// Eval runs arbitrary JavaScript and returns immediately (fire-and-forget).
func (p *WebViewPanel) Eval(js string) {
	if p.wv == nil {
		return
	}
	p.wv.Eval(js)
}

// WebView returns the underlying go-webview2 handle.
func (p *WebViewPanel) WebView() webview2.WebView {
	return p.wv
}

// UpdateProfile re-injects the polyfill after a profile hot-swap.
func (p *WebViewPanel) UpdateProfile() {
	if p.wv == nil {
		return
	}
	if err := p.injectPolyfill(); err != nil {
		p.log.Warn("polyfill re-injection failed", "error", err.Error())
	}
}

// ── event bridge ─────────────────────────────────────────────────────────────

// bindEventBridge exposes Go callbacks to JavaScript via wv.Bind() and injects
// an Init script that calls them at the right page lifecycle moments.
//
// This is the only way to receive navigation events from go-webview2 — the
// library wraps the C webview layer which does not expose CoreWebView2's own
// event interfaces (NavigationStarting, NavigationCompleted, etc.).
//
// How it works:
//   - Bind() makes a Go func callable from JS as an async function.
//   - Init() (AddScriptToExecuteOnDocumentCreated) runs our listener script
//     before any page code, so events are captured from the very first frame.
//   - The script uses the Navigation API (Chrome 102+ / modern WebView2) for
//     reliable SPA-aware navigation detection, with a load-event fallback.
func (p *WebViewPanel) bindEventBridge() {
	// Bind Go callbacks — each becomes an async JS function with the same name.
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

	// Listener script — injected before every page's own scripts.
	p.wv.Init(`(function() {
  'use strict';

  // ── Navigation API (modern WebView2 / Chrome 102+) ──────────────────
  if (window.navigation) {
    window.navigation.addEventListener('navigate', function(e) {
      try { __ghostOnNavStart(e.destination.url); } catch(_) {}
    });
    window.navigation.addEventListener('navigatesuccess', function() {
      var url = window.location.href;
      try { __ghostOnNavDone(url); } catch(_) {}
      try { __ghostOnURL(url); } catch(_) {}
    });
    window.navigation.addEventListener('navigateerror', function(e) {
      try { __ghostOnError(window.location.href, e.message || 'navigation error'); } catch(_) {}
    });
  }

  // ── load / DOMContentLoaded fallback ────────────────────────────────
  // Fires for every full-page navigation even when Navigation API is absent.
  window.addEventListener('load', function() {
    var url = window.location.href;
    try { __ghostOnNavDone(url); } catch(_) {}
    try { __ghostOnURL(url); } catch(_) {}
  });

  // ── Title observer ───────────────────────────────────────────────────
  function startTitleWatch() {
    // Report current title immediately.
    if (document.title) {
      try { __ghostOnTitle(document.title); } catch(_) {}
    }
    // Watch for programmatic title changes (SPAs, etc.).
    var titleEl = document.querySelector('title');
    if (!titleEl) return;
    new MutationObserver(function() {
      try { __ghostOnTitle(document.title); } catch(_) {}
    }).observe(titleEl, { childList: true, characterData: true, subtree: true });
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', startTitleWatch);
  } else {
    startTitleWatch();
  }

  // ── History API monkey-patch (SPA pushState / replaceState) ─────────
  // Many single-page apps navigate without triggering the Navigation API.
  ['pushState', 'replaceState'].forEach(function(method) {
    var orig = history[method];
    history[method] = function() {
      orig.apply(this, arguments);
      var url = window.location.href;
      try { __ghostOnURL(url); } catch(_) {}
    };
  });
  window.addEventListener('popstate', function() {
    try { __ghostOnURL(window.location.href); } catch(_) {}
  });

})();`)
}

// ── polyfill ──────────────────────────────────────────────────────────────────

// profileConfig is the JSON shape we inject into the page.
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

// injectPolyfill calls wv.Init with a script that:
//  1. Freezes a __ghost config object built from the active profile.
//  2. Overrides navigator.*, screen.*, window.devicePixelRatio, and
//     the WebGL UNMASKED_VENDOR/RENDERER extension values.
//
// wv.Init maps to AddScriptToExecuteOnDocumentCreated, which runs the
// script before any page code — so the overrides are in place from the
// very first line of every page's JavaScript.
func (p *WebViewPanel) injectPolyfill() error {
	cfg := p.buildConfig()
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal profile config: %w", err)
	}

	polyfill := fmt.Sprintf(`(function() {
  'use strict';

  // ── Frozen profile config ───────────────────────────────────────────
  const _ghost = Object.freeze(%s);
  Object.defineProperty(window, '__ghost', { value: _ghost, writable: false });

  // ── Navigator overrides ─────────────────────────────────────────────
  function def(obj, prop, val) {
    try {
      Object.defineProperty(obj, prop, {
        get: function() { return val; },
        configurable: false,
        enumerable: true
      });
    } catch(e) {}
  }

  const nav = window.navigator;
  def(nav, 'userAgent',           _ghost.userAgent);
  def(nav, 'appVersion',          _ghost.appVersion);
  def(nav, 'vendor',              _ghost.vendor);
  def(nav, 'vendorSub',           _ghost.vendorSub);
  def(nav, 'product',             _ghost.product);
  def(nav, 'productSub',          _ghost.productSub);
  def(nav, 'languages',           Object.freeze(_ghost.languages.slice()));
  def(nav, 'language',            _ghost.language);
  def(nav, 'doNotTrack',          _ghost.doNotTrack || null);
  def(nav, 'cookieEnabled',       _ghost.cookieEnabled);
  def(nav, 'hardwareConcurrency', _ghost.hardwareConcurrency);
  def(nav, 'deviceMemory',        _ghost.deviceMemory);
  def(nav, 'maxTouchPoints',      _ghost.maxTouchPoints);
  def(nav, 'platform',            _ghost.platform);

  // ── Screen overrides ────────────────────────────────────────────────
  const scr = window.screen;
  def(scr, 'width',       _ghost.screenWidth);
  def(scr, 'height',      _ghost.screenHeight);
  def(scr, 'availWidth',  _ghost.screenAvailWidth);
  def(scr, 'availHeight', _ghost.screenAvailHeight);
  def(scr, 'colorDepth',  _ghost.colorDepth);
  def(scr, 'pixelDepth',  _ghost.pixelDepth);

  // ── Window-level overrides ──────────────────────────────────────────
  def(window, 'devicePixelRatio', _ghost.devicePixelRatio);

  // ── Canvas noise ────────────────────────────────────────────────────
  if (_ghost.canvasSeed !== 0) {
    const origToDataURL = HTMLCanvasElement.prototype.toDataURL;
    const origGetImageData = CanvasRenderingContext2D.prototype.getImageData;
    const _seed = _ghost.canvasSeed;
    function lcg(s) { return ((s * 1664525 + 1013904223) & 0xFFFFFFFF) >>> 0; }
    HTMLCanvasElement.prototype.toDataURL = function(type, quality) {
      const ctx = this.getContext('2d');
      if (ctx) {
        const id = origGetImageData.call(ctx, 0, 0, this.width, this.height);
        let s = _seed;
        for (let i = 0; i < id.data.length; i += 4) {
          s = lcg(s);
          id.data[i]   ^= (s & 0x01);
          id.data[i+1] ^= ((s >> 1) & 0x01);
          id.data[i+2] ^= ((s >> 2) & 0x01);
        }
        ctx.putImageData(id, 0, 0);
      }
      return origToDataURL.call(this, type, quality);
    };
  }

  // ── WebGL UNMASKED extension override ──────────────────────────────
  const origGetParameter = WebGLRenderingContext.prototype.getParameter;
  WebGLRenderingContext.prototype.getParameter = function(param) {
    const ext = this.getExtension('WEBGL_debug_renderer_info');
    if (ext) {
      if (param === ext.UNMASKED_VENDOR_WEBGL)   return _ghost.gpuVendor;
      if (param === ext.UNMASKED_RENDERER_WEBGL) return _ghost.gpuRenderer;
    }
    return origGetParameter.call(this, param);
  };
  const origGetParameterGL2 = WebGL2RenderingContext.prototype.getParameter;
  WebGL2RenderingContext.prototype.getParameter = function(param) {
    const ext = this.getExtension('WEBGL_debug_renderer_info');
    if (ext) {
      if (param === ext.UNMASKED_VENDOR_WEBGL)   return _ghost.gpuVendor;
      if (param === ext.UNMASKED_RENDERER_WEBGL) return _ghost.gpuRenderer;
    }
    return origGetParameterGL2.call(this, param);
  };

  // ── Timezone ─────────────────────────────────────────────────────────
  if (_ghost.timezone) {
    const origResolvedOptions = Intl.DateTimeFormat.prototype.resolvedOptions;
    Intl.DateTimeFormat.prototype.resolvedOptions = function() {
      const opts = origResolvedOptions.call(this);
      opts.timeZone = _ghost.timezone;
      return opts;
    };
  }

})();`, string(cfgJSON))

	p.wv.Init(polyfill)

	p.log.Debug("polyfill injected",
		"profile_id", p.br.Profile().ID,
		"ua", cfg.UserAgent[:min(40, len(cfg.UserAgent))],
	)
	return nil
}

// ── resize ────────────────────────────────────────────────────────────────────

// onResize is called by Walk's SizeChanged on the main window.
func (p *WebViewPanel) onResize() {
	if p.wv == nil || p.mainWindow == nil {
		return
	}
	bounds := p.mainWindow.ClientBounds()
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	p.log.Info("webview2 resize", "w", bounds.Width, "h", bounds.Height)
	p.wv.SetSize(bounds.Width, bounds.Height, webview2.HintNone)
}

// ForceResize explicitly sizes WebView2 to fill the main window.
// Called 300 ms after startup once Walk has completed its first layout pass.
func (p *WebViewPanel) ForceResize() {
	p.onResize()
}

// ── ghost:// scheme ───────────────────────────────────────────────────────────

// handleGhostScheme is a JS-bound function called when the polyfill or a
// page navigation targets a ghost:// URL.  Currently renders a stub page;
// real implementations will be added per URL in later phases.
func (p *WebViewPanel) handleGhostScheme(url string) string {
	return fmt.Sprintf(`<h2>ghost://</h2><p>%s</p>`, url)
}

// ghostPageHTML returns a minimal HTML page for ghost:// internal URLs.
func (p *WebViewPanel) ghostPageHTML(url string) string {
	page := strings.TrimPrefix(url, "ghost://")
	var body string
	switch page {
	case "settings":
		body = `<h1>⚙ Settings</h1><p>Settings UI coming in Phase 2.</p>`
	case "newtab":
		body = `<h1>👻 Ghost-Silicon</h1><p>New Tab — type an address above and press Enter.</p>`
	case "network":
		body = `<h1>🌐 Network Monitor</h1><p>Coming in Phase 2.</p>`
	case "audit":
		body = `<h1>📋 Audit Log</h1><p>Coming in Phase 2.</p>`
	default:
		body = fmt.Sprintf(`<h1>%s</h1><p>Page not found.</p>`, page)
	}
	return fmt.Sprintf(`<!DOCTYPE html><html><head>
<meta charset="utf-8">
<title>ghost://%s</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: 'Segoe UI', system-ui, sans-serif;
    background: #ffffff;
    color: #1a1a2e;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    padding: 2rem;
  }
  div { text-align: center; }
  h1 { font-size: 2rem; margin-bottom: 1rem; }
  p  { font-size: 1rem; color: #555; }
</style>
</head><body><div>%s</div></body></html>`, page, body)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

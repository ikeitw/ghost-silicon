// pkg/browser/setup.go
//go:build windows

// Package browser — first-launch identity setup wizard.
package browser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/google/uuid"
	webview2 "github.com/jchv/go-webview2"
	"github.com/lxn/win"

	"ghost-silicon/pkg/identity"
)

// SetupResult is returned by RunSetupWizard.
type SetupResult struct {
	Profile         *identity.Profile
	SearchEngineURL string
}

// ── preset tables ─────────────────────────────────────────────────────────────

type setupOSPreset struct {
	Label    string
	Platform string
	CPUCores int
	RAMMb    int
	Width    int
	Height   int
}

type setupGPUPreset struct {
	Label    string
	Vendor   string
	Renderer string
}

type setupBrowserPreset struct {
	Label         string
	UAWindows     string // UA for Windows OS presets
	UAMac         string // UA for macOS OS presets
	UALinux       string // UA for Linux OS presets
	AppVerWindows string
	AppVerMac     string
	AppVerLinux   string
	Vendor        string
	ProductSub    string
	// DefaultSearch is the URL prefix used when text is typed in the address bar.
	DefaultSearch string
}

var wizardOSPresets = []setupOSPreset{
	{"Windows 11 Desktop  (1920×1080, 8-core, 16 GB)", "Win32", 8, 16384, 1920, 1080},
	{"Windows 11 Laptop   (1366×768,  4-core,  8 GB)", "Win32", 4, 8192, 1366, 768},
	{"Windows 11 Gaming   (2560×1440, 16-core, 32 GB)", "Win32", 16, 32768, 2560, 1440},
	{"Windows 10 Desktop  (1920×1080,  8-core, 16 GB)", "Win32", 8, 16384, 1920, 1080},
	{"macOS (spoofed)     (1920×1080,  8-core, 16 GB)", "MacIntel", 8, 16384, 1920, 1080},
	{"Linux (spoofed)     (1920×1080,  4-core,  8 GB)", "Linux x86_64", 4, 8192, 1920, 1080},
}

var wizardGPUPresets = []setupGPUPreset{
	{"NVIDIA GeForce RTX 4070", "Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce RTX 4070 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"NVIDIA GeForce RTX 3060", "Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"NVIDIA GeForce GTX 1660 Ti", "Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce GTX 1660 Ti Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"NVIDIA GeForce RTX 2060", "Google Inc. (NVIDIA)", "ANGLE (NVIDIA, NVIDIA GeForce RTX 2060 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"AMD Radeon RX 7600", "Google Inc. (AMD)", "ANGLE (AMD, AMD Radeon RX 7600 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"AMD Radeon RX 6700 XT", "Google Inc. (AMD)", "ANGLE (AMD, AMD Radeon RX 6700 XT Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"AMD Radeon RX 5700 XT", "Google Inc. (AMD)", "ANGLE (AMD, AMD Radeon RX 5700 XT Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Intel UHD Graphics 630", "Google Inc. (Intel)", "ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Intel UHD Graphics 620", "Google Inc. (Intel)", "ANGLE (Intel, Intel(R) UHD Graphics 620 Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"Intel Iris Xe Graphics", "Google Inc. (Intel)", "ANGLE (Intel, Intel(R) Iris(R) Xe Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)"},
	{"AMD Radeon 680M (laptop)", "Google Inc. (AMD)", "ANGLE (AMD, AMD Radeon(TM) 680M Direct3D11 vs_5_0 ps_5_0, D3D11)"},
}

var wizardBrowserPresets = []setupBrowserPreset{
	{
		Label:         "Google Chrome 125",
		UAWindows:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		UAMac:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		UALinux:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		AppVerWindows: "5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		AppVerMac:     "5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		AppVerLinux:   "5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		Vendor:        "Google Inc.", ProductSub: "20030107",
		DefaultSearch: "https://www.google.com/search?q=",
	},
	{
		Label:         "Microsoft Edge 125",
		UAWindows:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0",
		UAMac:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0",
		UALinux:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0",
		AppVerWindows: "5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0",
		AppVerMac:     "5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0",
		AppVerLinux:   "5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0",
		Vendor:        "Google Inc.", ProductSub: "20030107",
		DefaultSearch: "https://www.bing.com/search?q=",
	},
	{
		Label:         "Brave 1.66",
		UAWindows:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		UAMac:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		UALinux:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		AppVerWindows: "5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		AppVerMac:     "5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		AppVerLinux:   "5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		Vendor:        "", ProductSub: "20030107",
		DefaultSearch: "https://duckduckgo.com/?q=",
	},
	{
		Label:         "Mozilla Firefox 126",
		UAWindows:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:126.0) Gecko/20100101 Firefox/126.0",
		UAMac:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:126.0) Gecko/20100101 Firefox/126.0",
		UALinux:       "Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
		AppVerWindows: "5.0 (Windows NT 10.0; Win64; x64; rv:126.0) Gecko/20100101 Firefox/126.0",
		AppVerMac:     "5.0 (Macintosh; Intel Mac OS X 10.15; rv:126.0) Gecko/20100101 Firefox/126.0",
		AppVerLinux:   "5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
		Vendor:        "", ProductSub: "",
		DefaultSearch: "https://www.google.com/search?q=",
	},
	{
		Label:         "DuckDuckGo Browser",
		UAWindows:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		UAMac:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		UALinux:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		AppVerWindows: "5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		AppVerMac:     "5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		AppVerLinux:   "5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		Vendor:        "", ProductSub: "20030107",
		DefaultSearch: "https://duckduckgo.com/?q=",
	},
	{
		Label:         "Yandex Browser 24.4",
		UAWindows:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 YaBrowser/24.4.2.985 Yowser/2.5 Safari/537.36",
		UAMac:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 YaBrowser/24.4.2.985 Yowser/2.5 Safari/537.36",
		UALinux:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 YaBrowser/24.4.2.985 Yowser/2.5 Safari/537.36",
		AppVerWindows: "5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 YaBrowser/24.4.2.985 Yowser/2.5 Safari/537.36",
		AppVerMac:     "5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 YaBrowser/24.4.2.985 Yowser/2.5 Safari/537.36",
		AppVerLinux:   "5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.6367.82 YaBrowser/24.4.2.985 Yowser/2.5 Safari/537.36",
		Vendor:        "Google Inc.", ProductSub: "20030107",
		DefaultSearch: "https://yandex.ru/search/?text=",
	},
	{
		Label:         "Opera 109",
		UAWindows:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 OPR/109.0.0.0",
		UAMac:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 OPR/109.0.0.0",
		UALinux:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 OPR/109.0.0.0",
		AppVerWindows: "5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 OPR/109.0.0.0",
		AppVerMac:     "5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 OPR/109.0.0.0",
		AppVerLinux:   "5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 OPR/109.0.0.0",
		Vendor:        "Google Inc.", ProductSub: "20030107",
		DefaultSearch: "https://www.google.com/search?q=",
	},
	{
		Label:         "Safari 17 (macOS spoof)",
		UAWindows:     "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		UAMac:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		UALinux:       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		AppVerWindows: "5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		AppVerMac:     "5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		AppVerLinux:   "5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		Vendor:        "Apple Computer, Inc.", ProductSub: "20030107",
		DefaultSearch: "https://www.google.com/search?q=",
	},
}

// ── wizard window ─────────────────────────────────────────────────────────────

type wizardChoice struct {
	OSPreset        int    `json:"osPreset"`
	GPUPreset       int    `json:"gpuPreset"`
	BrowserPreset   int    `json:"browserPreset"`
	Language        string `json:"language"`
	Timezone        string `json:"timezone"`
	SearchEngineURL string `json:"searchEngineURL"`
}

type setupWin struct {
	wv     webview2.WebView
	hwnd   win.HWND
	choice *wizardChoice
	cbPtr  uintptr
}

// RunSetupWizard opens the identity-setup wizard and blocks until the user
// clicks Launch or closes the window. Returns nil if the user cancelled.
func RunSetupWizard(userDataDir string) (*SetupResult, error) {
	sw := &setupWin{}
	return sw.run(userDataDir)
}

func (sw *setupWin) run(userDataDir string) (*SetupResult, error) {
	wv := webview2.NewWithOptions(webview2.WebViewOptions{
		Window:    nil,
		Debug:     false,
		DataPath:  userDataDir,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "Ghost-Silicon — Setup",
			Width:  540,
			Height: 680,
			Center: true,
		},
	})
	if wv == nil {
		return nil, fmt.Errorf("setup wizard: failed to create WebView2 instance")
	}
	sw.wv = wv
	sw.hwnd = win.HWND(uintptr(wv.Window()))

	// ── Frameless chrome ──────────────────────────────────────────────────
	curStyle := win.GetWindowLong(sw.hwnd, win.GWL_STYLE)
	win.SetWindowLong(sw.hwnd, win.GWL_STYLE, curStyle&^win.WS_CAPTION|win.WS_CLIPCHILDREN)
	win.SetWindowPos(sw.hwnd, 0, 0, 0, 0, 0,
		win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)

	dwmapi := syscall.NewLazyDLL("dwmapi.dll")
	type dwmMargins struct{ L, R, T, B int32 }
	m := dwmMargins{0, 0, 1, 0}
	dwmapi.NewProc("DwmExtendFrameIntoClientArea").Call(
		uintptr(sw.hwnd), uintptr(unsafe.Pointer(&m)))
	cornerPref := uint32(2)
	dwmapi.NewProc("DwmSetWindowAttribute").Call(
		uintptr(sw.hwnd), 33, uintptr(unsafe.Pointer(&cornerPref)), 4)

	sw.subclassWndProc()

	// ── Window-control bindings ───────────────────────────────────────────
	wv.Bind("__setupMinimize", func() {
		win.PostMessage(sw.hwnd, win.WM_SYSCOMMAND, win.SC_MINIMIZE, 0)
	})
	wv.Bind("__setupClose", func() {
		win.PostMessage(sw.hwnd, win.WM_CLOSE, 0, 0)
	})
	wv.Bind("__setupDrag", func() {
		var pt win.POINT
		win.GetCursorPos(&pt)
		lp := uintptr(pt.Y)<<16 | uintptr(uint16(pt.X))
		win.ReleaseCapture()
		win.PostMessage(sw.hwnd, win.WM_NCLBUTTONDOWN, win.HTCAPTION, lp)
	})

	// ── Launch binding ────────────────────────────────────────────────────
	wv.Bind("__setupDone", func(jsonStr string) {
		var ch wizardChoice
		if err := json.Unmarshal([]byte(jsonStr), &ch); err == nil {
			sw.choice = &ch
		}
		win.PostMessage(sw.hwnd, win.WM_CLOSE, 0, 0)
	})

	html := setupPageHTML()
	wv.Navigate("data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html)))
	wv.Run()
	wv.Destroy()

	if sw.choice == nil {
		return nil, nil
	}
	return buildSetupResult(sw.choice), nil
}

func (sw *setupWin) subclassWndProc() {
	const (
		gwlpWndProc  = ^uintptr(3)
		wmNcCalcSize = uintptr(0x0083)
		wmNcHitTest  = uintptr(0x0084)
		htClient     = uintptr(1)
	)
	user32 := syscall.NewLazyDLL("user32.dll")
	getPtr := user32.NewProc("GetWindowLongPtrW")
	setPtr := user32.NewProc("SetWindowLongPtrW")
	callProc := user32.NewProc("CallWindowProcW")

	orig, _, _ := getPtr.Call(uintptr(sw.hwnd), gwlpWndProc)
	cb := syscall.NewCallback(func(h, msg, wp, lp uintptr) uintptr {
		switch msg {
		case wmNcCalcSize:
			if wp != 0 {
				return 0
			}
		case wmNcHitTest:
			return htClient
		}
		r, _, _ := callProc.Call(orig, h, msg, wp, lp)
		return r
	})
	sw.cbPtr = cb
	setPtr.Call(uintptr(sw.hwnd), gwlpWndProc, cb)
}

// ── result builder ────────────────────────────────────────────────────────────

func buildSetupResult(ch *wizardChoice) *SetupResult {
	now := time.Now().UTC()

	osIdx := clampIdx(ch.OSPreset, len(wizardOSPresets))
	os := wizardOSPresets[osIdx]

	brIdx := clampIdx(ch.BrowserPreset, len(wizardBrowserPresets))
	br := wizardBrowserPresets[brIdx]

	// Pick UA and AppVersion matching the chosen OS platform.
	ua, av := uaForPlatform(br, os.Platform)

	lang := ch.Language
	if lang == "" {
		lang = "en-US"
	}
	tz := ch.Timezone
	if tz == "" {
		tz = "America/New_York"
	}
	searchURL := ch.SearchEngineURL
	if searchURL == "" {
		searchURL = br.DefaultSearch
	}

	gpuIdx := clampIdx(ch.GPUPreset, len(wizardGPUPresets))
	gpu := wizardGPUPresets[gpuIdx]

	p := &identity.Profile{
		ID:            uuid.New().String(),
		Name:          "custom-session",
		SchemaVersion: identity.CurrentSchemaVersion,
		CreatedAt:     now,
		UpdatedAt:     now,

		Hardware: identity.HardwareProfile{
			Platform:    os.Platform,
			CPUCores:    os.CPUCores,
			RAMMb:       os.RAMMb,
			GPUVendor:   gpu.Vendor,
			GPURenderer: gpu.Renderer,
		},
		Browser: identity.BrowserProfile{
			UserAgent:     ua,
			AppVersion:    av,
			Vendor:        br.Vendor,
			Product:       "Gecko",
			ProductSub:    br.ProductSub,
			Languages:     languageList(lang),
			CookieEnabled: true,
		},
		Screen: identity.ScreenProfile{
			Width:            os.Width,
			Height:           os.Height,
			AvailWidth:       os.Width,
			AvailHeight:      os.Height - 40,
			ColorDepth:       24,
			PixelDepth:       24,
			DevicePixelRatio: 1.0,
			Orientation:      identity.OrientationLandscapePrimary,
		},
		Network: identity.NetworkProfile{Timezone: tz},
		Storage: identity.StoragePolicy{
			EnableCookies:        true,
			EnableLocalStorage:   true,
			EnableSessionStorage: true,
			EnableIndexedDB:      true,
			EnableCacheStorage:   true,
			EnableServiceWorker:  true,
		},
		Permissions: identity.PermissionsPolicy{
			Geolocation:    identity.PermissionDeny,
			Notifications:  identity.PermissionDeny,
			Microphone:     identity.PermissionDeny,
			Camera:         identity.PermissionDeny,
			Clipboard:      identity.PermissionPrompt,
			FullScreen:     identity.PermissionPrompt,
			PaymentHandler: identity.PermissionDeny,
			MIDI:           identity.PermissionDeny,
			USB:            identity.PermissionDeny,
			Bluetooth:      identity.PermissionDeny,
		},
	}

	return &SetupResult{Profile: p, SearchEngineURL: searchURL}
}

// uaForPlatform picks the OS-appropriate UA and AppVersion from a browser preset.
func uaForPlatform(br setupBrowserPreset, platform string) (ua, av string) {
	switch platform {
	case "MacIntel":
		return br.UAMac, br.AppVerMac
	case "Linux x86_64":
		return br.UALinux, br.AppVerLinux
	default:
		return br.UAWindows, br.AppVerWindows
	}
}

func clampIdx(v, n int) int {
	if v < 0 {
		return 0
	}
	if v >= n {
		return n - 1
	}
	return v
}

func languageList(primary string) []string {
	base := strings.SplitN(primary, "-", 2)[0]
	if base == primary {
		return []string{primary}
	}
	return []string{primary, base}
}

// ── HTML ──────────────────────────────────────────────────────────────────────

func setupPageHTML() string {
	var osOpts, gpuOpts, brOpts strings.Builder
	for i, p := range wizardOSPresets {
		fmt.Fprintf(&osOpts, `<option value="%d">%s</option>`, i, p.Label)
	}
	for i, p := range wizardGPUPresets {
		fmt.Fprintf(&gpuOpts, `<option value="%d">%s</option>`, i, p.Label)
	}
	for i, p := range wizardBrowserPresets {
		fmt.Fprintf(&brOpts, `<option value="%d">%s</option>`, i, p.Label)
	}

	// Build per-browser default search engine map for JS auto-select
	var seMap strings.Builder
	seMap.WriteString("{")
	for i, p := range wizardBrowserPresets {
		if i > 0 {
			seMap.WriteString(",")
		}
		fmt.Fprintf(&seMap, `"%d":"%s"`, i, p.DefaultSearch)
	}
	seMap.WriteString("}")

	page := strings.ReplaceAll(setupPageTemplate, "{{OS_OPTIONS}}", osOpts.String())
	page = strings.ReplaceAll(page, "{{GPU_OPTIONS}}", gpuOpts.String())
	page = strings.ReplaceAll(page, "{{BROWSER_OPTIONS}}", brOpts.String())
	page = strings.ReplaceAll(page, "{{SE_MAP}}", seMap.String())
	return page
}

const setupPageTemplate = `<!DOCTYPE html>
<html><head>
<meta charset="utf-8">
<title>Ghost-Silicon Setup</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
html,body{height:100%;overflow:hidden}
body{
  font-family:'Segoe UI',system-ui,sans-serif;
  background:linear-gradient(160deg,#1a0a0a 0%,#0d0d30 50%,#050a28 100%);
  color:#E8E8F4;
  display:flex;flex-direction:column;
}
#hdr{
  height:52px;flex-shrink:0;
  background:linear-gradient(90deg,#3D1A0A 0%,#2A1560 40%,#0A1A6B 70%,#050E40 100%);
  border-bottom:1px solid rgba(255,255,255,.1);
  display:flex;align-items:center;padding:0 12px 0 18px;gap:8px;
  user-select:none;
}
#hdr-logo{
  font-size:15px;font-weight:700;
  background:linear-gradient(90deg,#F4A460,#A080FF);
  -webkit-background-clip:text;-webkit-text-fill-color:transparent;
  flex:1;
}
#hdr-sub{font-size:11px;color:rgba(255,255,255,.35)}
.wm-btn{
  width:32px;height:32px;border:none;background:transparent;
  color:rgba(255,255,255,.55);font-size:13px;cursor:pointer;
  border-radius:5px;display:flex;align-items:center;justify-content:center;
}
.wm-btn:hover{background:rgba(255,255,255,.15);color:#fff}
#btn-cls:hover{background:#E81123!important;color:#fff}
#content{
  flex:1;overflow-y:auto;padding:20px 26px 8px;
  scrollbar-width:thin;
  scrollbar-color:rgba(160,128,255,.25) transparent;
}
#content::-webkit-scrollbar{width:4px}
#content::-webkit-scrollbar-thumb{background:rgba(160,128,255,.25);border-radius:2px}
.hero{margin-bottom:20px}
.hero h1{
  font-size:1.4rem;font-weight:700;
  background:linear-gradient(90deg,#F4A460,#A080FF);
  -webkit-background-clip:text;-webkit-text-fill-color:transparent;
  margin-bottom:3px;
}
.hero p{font-size:12px;color:rgba(255,255,255,.38)}
.sec{margin-bottom:18px}
.sec-title{
  font-size:10px;font-weight:700;color:#A080FF;
  text-transform:uppercase;letter-spacing:.1em;
  margin-bottom:11px;
  display:flex;align-items:center;gap:8px;
}
.sec-title::after{content:'';flex:1;height:1px;background:rgba(160,128,255,.18)}
.field{margin-bottom:10px}
.field label{display:block;font-size:11px;color:rgba(255,255,255,.42);margin-bottom:4px;letter-spacing:.02em}
.field select{
  width:100%;
  padding:8px 30px 8px 11px;
  background:rgba(255,255,255,.07);
  border:1px solid rgba(255,255,255,.11);
  border-radius:7px;
  color:#E8E8F4;
  font-size:12.5px;
  font-family:inherit;
  outline:none;cursor:pointer;
  appearance:none;-webkit-appearance:none;
  background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6' viewBox='0 0 10 6'%3E%3Cpath d='M1 1l4 4 4-4' stroke='rgba(255,255,255,.35)' stroke-width='1.5' fill='none' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E");
  background-repeat:no-repeat;
  background-position:right 10px center;
  transition:border-color .15s,background-color .15s;
}
.field select:focus{border-color:rgba(160,128,255,.55);background-color:rgba(255,255,255,.1)}
.field select option{background:#12103a;color:#E8E8F4}
.row{display:grid;grid-template-columns:1fr 1fr;gap:11px}
#footer{
  flex-shrink:0;
  padding:13px 26px;
  border-top:1px solid rgba(255,255,255,.07);
  background:rgba(5,8,35,.85);
  display:flex;align-items:center;justify-content:space-between;
}
#footer-hint{font-size:11px;color:rgba(255,255,255,.22)}
#launch{
  padding:10px 26px;
  background:linear-gradient(90deg,#6A40FF,#A080FF);
  border:none;color:#fff;font-size:13px;font-weight:600;
  border-radius:8px;cursor:pointer;letter-spacing:.02em;
  transition:opacity .15s,transform .1s;
}
#launch:hover{opacity:.85}
#launch:active{opacity:.7;transform:scale(.98)}
</style>
</head>
<body>

<div id="hdr">
  <span id="hdr-logo">Ghost-Silicon</span>
  <span id="hdr-sub">Session Setup</span>
  <button class="wm-btn" title="Minimise" onclick="__setupMinimize()">&#8212;</button>
  <button class="wm-btn" id="btn-cls" title="Cancel" onclick="__setupClose()">&#10005;</button>
</div>

<div id="content">
  <div class="hero">
    <h1>Identity Setup</h1>
    <p>Choose the fingerprint this session will present to websites. Your real hardware stays private.</p>
  </div>

  <div class="sec">
    <div class="sec-title">System Identity</div>
    <div class="field">
      <label>Operating System &amp; Screen</label>
      <select id="os">{{OS_OPTIONS}}</select>
    </div>
    <div class="field">
      <label>Graphics Card (GPU)</label>
      <select id="gpu">{{GPU_OPTIONS}}</select>
    </div>
    <div class="row">
      <div class="field">
        <label>Language</label>
        <select id="lang">
          <option value="en-US">English (US)</option>
          <option value="en-GB">English (UK)</option>
          <option value="de-DE">German</option>
          <option value="fr-FR">French</option>
          <option value="es-ES">Spanish</option>
          <option value="ru-RU">Russian</option>
          <option value="pt-BR">Portuguese (BR)</option>
          <option value="zh-CN">Chinese (Simplified)</option>
          <option value="ja-JP">Japanese</option>
          <option value="ko-KR">Korean</option>
          <option value="it-IT">Italian</option>
          <option value="nl-NL">Dutch</option>
        </select>
      </div>
      <div class="field">
        <label>Timezone</label>
        <select id="tz">
          <option value="America/New_York">New York (UTC-5)</option>
          <option value="America/Chicago">Chicago (UTC-6)</option>
          <option value="America/Denver">Denver (UTC-7)</option>
          <option value="America/Los_Angeles">Los Angeles (UTC-8)</option>
          <option value="Europe/London">London (UTC+0)</option>
          <option value="Europe/Paris">Paris (UTC+1)</option>
          <option value="Europe/Berlin">Berlin (UTC+1)</option>
          <option value="Europe/Moscow">Moscow (UTC+3)</option>
          <option value="Asia/Dubai">Dubai (UTC+4)</option>
          <option value="Asia/Shanghai">Shanghai (UTC+8)</option>
          <option value="Asia/Tokyo">Tokyo (UTC+9)</option>
          <option value="Asia/Seoul">Seoul (UTC+9)</option>
          <option value="Australia/Sydney">Sydney (UTC+10)</option>
          <option value="Pacific/Auckland">Auckland (UTC+12)</option>
        </select>
      </div>
    </div>
  </div>

  <div class="sec">
    <div class="sec-title">Browser &amp; Search</div>
    <div class="field">
      <label>Browser Identity</label>
      <select id="browser">{{BROWSER_OPTIONS}}</select>
    </div>
    <div class="field">
      <label>Default Search Engine</label>
      <select id="search">
        <option value="https://www.google.com/search?q=">Google</option>
        <option value="https://duckduckgo.com/?q=">DuckDuckGo</option>
        <option value="https://www.bing.com/search?q=">Bing</option>
        <option value="https://yandex.ru/search/?text=">Yandex</option>
        <option value="https://search.brave.com/search?q=">Brave Search</option>
        <option value="https://www.ecosia.org/search?q=">Ecosia</option>
        <option value="https://search.yahoo.com/search?p=">Yahoo</option>
      </select>
    </div>
  </div>
</div>

<div id="footer">
  <span id="footer-hint">&#128274; Your real hardware stays private.</span>
  <button id="launch" onclick="launch()">Launch Browser &#8594;</button>
</div>

<script>
(function(){
  var seMap={{SE_MAP}};
  // Drag on header
  document.getElementById('hdr').addEventListener('mousedown',function(e){
    if(e.button===0&&!e.target.closest('button')){try{__setupDrag();}catch(_){}}
  });
  // Auto-select search engine when browser changes
  document.getElementById('browser').addEventListener('change',function(){
    var url=seMap[this.value];
    if(url){document.getElementById('search').value=url;}
  });
  // Enter key launches
  document.addEventListener('keydown',function(e){
    if(e.key==='Enter'&&!e.target.matches('select')){e.preventDefault();launch();}
  });
})();
function launch(){
  var payload=JSON.stringify({
    osPreset:        parseInt(document.getElementById('os').value),
    gpuPreset:       parseInt(document.getElementById('gpu').value),
    browserPreset:   parseInt(document.getElementById('browser').value),
    language:        document.getElementById('lang').value,
    timezone:        document.getElementById('tz').value,
    searchEngineURL: document.getElementById('search').value
  });
  try{__setupDone(payload);}catch(e){console.error(e);}
}
</script>
</body></html>`

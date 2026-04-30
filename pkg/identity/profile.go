// Package identity provides the Profile model and all identity management
// primitives for Ghost-Silicon. A Profile is the single source of truth for
// the controlled values that the supervisor exposes to a renderer session —
// hardware concurrency, GPU vendor, RAM, canvas noise seeds, user-agent, and
// so on. Nothing inside the renderer reads host hardware directly; every value
// flows through the profile.
package identity

import "time"

// CurrentSchemaVersion is the profile JSON schema version written by this
// build. Older profiles are migrated up by the schema package on load.
const CurrentSchemaVersion = 1

// PermissionState is the policy for a single browser permission.
type PermissionState string

const (
	PermissionDeny   PermissionState = "deny"
	PermissionPrompt PermissionState = "prompt"
	PermissionGrant  PermissionState = "grant"
)

// Orientation describes screen orientation.
type Orientation string

const (
	OrientationLandscapePrimary   Orientation = "landscape-primary"
	OrientationLandscapeSecondary Orientation = "landscape-secondary"
	OrientationPortraitPrimary    Orientation = "portrait-primary"
	OrientationPortraitSecondary  Orientation = "portrait-secondary"
)

// Profile is the complete identity specification for one browser session.
// Every field is intentional and controlled — nothing defaults to the host
// machine's real value.
type Profile struct {
	// Identity metadata
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	SchemaVersion int       `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Tags          []string  `json:"tags,omitempty"`

	// Controlled subsystems
	Hardware    HardwareProfile   `json:"hardware"`
	Browser     BrowserProfile    `json:"browser"`
	Screen      ScreenProfile     `json:"screen"`
	Network     NetworkProfile    `json:"network"`
	Noise       NoiseProfile      `json:"noise"`
	Storage     StoragePolicy     `json:"storage"`
	Permissions PermissionsPolicy `json:"permissions"`
}

// HardwareProfile controls what hardware values the renderer receives.
// These map directly to the JavaScript APIs they protect:
//   - CPUCores     → navigator.hardwareConcurrency
//   - GPUVendor    → UNMASKED_VENDOR_WEBGL extension
//   - GPURenderer  → UNMASKED_RENDERER_WEBGL extension
//   - RAMMb        → navigator.deviceMemory  (reported in GB; stored in MB here)
//   - Platform     → navigator.platform
//   - MaxTouchPoints → navigator.maxTouchPoints
type HardwareProfile struct {
	CPUCores       int    `json:"cpu_cores"`
	GPUVendor      string `json:"gpu_vendor"`
	GPURenderer    string `json:"gpu_renderer"`
	RAMMb          int    `json:"ram_mb"`
	Platform       string `json:"platform"`
	MaxTouchPoints int    `json:"max_touch_points"`
}

// DeviceMemoryGB returns RAMMb rounded to the nearest value that
// navigator.deviceMemory would report (0.25, 0.5, 1, 2, 4, 8).
func (h HardwareProfile) DeviceMemoryGB() float64 {
	gb := float64(h.RAMMb) / 1024.0
	buckets := []float64{0.25, 0.5, 1, 2, 4, 8}
	best := buckets[0]
	for _, b := range buckets {
		if b <= gb {
			best = b
		}
	}
	return best
}

// BrowserProfile controls navigator and window identity strings.
type BrowserProfile struct {
	UserAgent     string   `json:"user_agent"`
	AppVersion    string   `json:"app_version"` // everything after "Mozilla/"
	Vendor        string   `json:"vendor"`      // "Google Inc." for Chrome
	VendorSub     string   `json:"vendor_sub,omitempty"`
	Product       string   `json:"product"` // always "Gecko"
	ProductSub    string   `json:"product_sub,omitempty"`
	Languages     []string `json:"languages"`    // ["en-US","en"]
	DoNotTrack    string   `json:"do_not_track"` // "1", "0", or "" (unset)
	CookieEnabled bool     `json:"cookie_enabled"`
}

// PrimaryLanguage returns the first entry in Languages or "en-US" as fallback.
func (b BrowserProfile) PrimaryLanguage() string {
	if len(b.Languages) > 0 {
		return b.Languages[0]
	}
	return "en-US"
}

// ScreenProfile controls the values returned by window.screen.*.
type ScreenProfile struct {
	Width            int         `json:"width"`
	Height           int         `json:"height"`
	AvailWidth       int         `json:"avail_width"`
	AvailHeight      int         `json:"avail_height"`
	ColorDepth       int         `json:"color_depth"`
	PixelDepth       int         `json:"pixel_depth"`
	DevicePixelRatio float64     `json:"device_pixel_ratio"`
	Orientation      Orientation `json:"orientation"`
}

// NetworkProfile controls timezone, locale, and routing configuration.
type NetworkProfile struct {
	// Timezone is an IANA timezone name, e.g. "Europe/Amsterdam".
	// It is used both for JavaScript Date APIs and for the Accept-Language header.
	Timezone string `json:"timezone"`

	// ProxyURL is an optional upstream proxy for the renderer's traffic.
	// Supported schemes: http, https, socks5.
	ProxyURL string `json:"proxy_url,omitempty"`

	// DNSServers overrides the system resolver for renderer sessions.
	// If empty, the system DNS is used.
	DNSServers []string `json:"dns_servers,omitempty"`
}

// NoiseProfile holds per-session seeds for the noise injected into
// fingerprinting APIs. Each seed produces deterministic-but-different output
// across profiles, preventing cross-profile correlation.
type NoiseProfile struct {
	// CanvasSeed seeds pixel-level noise in Canvas2D readback operations.
	CanvasSeed int64 `json:"canvas_seed"`

	// AudioSeed seeds noise injected into AudioContext getChannelData output.
	AudioSeed int64 `json:"audio_seed"`

	// WebGLSeed seeds noise in WebGL parameter readback.
	WebGLSeed int64 `json:"webgl_seed"`

	// FontSeed seeds the font metrics noise injected via measureText.
	FontSeed int64 `json:"font_seed"`
}

// StoragePolicy controls which storage APIs are available to the renderer.
type StoragePolicy struct {
	EnableCookies        bool `json:"enable_cookies"`
	EnableLocalStorage   bool `json:"enable_local_storage"`
	EnableSessionStorage bool `json:"enable_session_storage"`
	EnableIndexedDB      bool `json:"enable_indexed_db"`
	EnableCacheStorage   bool `json:"enable_cache_storage"`
	EnableServiceWorker  bool `json:"enable_service_worker"`

	// MaxCookieJarMB caps the cookie jar size (0 = no limit).
	MaxCookieJarMB int `json:"max_cookie_jar_mb,omitempty"`
	// MaxStorageMB caps total localStorage + IndexedDB size (0 = no limit).
	MaxStorageMB int `json:"max_storage_mb,omitempty"`
}

// PermissionsPolicy declares the default answer for each permission type.
// "deny"   — blocked silently, no prompt shown.
// "prompt" — the renderer's native permission UI is shown.
// "grant"  — automatically granted (use with care).
type PermissionsPolicy struct {
	Geolocation    PermissionState `json:"geolocation"`
	Notifications  PermissionState `json:"notifications"`
	Microphone     PermissionState `json:"microphone"`
	Camera         PermissionState `json:"camera"`
	Clipboard      PermissionState `json:"clipboard"`
	FullScreen     PermissionState `json:"full_screen"`
	PaymentHandler PermissionState `json:"payment_handler"`
	MIDI           PermissionState `json:"midi"`
	USB            PermissionState `json:"usb"`
	Bluetooth      PermissionState `json:"bluetooth"`
}

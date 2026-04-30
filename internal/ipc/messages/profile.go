// Package messages defines the request and response structs for every
// JSON-RPC method on the ghost-silicon IPC bridge.
//
// Method naming convention: <subsystem>.<action>
//
//	hardware.getCPUCores
//	hardware.getRAM
//	hardware.getGPUVendor
//	hardware.getGPURenderer
//	navigator.getUserAgent
//	navigator.getLanguages
//	navigator.getPlatform
//	screen.getProfile
//	noise.getCanvasSeed
//	noise.getAudioSeed
//	noise.getWebGLSeed
//	storage.getPolicy
//	permissions.getState
//	session.getID
//	session.getProfileID
package messages

// ── hardware ────────────────────────────────────────────────────────────────

// HardwareCPUCoresResponse is returned by hardware.getCPUCores.
type HardwareCPUCoresResponse struct {
	Cores int `json:"cores"`
}

// HardwareRAMResponse is returned by hardware.getRAM.
type HardwareRAMResponse struct {
	// DeviceMemoryGB is the value to report via navigator.deviceMemory.
	// It is already bucketed to a valid spec value (0.25, 0.5, 1, 2, 4, 8).
	DeviceMemoryGB float64 `json:"device_memory_gb"`
}

// HardwareGPUResponse is returned by hardware.getGPUVendor and hardware.getGPURenderer.
type HardwareGPUResponse struct {
	Vendor   string `json:"vendor"`
	Renderer string `json:"renderer"`
}

// ── navigator ───────────────────────────────────────────────────────────────

// NavigatorProfileResponse is returned by navigator.getProfile.
// It bundles all navigator.* values into one round-trip.
type NavigatorProfileResponse struct {
	UserAgent      string   `json:"user_agent"`
	AppVersion     string   `json:"app_version"`
	Vendor         string   `json:"vendor"`
	VendorSub      string   `json:"vendor_sub"`
	Product        string   `json:"product"`
	ProductSub     string   `json:"product_sub"`
	Languages      []string `json:"languages"`
	DoNotTrack     string   `json:"do_not_track"`
	CookieEnabled  bool     `json:"cookie_enabled"`
	MaxTouchPoints int      `json:"max_touch_points"`
	Platform       string   `json:"platform"`
}

// ── screen ──────────────────────────────────────────────────────────────────

// ScreenProfileResponse is returned by screen.getProfile.
type ScreenProfileResponse struct {
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	AvailWidth       int     `json:"avail_width"`
	AvailHeight      int     `json:"avail_height"`
	ColorDepth       int     `json:"color_depth"`
	PixelDepth       int     `json:"pixel_depth"`
	DevicePixelRatio float64 `json:"device_pixel_ratio"`
	Orientation      string  `json:"orientation"`
}

// ── noise ───────────────────────────────────────────────────────────────────

// NoiseSeedResponse is returned by noise.getCanvasSeed, noise.getAudioSeed,
// noise.getWebGLSeed, and noise.getFontSeed.
type NoiseSeedResponse struct {
	Seed int64 `json:"seed"`
}

// ── storage ─────────────────────────────────────────────────────────────────

// StoragePolicyResponse is returned by storage.getPolicy.
type StoragePolicyResponse struct {
	EnableCookies        bool `json:"enable_cookies"`
	EnableLocalStorage   bool `json:"enable_local_storage"`
	EnableSessionStorage bool `json:"enable_session_storage"`
	EnableIndexedDB      bool `json:"enable_indexed_db"`
	EnableCacheStorage   bool `json:"enable_cache_storage"`
	EnableServiceWorker  bool `json:"enable_service_worker"`
	MaxCookieJarMB       int  `json:"max_cookie_jar_mb"`
	MaxStorageMB         int  `json:"max_storage_mb"`
}

// ── permissions ─────────────────────────────────────────────────────────────

// PermissionQueryRequest is the request body for permissions.getState.
type PermissionQueryRequest struct {
	// Permission is the name of the browser permission to query,
	// e.g. "geolocation", "notifications", "camera".
	Permission string `json:"permission"`
}

// PermissionStateResponse is returned by permissions.getState.
type PermissionStateResponse struct {
	// State is one of "deny", "prompt", or "grant".
	State string `json:"state"`
}

// ── session ──────────────────────────────────────────────────────────────────

// SessionInfoResponse is returned by session.getInfo.
type SessionInfoResponse struct {
	SessionID string `json:"session_id"`
	ProfileID string `json:"profile_id"`
}

// ── network ──────────────────────────────────────────────────────────────────

// NetworkProfileResponse is returned by network.getProfile.
type NetworkProfileResponse struct {
	Timezone   string   `json:"timezone"`
	ProxyURL   string   `json:"proxy_url,omitempty"`
	DNSServers []string `json:"dns_servers,omitempty"`
}

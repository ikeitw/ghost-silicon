package identity

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ValidationError represents a single field-level validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("profile validation: field %q: %s", e.Field, e.Message)
}

// ValidationResult collects all errors found during validation.
// A result with no errors means the profile is valid.
type ValidationResult struct {
	Errors []ValidationError
}

func (r *ValidationResult) add(field, msg string) {
	r.Errors = append(r.Errors, ValidationError{Field: field, Message: msg})
}

func (r *ValidationResult) addf(field, format string, args ...any) {
	r.add(field, fmt.Sprintf(format, args...))
}

// Valid returns true when there are no validation errors.
func (r *ValidationResult) Valid() bool {
	return len(r.Errors) == 0
}

// Error returns a formatted multi-line error string, or empty if valid.
func (r *ValidationResult) Error() string {
	if r.Valid() {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d validation error(s):\n", len(r.Errors)))
	for _, e := range r.Errors {
		sb.WriteString("  • ")
		sb.WriteString(e.Error())
		sb.WriteByte('\n')
	}
	return sb.String()
}

// Validate performs full validation of a Profile and returns the result.
// All errors are collected — validation is not short-circuited on the first
// failure so that callers can surface everything at once.
func Validate(p *Profile) *ValidationResult {
	r := &ValidationResult{}

	validateMeta(p, r)
	validateHardware(&p.Hardware, r)
	validateBrowser(&p.Browser, r)
	validateScreen(&p.Screen, r)
	validateNetwork(&p.Network, r)
	validateStorage(&p.Storage, r)
	validatePermissions(&p.Permissions, r)

	return r
}

// validateMeta checks the top-level profile metadata.
func validateMeta(p *Profile, r *ValidationResult) {
	if strings.TrimSpace(p.ID) == "" {
		r.add("id", "must not be empty")
	}
	if strings.TrimSpace(p.Name) == "" {
		r.add("name", "must not be empty")
	}
	if p.SchemaVersion < 1 || p.SchemaVersion > CurrentSchemaVersion {
		r.addf("schema_version", "must be between 1 and %d, got %d",
			CurrentSchemaVersion, p.SchemaVersion)
	}
	if p.CreatedAt.IsZero() {
		r.add("created_at", "must not be zero")
	}
	if p.UpdatedAt.IsZero() {
		r.add("updated_at", "must not be zero")
	}
	if !p.UpdatedAt.IsZero() && !p.CreatedAt.IsZero() && p.UpdatedAt.Before(p.CreatedAt) {
		r.add("updated_at", "must not be before created_at")
	}
	if p.CreatedAt.After(time.Now().Add(5 * time.Minute)) {
		r.add("created_at", "must not be in the future")
	}
}

// validateHardware checks hardware fingerprint field constraints.
func validateHardware(h *HardwareProfile, r *ValidationResult) {
	validCores := []int{1, 2, 4, 6, 8, 10, 12, 16, 20, 24, 32}
	found := false
	for _, v := range validCores {
		if h.CPUCores == v {
			found = true
			break
		}
	}
	if !found {
		r.addf("hardware.cpu_cores",
			"must be a realistic value (1,2,4,6,8,10,12,16,20,24,32), got %d", h.CPUCores)
	}

	if strings.TrimSpace(h.GPUVendor) == "" {
		r.add("hardware.gpu_vendor", "must not be empty")
	}
	if strings.TrimSpace(h.GPURenderer) == "" {
		r.add("hardware.gpu_renderer", "must not be empty")
	}

	validRAM := []int{512, 1024, 2048, 4096, 8192, 16384, 32768, 65536}
	foundRAM := false
	for _, v := range validRAM {
		if h.RAMMb == v {
			foundRAM = true
			break
		}
	}
	if !foundRAM {
		r.addf("hardware.ram_mb",
			"must be a power-of-two value in MB (512..65536), got %d", h.RAMMb)
	}

	validPlatforms := map[string]bool{
		"Win32": true, "Win64": true, "Linux x86_64": true,
		"Linux aarch64": true, "MacIntel": true, "MacPPC": true,
	}
	if !validPlatforms[h.Platform] {
		r.addf("hardware.platform", "unrecognised platform %q", h.Platform)
	}

	if h.MaxTouchPoints < 0 || h.MaxTouchPoints > 10 {
		r.addf("hardware.max_touch_points", "must be 0–10, got %d", h.MaxTouchPoints)
	}
}

// validateBrowser checks navigator identity strings.
func validateBrowser(b *BrowserProfile, r *ValidationResult) {
	if strings.TrimSpace(b.UserAgent) == "" {
		r.add("browser.user_agent", "must not be empty")
	} else if !strings.HasPrefix(b.UserAgent, "Mozilla/") {
		r.add("browser.user_agent", `must begin with "Mozilla/"`)
	}

	if strings.TrimSpace(b.AppVersion) == "" {
		r.add("browser.app_version", "must not be empty")
	}
	if strings.TrimSpace(b.Vendor) == "" {
		r.add("browser.vendor", "must not be empty")
	}
	if strings.TrimSpace(b.Product) == "" {
		r.add("browser.product", "must not be empty")
	}
	if len(b.Languages) == 0 {
		r.add("browser.languages", "must contain at least one language tag")
	}
	for i, lang := range b.Languages {
		if strings.TrimSpace(lang) == "" {
			r.addf("browser.languages[%d]", i, "must not be empty")
		}
	}
	if b.DoNotTrack != "" && b.DoNotTrack != "0" && b.DoNotTrack != "1" {
		r.addf("browser.do_not_track", `must be "", "0", or "1", got %q`, b.DoNotTrack)
	}
}

// validateScreen checks screen geometry consistency.
func validateScreen(s *ScreenProfile, r *ValidationResult) {
	if s.Width < 320 || s.Width > 7680 {
		r.addf("screen.width", "must be 320–7680, got %d", s.Width)
	}
	if s.Height < 240 || s.Height > 4320 {
		r.addf("screen.height", "must be 240–4320, got %d", s.Height)
	}
	if s.AvailWidth > s.Width {
		r.addf("screen.avail_width", "must be ≤ width (%d), got %d", s.Width, s.AvailWidth)
	}
	if s.AvailHeight > s.Height {
		r.addf("screen.avail_height", "must be ≤ height (%d), got %d", s.Height, s.AvailHeight)
	}
	if s.ColorDepth != 24 && s.ColorDepth != 30 && s.ColorDepth != 32 {
		r.addf("screen.color_depth", "must be 24, 30, or 32, got %d", s.ColorDepth)
	}
	if s.PixelDepth != s.ColorDepth {
		r.addf("screen.pixel_depth", "must equal color_depth (%d), got %d",
			s.ColorDepth, s.PixelDepth)
	}
	validDPR := []float64{0.75, 1.0, 1.25, 1.5, 2.0, 2.5, 3.0}
	foundDPR := false
	for _, v := range validDPR {
		if s.DevicePixelRatio == v {
			foundDPR = true
			break
		}
	}
	if !foundDPR {
		r.addf("screen.device_pixel_ratio",
			"must be a standard value (0.75, 1.0, 1.25, 1.5, 2.0, 2.5, 3.0), got %v",
			s.DevicePixelRatio)
	}
	validOrientations := map[Orientation]bool{
		OrientationLandscapePrimary:   true,
		OrientationLandscapeSecondary: true,
		OrientationPortraitPrimary:    true,
		OrientationPortraitSecondary:  true,
	}
	if !validOrientations[s.Orientation] {
		r.addf("screen.orientation", "unrecognised orientation %q", s.Orientation)
	}
}

// validateNetwork checks proxy URL and timezone sanity.
func validateNetwork(n *NetworkProfile, r *ValidationResult) {
	if strings.TrimSpace(n.Timezone) == "" {
		r.add("network.timezone", "must not be empty")
	}
	if n.ProxyURL != "" {
		u, err := url.Parse(n.ProxyURL)
		if err != nil {
			r.addf("network.proxy_url", "invalid URL: %v", err)
		} else {
			switch u.Scheme {
			case "http", "https", "socks5":
			default:
				r.addf("network.proxy_url", "unsupported scheme %q (use http, https, or socks5)", u.Scheme)
			}
			if u.Host == "" {
				r.add("network.proxy_url", "host must not be empty")
			}
		}
	}
}

// validateStorage checks storage policy fields.
func validateStorage(s *StoragePolicy, r *ValidationResult) {
	if s.MaxCookieJarMB < 0 {
		r.addf("storage.max_cookie_jar_mb", "must be ≥ 0, got %d", s.MaxCookieJarMB)
	}
	if s.MaxStorageMB < 0 {
		r.addf("storage.max_storage_mb", "must be ≥ 0, got %d", s.MaxStorageMB)
	}
}

// validatePermissions checks that every permission state is a known value.
func validatePermissions(p *PermissionsPolicy, r *ValidationResult) {
	check := func(field string, state PermissionState) {
		switch state {
		case PermissionDeny, PermissionPrompt, PermissionGrant:
		default:
			r.addf("permissions.%s", field,
				`must be "deny", "prompt", or "grant", got %q`, state)
		}
	}
	check("geolocation", p.Geolocation)
	check("notifications", p.Notifications)
	check("microphone", p.Microphone)
	check("camera", p.Camera)
	check("clipboard", p.Clipboard)
	check("full_screen", p.FullScreen)
	check("payment_handler", p.PaymentHandler)
	check("midi", p.MIDI)
	check("usb", p.USB)
	check("bluetooth", p.Bluetooth)
}

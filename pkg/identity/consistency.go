package identity

import (
	"fmt"
	"strings"
)

// ConsistencyError describes a cross-field inconsistency in a profile.
// Unlike ValidationError (which checks individual fields), ConsistencyError
// describes problems that only appear when two or more fields are compared.
type ConsistencyError struct {
	Fields  []string
	Message string
}

func (e ConsistencyError) Error() string {
	return fmt.Sprintf("consistency: [%s] %s", strings.Join(e.Fields, ", "), e.Message)
}

// ConsistencyResult collects all inconsistencies found.
type ConsistencyResult struct {
	Warnings []ConsistencyError // suspicious but not necessarily wrong
	Errors   []ConsistencyError // clearly contradictory combinations
}

func (r *ConsistencyResult) warn(fields []string, msg string) {
	r.Warnings = append(r.Warnings, ConsistencyError{Fields: fields, Message: msg})
}

func (r *ConsistencyResult) fail(fields []string, msg string) {
	r.Errors = append(r.Errors, ConsistencyError{Fields: fields, Message: msg})
}

// Clean returns true when there are no errors (warnings are tolerated).
func (r *ConsistencyResult) Clean() bool { return len(r.Errors) == 0 }

// CheckConsistency analyses cross-field relationships in a profile.
// It does not repeat single-field validation — call Validate() first.
func CheckConsistency(p *Profile) *ConsistencyResult {
	r := &ConsistencyResult{}

	checkUserAgentPlatform(p, r)
	checkScreenOrientation(p, r)
	checkRAMCPU(p, r)
	checkGPUPlatform(p, r)
	checkLanguageTimezone(p, r)
	checkTouchScreen(p, r)

	return r
}

// checkUserAgentPlatform ensures UA and platform agree on OS/architecture.
func checkUserAgentPlatform(p *Profile, r *ConsistencyResult) {
	ua := p.Browser.UserAgent
	plat := p.Hardware.Platform

	// UA says Windows → platform should be Win32 or Win64
	if strings.Contains(ua, "Windows NT") {
		if plat != "Win32" && plat != "Win64" {
			r.fail(
				[]string{"browser.user_agent", "hardware.platform"},
				fmt.Sprintf("user-agent contains Windows NT but platform is %q (expected Win32 or Win64)", plat),
			)
		}
	}

	// UA says Linux → platform should reflect Linux
	if strings.Contains(ua, "Linux") && !strings.Contains(ua, "Android") {
		if !strings.HasPrefix(plat, "Linux") {
			r.warn(
				[]string{"browser.user_agent", "hardware.platform"},
				fmt.Sprintf("user-agent contains Linux but platform is %q", plat),
			)
		}
	}

	// UA says Mac → platform should be MacIntel or MacPPC
	if strings.Contains(ua, "Macintosh") {
		if plat != "MacIntel" && plat != "MacPPC" {
			r.fail(
				[]string{"browser.user_agent", "hardware.platform"},
				fmt.Sprintf("user-agent contains Macintosh but platform is %q", plat),
			)
		}
	}
}

// checkScreenOrientation checks that portrait screens have plausible dimensions.
func checkScreenOrientation(p *Profile, r *ConsistencyResult) {
	s := p.Screen
	isPortrait := s.Orientation == OrientationPortraitPrimary ||
		s.Orientation == OrientationPortraitSecondary

	if isPortrait && s.Width > s.Height {
		r.fail(
			[]string{"screen.width", "screen.height", "screen.orientation"},
			fmt.Sprintf("portrait orientation but width (%d) > height (%d)", s.Width, s.Height),
		)
	}
	if !isPortrait && s.Height > s.Width {
		r.warn(
			[]string{"screen.width", "screen.height", "screen.orientation"},
			fmt.Sprintf("landscape orientation but height (%d) > width (%d) — unusual aspect ratio", s.Height, s.Width),
		)
	}
}

// checkRAMCPU warns about implausible RAM/CPU combinations.
func checkRAMCPU(p *Profile, r *ConsistencyResult) {
	cores := p.Hardware.CPUCores
	ramMb := p.Hardware.RAMMb

	// 32+ cores with only 512 MB is suspicious.
	if cores >= 32 && ramMb < 4096 {
		r.warn(
			[]string{"hardware.cpu_cores", "hardware.ram_mb"},
			fmt.Sprintf("%d cores with only %d MB RAM is unrealistic", cores, ramMb),
		)
	}
	// 1 core with 32 GB+ is suspicious.
	if cores == 1 && ramMb >= 32768 {
		r.warn(
			[]string{"hardware.cpu_cores", "hardware.ram_mb"},
			fmt.Sprintf("1 core with %d MB RAM is unusual", ramMb),
		)
	}
}

// checkGPUPlatform ensures GPU strings are consistent with the reported platform.
func checkGPUPlatform(p *Profile, r *ConsistencyResult) {
	vendor := p.Hardware.GPUVendor
	renderer := p.Hardware.GPURenderer
	plat := p.Hardware.Platform

	// ANGLE D3D11/D3D12 in the renderer string implies Windows.
	isANGLED3D := strings.Contains(renderer, "Direct3D")

	if isANGLED3D && plat != "Win32" && plat != "Win64" {
		r.fail(
			[]string{"hardware.gpu_renderer", "hardware.platform"},
			fmt.Sprintf("gpu_renderer contains Direct3D (Windows ANGLE) but platform is %q", plat),
		)
	}

	// Vendor prefix should start "Google Inc." for ANGLE-wrapped GPUs.
	if strings.Contains(renderer, "ANGLE") && !strings.HasPrefix(vendor, "Google Inc.") {
		r.warn(
			[]string{"hardware.gpu_vendor", "hardware.gpu_renderer"},
			fmt.Sprintf("gpu_renderer uses ANGLE but gpu_vendor is %q (expected \"Google Inc. (...)\")", vendor),
		)
	}
}

// checkLanguageTimezone issues a warning when the primary language and timezone
// region are clearly different continents — not always wrong, but suspicious.
func checkLanguageTimezone(p *Profile, r *ConsistencyResult) {
	lang := p.Browser.PrimaryLanguage()
	tz := p.Network.Timezone

	type pair struct{ lang, tzPrefix string }
	obviousMismatches := []pair{
		{"zh-CN", "America/"},
		{"zh-TW", "America/"},
		{"ja", "America/"},
		{"ko", "America/"},
		{"ar", "America/"},
		{"he", "America/"},
	}
	for _, m := range obviousMismatches {
		if strings.HasPrefix(lang, m.lang) && strings.HasPrefix(tz, m.tzPrefix) {
			r.warn(
				[]string{"browser.languages", "network.timezone"},
				fmt.Sprintf("primary language %q with timezone %q is geographically suspicious", lang, tz),
			)
			break
		}
	}
}

// checkTouchScreen checks that MaxTouchPoints is consistent with screen type.
func checkTouchScreen(p *Profile, r *ConsistencyResult) {
	touch := p.Hardware.MaxTouchPoints
	isPortrait := p.Screen.Orientation == OrientationPortraitPrimary ||
		p.Screen.Orientation == OrientationPortraitSecondary

	// Portrait + zero touch points is unusual (most portrait devices are touch).
	if isPortrait && touch == 0 {
		r.warn(
			[]string{"hardware.max_touch_points", "screen.orientation"},
			"portrait orientation with 0 touch points — uncommon for touch-capable devices",
		)
	}
}

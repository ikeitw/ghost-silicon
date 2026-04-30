package identity

import (
	"time"

	"github.com/google/uuid"
)

// TemplateName identifies a built-in profile template.
type TemplateName string

const (
	TemplateWindows11Desktop TemplateName = "windows11-desktop"
	TemplateWindows11Laptop  TemplateName = "windows11-laptop"
	TemplateMobileLike       TemplateName = "mobile-like"
	TemplateHardened         TemplateName = "hardened"
)

// TemplateNames returns all available template identifiers.
func TemplateNames() []TemplateName {
	return []TemplateName{
		TemplateWindows11Desktop,
		TemplateWindows11Laptop,
		TemplateMobileLike,
		TemplateHardened,
	}
}

// FromTemplate instantiates a named template, assigning a fresh ID and
// timestamps.  Returns nil if the name is not recognised.
func FromTemplate(name TemplateName) *Profile {
	switch name {
	case TemplateWindows11Desktop:
		return Windows11DesktopTemplate()
	case TemplateWindows11Laptop:
		return Windows11LaptopTemplate()
	case TemplateMobileLike:
		return MobileLikeTemplate()
	case TemplateHardened:
		return HardenedTemplate()
	default:
		return nil
	}
}

// Windows11DesktopTemplate returns a profile that mimics a mainstream Windows
// 11 desktop with a mid-range NVIDIA GPU and 1080p display.
func Windows11DesktopTemplate() *Profile {
	now := time.Now().UTC()
	return &Profile{
		ID:            uuid.New().String(),
		Name:          "windows11-desktop",
		Description:   "Mainstream Windows 11 desktop — 8-core CPU, RTX 3060, 1080p",
		SchemaVersion: CurrentSchemaVersion,
		CreatedAt:     now,
		UpdatedAt:     now,
		Tags:          []string{"windows11", "desktop", "nvidia"},

		Hardware: HardwareProfile{
			CPUCores:       8,
			GPUVendor:      "Google Inc. (NVIDIA)",
			GPURenderer:    "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Direct3D11 vs_5_0 ps_5_0, D3D11)",
			RAMMb:          16384,
			Platform:       "Win32",
			MaxTouchPoints: 0,
		},

		Browser: BrowserProfile{
			UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
			AppVersion:    "5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
			Vendor:        "Google Inc.",
			Product:       "Gecko",
			ProductSub:    "20030107",
			Languages:     []string{"en-US", "en"},
			DoNotTrack:    "",
			CookieEnabled: true,
		},

		Screen: ScreenProfile{
			Width:            1920,
			Height:           1080,
			AvailWidth:       1920,
			AvailHeight:      1040,
			ColorDepth:       24,
			PixelDepth:       24,
			DevicePixelRatio: 1.0,
			Orientation:      OrientationLandscapePrimary,
		},

		Network: NetworkProfile{
			Timezone: "America/New_York",
		},

		Noise: NoiseProfile{
			CanvasSeed: 0,
			AudioSeed:  0,
			WebGLSeed:  0,
			FontSeed:   0,
		},

		Storage: StoragePolicy{
			EnableCookies:        true,
			EnableLocalStorage:   true,
			EnableSessionStorage: true,
			EnableIndexedDB:      true,
			EnableCacheStorage:   true,
			EnableServiceWorker:  true,
		},

		Permissions: PermissionsPolicy{
			Geolocation:    PermissionDeny,
			Notifications:  PermissionDeny,
			Microphone:     PermissionDeny,
			Camera:         PermissionDeny,
			Clipboard:      PermissionPrompt,
			FullScreen:     PermissionPrompt,
			PaymentHandler: PermissionDeny,
			MIDI:           PermissionDeny,
			USB:            PermissionDeny,
			Bluetooth:      PermissionDeny,
		},
	}
}

// Windows11LaptopTemplate returns a profile resembling a Windows 11 laptop
// with integrated Intel graphics and a 1366×768 or 1920×1080 display at
// 1.25× DPI scaling.
func Windows11LaptopTemplate() *Profile {
	p := Windows11DesktopTemplate()
	p.Name = "windows11-laptop"
	p.Description = "Windows 11 laptop — 4-core CPU, Intel UHD, 1366×768 at 1× DPI"
	p.Tags = []string{"windows11", "laptop", "intel"}

	p.Hardware.CPUCores = 4
	p.Hardware.RAMMb = 8192
	p.Hardware.GPUVendor = "Google Inc. (Intel)"
	p.Hardware.GPURenderer = "ANGLE (Intel, Intel(R) UHD Graphics 620 Direct3D11 vs_5_0 ps_5_0, D3D11)"
	p.Hardware.MaxTouchPoints = 0

	p.Screen.Width = 1366
	p.Screen.Height = 768
	p.Screen.AvailWidth = 1366
	p.Screen.AvailHeight = 728
	p.Screen.DevicePixelRatio = 1.0

	p.Network.Timezone = "Europe/Berlin"
	p.Browser.Languages = []string{"de-DE", "de", "en-US", "en"}

	return p
}

// MobileLikeTemplate returns a profile that mimics a high-DPI mobile-class
// device running Windows 11 in tablet mode.
func MobileLikeTemplate() *Profile {
	p := Windows11DesktopTemplate()
	p.Name = "mobile-like"
	p.Description = "Mobile-style viewport — touch-enabled, high-DPI, portrait"
	p.Tags = []string{"windows11", "mobile", "touch"}

	p.Hardware.CPUCores = 4
	p.Hardware.RAMMb = 4096
	p.Hardware.GPUVendor = "Google Inc. (Qualcomm)"
	p.Hardware.GPURenderer = "ANGLE (Qualcomm, Qualcomm(R) Adreno(TM) 650 Direct3D11 vs_5_0 ps_5_0, D3D11)"
	p.Hardware.MaxTouchPoints = 5

	p.Screen.Width = 390
	p.Screen.Height = 844
	p.Screen.AvailWidth = 390
	p.Screen.AvailHeight = 844
	p.Screen.DevicePixelRatio = 3.0
	p.Screen.Orientation = OrientationPortraitPrimary

	p.Browser.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"
	p.Browser.Languages = []string{"en-US", "en"}

	p.Permissions.Geolocation = PermissionPrompt

	return p
}

// HardenedTemplate returns a privacy-maximised profile with all optional
// APIs denied and noise seeds set to non-zero safe values.
func HardenedTemplate() *Profile {
	p := Windows11DesktopTemplate()
	p.Name = "hardened"
	p.Description = "Maximum privacy — all optional APIs denied, storage restricted"
	p.Tags = []string{"hardened", "privacy"}

	// Fixed, well-known noise seeds (same across sessions for consistency).
	p.Noise.CanvasSeed = 7493852614927364
	p.Noise.AudioSeed = 3829473615728394
	p.Noise.WebGLSeed = 5847362917483920
	p.Noise.FontSeed = 1928374650183746

	p.Storage = StoragePolicy{
		EnableCookies:        true,
		EnableLocalStorage:   false,
		EnableSessionStorage: true,
		EnableIndexedDB:      false,
		EnableCacheStorage:   false,
		EnableServiceWorker:  false,
		MaxCookieJarMB:       10,
		MaxStorageMB:         50,
	}

	p.Permissions = PermissionsPolicy{
		Geolocation:    PermissionDeny,
		Notifications:  PermissionDeny,
		Microphone:     PermissionDeny,
		Camera:         PermissionDeny,
		Clipboard:      PermissionDeny,
		FullScreen:     PermissionDeny,
		PaymentHandler: PermissionDeny,
		MIDI:           PermissionDeny,
		USB:            PermissionDeny,
		Bluetooth:      PermissionDeny,
	}

	// Hardened profile reports minimal but plausible hardware.
	p.Hardware.CPUCores = 4
	p.Hardware.RAMMb = 8192
	p.Browser.DoNotTrack = "1"

	return p
}

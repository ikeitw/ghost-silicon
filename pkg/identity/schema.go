package identity

import (
	"encoding/json"
	"fmt"
)

// schemaV0 is the raw shape of a version-0 profile (pre-schema-version field).
// Ghost-Silicon never wrote v0 files; this handles hand-crafted or imported
// profiles that lack the schema_version key.
type schemaV0 struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	CPUCores   int             `json:"cpu_cores"`
	GPUVendor  string          `json:"gpu_vendor"`
	RAM        int             `json:"ram"`
	CanvasSeed int64           `json:"canvas_seed"`
	UserAgent  string          `json:"user_agent"`
	Extra      json.RawMessage `json:"-"`
}

// Migrate takes the raw JSON bytes of an on-disk profile and returns a fully
// populated *Profile at CurrentSchemaVersion, applying any necessary
// migrations in order.
//
// If the data is already at CurrentSchemaVersion no copies are made.
func Migrate(data []byte) (*Profile, error) {
	// Peek at the schema_version field without full decode.
	var peek struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, fmt.Errorf("identity/schema: cannot read schema_version: %w", err)
	}

	version := peek.SchemaVersion

	switch {
	case version == 0:
		return migrateV0toV1(data)
	case version == CurrentSchemaVersion:
		var p Profile
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("identity/schema: decode v%d profile: %w", version, err)
		}
		return &p, nil
	case version > CurrentSchemaVersion:
		return nil, fmt.Errorf("identity/schema: profile schema v%d is newer than this build (max v%d); upgrade ghost-silicon",
			version, CurrentSchemaVersion)
	default:
		return nil, fmt.Errorf("identity/schema: unsupported schema version %d", version)
	}
}

// migrateV0toV1 converts a flat / hand-written v0 JSON profile to the v1
// nested structure. Fields that have no v0 equivalent receive safe defaults.
func migrateV0toV1(data []byte) (*Profile, error) {
	var v0 schemaV0
	if err := json.Unmarshal(data, &v0); err != nil {
		return nil, fmt.Errorf("identity/schema: decode v0 profile: %w", err)
	}

	p := defaultProfile()
	if v0.ID != "" {
		p.ID = v0.ID
	}
	if v0.Name != "" {
		p.Name = v0.Name
	}
	if v0.CPUCores > 0 {
		p.Hardware.CPUCores = v0.CPUCores
	}
	if v0.GPUVendor != "" {
		p.Hardware.GPUVendor = v0.GPUVendor
	}
	if v0.RAM > 0 {
		p.Hardware.RAMMb = v0.RAM
	}
	if v0.CanvasSeed != 0 {
		p.Noise.CanvasSeed = v0.CanvasSeed
	}
	if v0.UserAgent != "" {
		p.Browser.UserAgent = v0.UserAgent
	}
	p.SchemaVersion = CurrentSchemaVersion
	return p, nil
}

// defaultProfile returns a minimal valid Profile used as a migration base.
// It must not be exported — callers should use Generator or a named template.
func defaultProfile() *Profile {
	return &Profile{
		SchemaVersion: CurrentSchemaVersion,
		Hardware: HardwareProfile{
			CPUCores:    4,
			GPUVendor:   "Google Inc. (NVIDIA)",
			GPURenderer: "ANGLE (NVIDIA, NVIDIA GeForce GTX 1060 Direct3D11 vs_5_0 ps_5_0, D3D11)",
			RAMMb:       8192,
			Platform:    "Win32",
		},
		Browser: BrowserProfile{
			UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
			AppVersion:    "5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
			Vendor:        "Google Inc.",
			Product:       "Gecko",
			ProductSub:    "20030107",
			Languages:     []string{"en-US", "en"},
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
			Timezone: "UTC",
		},
		Noise: NoiseProfile{},
		Storage: StoragePolicy{
			EnableCookies:        true,
			EnableLocalStorage:   true,
			EnableSessionStorage: true,
			EnableIndexedDB:      true,
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

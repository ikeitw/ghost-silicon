// test/identity/profile_test.go
package identity_test

import (
	"testing"
	"time"

	"ghost-silicon/pkg/identity"
)

func TestProfileValidation_ValidProfile(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	result := identity.Validate(p)
	if !result.Valid() {
		t.Fatalf("expected valid profile, got errors:\n%s", result.Error())
	}
}

func TestProfileValidation_MissingID(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.ID = ""
	result := identity.Validate(p)
	if result.Valid() {
		t.Fatal("expected validation to fail for empty ID")
	}
}

func TestProfileValidation_InvalidCPUCores(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Hardware.CPUCores = 7 // not a valid value
	result := identity.Validate(p)
	if result.Valid() {
		t.Fatal("expected validation to fail for invalid cpu_cores=7")
	}
}

func TestProfileValidation_InvalidRAM(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Hardware.RAMMb = 3000 // not a power-of-two bucket
	result := identity.Validate(p)
	if result.Valid() {
		t.Fatal("expected validation to fail for invalid ram_mb=3000")
	}
}

func TestProfileValidation_InvalidUserAgent(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Browser.UserAgent = "NotMozilla/5.0"
	result := identity.Validate(p)
	if result.Valid() {
		t.Fatal("expected validation to fail for UA not starting with Mozilla/")
	}
}

func TestProfileValidation_InvalidScreenDPR(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Screen.DevicePixelRatio = 1.7 // not a standard value
	result := identity.Validate(p)
	if result.Valid() {
		t.Fatal("expected validation to fail for non-standard DPR 1.7")
	}
}

func TestProfileValidation_UpdatedAtBeforeCreatedAt(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt.Add(-time.Hour)
	result := identity.Validate(p)
	if result.Valid() {
		t.Fatal("expected validation to fail when updated_at < created_at")
	}
}

func TestProfileValidation_InvalidPermissionState(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Permissions.Geolocation = "maybe" // not a valid state
	result := identity.Validate(p)
	if result.Valid() {
		t.Fatal("expected validation to fail for invalid permission state")
	}
}

func TestDeviceMemoryGB_Bucketing(t *testing.T) {
	cases := []struct {
		ramMb    int
		expected float64
	}{
		{512, 0.5}, // 0.5 GB → bucket 0.5
		{1024, 1},  // 1 GB   → bucket 1
		{2048, 2},  // 2 GB   → bucket 2
		{4096, 4},  // 4 GB   → bucket 4
		{8192, 8},  // 8 GB   → bucket 8
		{16384, 8}, // 16 GB  → capped at 8
		{32768, 8}, // 32 GB  → capped at 8
	}
	for _, c := range cases {
		h := identity.HardwareProfile{RAMMb: c.ramMb}
		got := h.DeviceMemoryGB()
		if got != c.expected {
			t.Errorf("RAMMb=%d: expected DeviceMemoryGB=%v, got %v",
				c.ramMb, c.expected, got)
		}
	}
}

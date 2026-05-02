// tools/profile-inspector/inspector.go
// Inspector helpers — comparison and diff utilities for profiles.
package main

import (
	"fmt"
	"strings"

	"ghost-silicon/pkg/identity"
)

// compareProfiles prints field-level differences between two profiles.
func compareProfiles(a, b *identity.Profile) {
	fmt.Printf("Comparing %q vs %q\n", a.Name, b.Name)
	diffs := []string{}

	if a.Hardware.CPUCores != b.Hardware.CPUCores {
		diffs = append(diffs, fmt.Sprintf("  cpu_cores: %d → %d", a.Hardware.CPUCores, b.Hardware.CPUCores))
	}
	if a.Hardware.RAMMb != b.Hardware.RAMMb {
		diffs = append(diffs, fmt.Sprintf("  ram_mb: %d → %d", a.Hardware.RAMMb, b.Hardware.RAMMb))
	}
	if a.Hardware.GPUVendor != b.Hardware.GPUVendor {
		diffs = append(diffs, fmt.Sprintf("  gpu_vendor: %q → %q", a.Hardware.GPUVendor, b.Hardware.GPUVendor))
	}
	if a.Hardware.Platform != b.Hardware.Platform {
		diffs = append(diffs, fmt.Sprintf("  platform: %q → %q", a.Hardware.Platform, b.Hardware.Platform))
	}
	if a.Browser.UserAgent != b.Browser.UserAgent {
		diffs = append(diffs, fmt.Sprintf("  user_agent changed"))
	}
	if a.Screen.Width != b.Screen.Width || a.Screen.Height != b.Screen.Height {
		diffs = append(diffs, fmt.Sprintf("  screen: %dx%d → %dx%d",
			a.Screen.Width, a.Screen.Height, b.Screen.Width, b.Screen.Height))
	}
	if a.Network.Timezone != b.Network.Timezone {
		diffs = append(diffs, fmt.Sprintf("  timezone: %q → %q", a.Network.Timezone, b.Network.Timezone))
	}
	if a.Noise.CanvasSeed != b.Noise.CanvasSeed {
		diffs = append(diffs, fmt.Sprintf("  canvas_seed: %d → %d", a.Noise.CanvasSeed, b.Noise.CanvasSeed))
	}

	if len(diffs) == 0 {
		fmt.Println("  No differences found.")
		return
	}
	fmt.Printf("  %d difference(s):\n", len(diffs))
	fmt.Println(strings.Join(diffs, "\n"))
}

// pkg/bridge/hardware.go
// Package bridge — hardware provider.
// Serves navigator.hardwareConcurrency, navigator.deviceMemory,
// and WebGL UNMASKED_VENDOR/RENDERER extension values from the active profile.
package bridge

import "ghost-silicon/pkg/identity"

// HardwareProvider extracts hardware values from a profile.
// It is stateless — the Bridge holds the profile reference.
type HardwareProvider struct{}

// CPUCores returns the value to report for navigator.hardwareConcurrency.
func (HardwareProvider) CPUCores(p *identity.Profile) int {
	return p.Hardware.CPUCores
}

// DeviceMemoryGB returns the bucketed value for navigator.deviceMemory.
func (HardwareProvider) DeviceMemoryGB(p *identity.Profile) float64 {
	return p.Hardware.DeviceMemoryGB()
}

// GPUVendor returns the value reported for UNMASKED_VENDOR_WEBGL.
func (HardwareProvider) GPUVendor(p *identity.Profile) string {
	return p.Hardware.GPUVendor
}

// GPURenderer returns the value reported for UNMASKED_RENDERER_WEBGL.
func (HardwareProvider) GPURenderer(p *identity.Profile) string {
	return p.Hardware.GPURenderer
}

// Platform returns the value reported for navigator.platform.
func (HardwareProvider) Platform(p *identity.Profile) string {
	return p.Hardware.Platform
}

// MaxTouchPoints returns the value for navigator.maxTouchPoints.
func (HardwareProvider) MaxTouchPoints(p *identity.Profile) int {
	return p.Hardware.MaxTouchPoints
}

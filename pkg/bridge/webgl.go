// pkg/bridge/webgl.go
// Package bridge — WebGL noise provider.
// Serves the per-session WebGL noise seed and GPU identity strings.
// The renderer-side script uses the seed to apply deterministic noise to
// WebGL parameter readback (getParameter, readPixels).
package bridge

import "ghost-silicon/pkg/identity"

// WebGLProvider extracts WebGL-related values from a profile.
type WebGLProvider struct{}

// Seed returns the WebGL noise seed for the session.
// A seed of 0 means no noise is applied.
func (WebGLProvider) Seed(p *identity.Profile) int64 {
	return p.Noise.WebGLSeed
}

// NoiseEnabled reports whether WebGL parameter noise is active.
func (WebGLProvider) NoiseEnabled(p *identity.Profile) bool {
	return p.Noise.WebGLSeed != 0
}

// UnmaskedVendor returns the string to report for UNMASKED_VENDOR_WEBGL.
func (WebGLProvider) UnmaskedVendor(p *identity.Profile) string {
	return p.Hardware.GPUVendor
}

// UnmaskedRenderer returns the string to report for UNMASKED_RENDERER_WEBGL.
func (WebGLProvider) UnmaskedRenderer(p *identity.Profile) string {
	return p.Hardware.GPURenderer
}

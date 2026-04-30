// pkg/bridge/canvas.go
// Package bridge — canvas noise provider.
// Serves the per-session canvas noise seed from the active profile.
// The renderer-side script uses this seed to apply deterministic pixel-level
// noise to Canvas2D readback operations (getImageData, toDataURL, toBlob).
package bridge

import "ghost-silicon/pkg/identity"

// CanvasProvider extracts canvas noise configuration from a profile.
type CanvasProvider struct{}

// Seed returns the canvas noise seed for the session.
// A seed of 0 means no noise is applied.
func (CanvasProvider) Seed(p *identity.Profile) int64 {
	return p.Noise.CanvasSeed
}

// NoiseEnabled reports whether canvas noise is active for this profile.
func (CanvasProvider) NoiseEnabled(p *identity.Profile) bool {
	return p.Noise.CanvasSeed != 0
}

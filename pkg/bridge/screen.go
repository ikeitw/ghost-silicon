// pkg/bridge/screen.go
// Package bridge — screen provider.
// Serves window.screen.* values from the active profile.
package bridge

import "ghost-silicon/pkg/identity"

// ScreenProvider extracts screen geometry values from a profile.
type ScreenProvider struct{}

// Width returns screen.width.
func (ScreenProvider) Width(p *identity.Profile) int { return p.Screen.Width }

// Height returns screen.height.
func (ScreenProvider) Height(p *identity.Profile) int { return p.Screen.Height }

// AvailWidth returns screen.availWidth.
func (ScreenProvider) AvailWidth(p *identity.Profile) int { return p.Screen.AvailWidth }

// AvailHeight returns screen.availHeight.
func (ScreenProvider) AvailHeight(p *identity.Profile) int { return p.Screen.AvailHeight }

// ColorDepth returns screen.colorDepth.
func (ScreenProvider) ColorDepth(p *identity.Profile) int { return p.Screen.ColorDepth }

// PixelDepth returns screen.pixelDepth.
func (ScreenProvider) PixelDepth(p *identity.Profile) int { return p.Screen.PixelDepth }

// DevicePixelRatio returns window.devicePixelRatio.
func (ScreenProvider) DevicePixelRatio(p *identity.Profile) float64 {
	return p.Screen.DevicePixelRatio
}

// Orientation returns the screen orientation type string.
func (ScreenProvider) Orientation(p *identity.Profile) string {
	return string(p.Screen.Orientation)
}

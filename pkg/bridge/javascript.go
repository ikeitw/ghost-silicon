// pkg/bridge/javascript.go
// Package bridge — JavaScript API policy provider.
// Determines which JavaScript APIs are exposed or suppressed in the renderer.
// The policy is read-only from the bridge's perspective; enforcement happens
// inside the renderer via the adapter's injection scripts.
package bridge

import "ghost-silicon/pkg/identity"

// JSAPIPolicy describes which JavaScript APIs are active for a session.
type JSAPIPolicy struct {
	// CanvasNoiseEnabled enables pixel-level noise on Canvas2D readback.
	CanvasNoiseEnabled bool `json:"canvas_noise_enabled"`

	// AudioNoiseEnabled enables noise on AudioContext getChannelData.
	AudioNoiseEnabled bool `json:"audio_noise_enabled"`

	// WebGLNoiseEnabled enables noise on WebGL parameter readback.
	WebGLNoiseEnabled bool `json:"webgl_noise_enabled"`

	// FontNoiseEnabled enables noise on Canvas measureText results.
	FontNoiseEnabled bool `json:"font_noise_enabled"`

	// TimerPrecisionMs is the resolution cap for performance.now() and Date.now().
	// 0 means no cap (full precision).  Typical values: 1, 2, 100.
	TimerPrecisionMs float64 `json:"timer_precision_ms"`

	// HardwareConcurrencyOverride enables navigator.hardwareConcurrency spoofing.
	HardwareConcurrencyOverride bool `json:"hardware_concurrency_override"`

	// DeviceMemoryOverride enables navigator.deviceMemory spoofing.
	DeviceMemoryOverride bool `json:"device_memory_override"`

	// WebRTCPolicy controls WebRTC behaviour.
	// "disable" — WebRTC is disabled entirely.
	// "proxy"   — WebRTC traffic is routed through the configured proxy.
	// "allow"   — WebRTC is allowed (may leak local IP).
	WebRTCPolicy string `json:"webrtc_policy"`
}

// JavaScriptProvider builds the JSAPIPolicy for a session from its profile.
type JavaScriptProvider struct{}

// Policy derives the JavaScript API policy from the given profile.
func (JavaScriptProvider) Policy(p *identity.Profile) *JSAPIPolicy {
	return &JSAPIPolicy{
		CanvasNoiseEnabled:          p.Noise.CanvasSeed != 0,
		AudioNoiseEnabled:           p.Noise.AudioSeed != 0,
		WebGLNoiseEnabled:           p.Noise.WebGLSeed != 0,
		FontNoiseEnabled:            p.Noise.FontSeed != 0,
		TimerPrecisionMs:            0, // full precision by default
		HardwareConcurrencyOverride: true,
		DeviceMemoryOverride:        true,
		WebRTCPolicy:                "disable",
	}
}

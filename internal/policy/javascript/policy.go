// internal/policy/javascript/policy.go
// Package javascript defines the supervisor-level JavaScript API policy.
// It determines which browser APIs the renderer is allowed to expose
// and controls noise injection for privacy-sensitive APIs.
package javascript

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Policy describes which JavaScript APIs are active for a session.
type Policy struct {
	// CanvasNoiseEnabled enables pixel-level noise on Canvas2D readback.
	CanvasNoiseEnabled bool `yaml:"canvas_noise_enabled"`

	// AudioNoiseEnabled enables noise on AudioContext getChannelData.
	AudioNoiseEnabled bool `yaml:"audio_noise_enabled"`

	// WebGLNoiseEnabled enables noise on WebGL parameter readback.
	WebGLNoiseEnabled bool `yaml:"webgl_noise_enabled"`

	// FontNoiseEnabled enables noise on Canvas measureText results.
	FontNoiseEnabled bool `yaml:"font_noise_enabled"`

	// HardwareConcurrencyOverride enables navigator.hardwareConcurrency spoofing.
	HardwareConcurrencyOverride bool `yaml:"hardware_concurrency_override"`

	// DeviceMemoryOverride enables navigator.deviceMemory spoofing.
	DeviceMemoryOverride bool `yaml:"device_memory_override"`

	// WebRTCPolicy controls WebRTC behaviour: "disable", "proxy", or "allow".
	WebRTCPolicy string `yaml:"webrtc_policy"`

	// TimerPrecisionMs caps performance.now() and Date.now() resolution (0 = full).
	TimerPrecisionMs float64 `yaml:"timer_precision_ms"`
}

// DefaultPolicy returns a policy that enables all overrides with WebRTC disabled.
func DefaultPolicy() *Policy {
	return &Policy{
		CanvasNoiseEnabled:          true,
		AudioNoiseEnabled:           true,
		WebGLNoiseEnabled:           true,
		FontNoiseEnabled:            true,
		HardwareConcurrencyOverride: true,
		DeviceMemoryOverride:        true,
		WebRTCPolicy:                "disable",
		TimerPrecisionMs:            0,
	}
}

// LoadFromFile reads a javascript-policy YAML file and returns a Policy.
func LoadFromFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("javascript/policy: read %q: %w", path, err)
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("javascript/policy: parse %q: %w", path, err)
	}
	return &p, nil
}

// internal/policy/permissions/policy.go
// Package permissions defines the supervisor-level browser permission policy.
// It resolves permission states for requests that arrive over the IPC bridge.
package permissions

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// State is the resolved state for a single browser permission.
type State string

const (
	StateDeny   State = "deny"
	StatePrompt State = "prompt"
	StateGrant  State = "grant"
)

// Policy maps permission names to their configured states.
type Policy struct {
	Geolocation    State `yaml:"geolocation"`
	Notifications  State `yaml:"notifications"`
	Microphone     State `yaml:"microphone"`
	Camera         State `yaml:"camera"`
	Clipboard      State `yaml:"clipboard"`
	FullScreen     State `yaml:"full_screen"`
	PaymentHandler State `yaml:"payment_handler"`
	MIDI           State `yaml:"midi"`
	USB            State `yaml:"usb"`
	Bluetooth      State `yaml:"bluetooth"`
}

// DefaultPolicy returns a deny-all policy.
func DefaultPolicy() *Policy {
	return &Policy{
		Geolocation:    StateDeny,
		Notifications:  StateDeny,
		Microphone:     StateDeny,
		Camera:         StateDeny,
		Clipboard:      StatePrompt,
		FullScreen:     StatePrompt,
		PaymentHandler: StateDeny,
		MIDI:           StateDeny,
		USB:            StateDeny,
		Bluetooth:      StateDeny,
	}
}

// Resolve returns the State for the named permission.
// Unknown names return StateDeny.
func (p *Policy) Resolve(name string) State {
	switch name {
	case "geolocation":
		return p.Geolocation
	case "notifications":
		return p.Notifications
	case "microphone":
		return p.Microphone
	case "camera":
		return p.Camera
	case "clipboard":
		return p.Clipboard
	case "full_screen":
		return p.FullScreen
	case "payment_handler":
		return p.PaymentHandler
	case "midi":
		return p.MIDI
	case "usb":
		return p.USB
	case "bluetooth":
		return p.Bluetooth
	default:
		return StateDeny
	}
}

// LoadFromFile reads a permissions policy YAML file.
func LoadFromFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("permissions/policy: read %q: %w", path, err)
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("permissions/policy: parse %q: %w", path, err)
	}
	return &p, nil
}

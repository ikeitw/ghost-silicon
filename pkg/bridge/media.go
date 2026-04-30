// pkg/bridge/media.go
// Package bridge — media devices provider.
// Controls what MediaDevices.enumerateDevices() returns to the renderer.
// If cameras and microphones are denied by the permissions policy, the
// renderer receives empty device lists so no labels or device IDs leak.
package bridge

import "ghost-silicon/pkg/identity"

// MediaDevice describes a single synthetic media device entry.
type MediaDevice struct {
	DeviceID string `json:"device_id"`
	Kind     string `json:"kind"` // "audioinput" | "audiooutput" | "videoinput"
	Label    string `json:"label"`
	GroupID  string `json:"group_id"`
}

// MediaProvider builds the synthetic media device list for the renderer.
type MediaProvider struct{}

// Devices returns the list of media devices the renderer should report.
// When camera/microphone permissions are denied, only a generic audiooutput
// (speaker) is returned — enough to avoid fingerprinting gaps, not enough
// to leak real hardware identifiers.
func (MediaProvider) Devices(p *identity.Profile) []MediaDevice {
	var devices []MediaDevice

	// Always include a generic speaker output — its presence is expected.
	devices = append(devices, MediaDevice{
		DeviceID: "default",
		Kind:     "audiooutput",
		Label:    "", // label is empty until the user grants permission
		GroupID:  "default",
	})

	// Only enumerate input devices if the relevant permission is not denied.
	if p.Permissions.Microphone != identity.PermissionDeny {
		devices = append(devices, MediaDevice{
			DeviceID: "audioinput-0",
			Kind:     "audioinput",
			Label:    "",
			GroupID:  "default",
		})
	}
	if p.Permissions.Camera != identity.PermissionDeny {
		devices = append(devices, MediaDevice{
			DeviceID: "videoinput-0",
			Kind:     "videoinput",
			Label:    "",
			GroupID:  "camera-0",
		})
	}

	return devices
}

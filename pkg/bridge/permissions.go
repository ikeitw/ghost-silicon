// pkg/bridge/permissions.go
// Package bridge — permissions provider.
// Resolves browser permission states from the active profile policy.
package bridge

import "ghost-silicon/pkg/identity"

// PermissionsProvider resolves permission states from a profile.
type PermissionsProvider struct{}

// State returns the permission state string ("deny", "prompt", or "grant")
// for the named permission.  Unknown names always return "deny".
func (PermissionsProvider) State(p *identity.Profile, name string) string {
	perm := &p.Permissions
	switch name {
	case "geolocation":
		return string(perm.Geolocation)
	case "notifications":
		return string(perm.Notifications)
	case "microphone":
		return string(perm.Microphone)
	case "camera":
		return string(perm.Camera)
	case "clipboard":
		return string(perm.Clipboard)
	case "full_screen":
		return string(perm.FullScreen)
	case "payment_handler":
		return string(perm.PaymentHandler)
	case "midi":
		return string(perm.MIDI)
	case "usb":
		return string(perm.USB)
	case "bluetooth":
		return string(perm.Bluetooth)
	default:
		return string(identity.PermissionDeny)
	}
}

// All returns a map of every permission name to its state string.
func (pp PermissionsProvider) All(p *identity.Profile) map[string]string {
	names := []string{
		"geolocation", "notifications", "microphone", "camera",
		"clipboard", "full_screen", "payment_handler", "midi", "usb", "bluetooth",
	}
	out := make(map[string]string, len(names))
	for _, n := range names {
		out[n] = pp.State(p, n)
	}
	return out
}

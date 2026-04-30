// internal/policy/permissions/grant.go
// Package permissions — grant helper.
// Provides a convenience builder for constructing permissive policies
// used in developer and testing environments.
package permissions

// GrantAll returns a Policy that grants every permission unconditionally.
// Only suitable for isolated developer environments — never for production use.
func GrantAll() *Policy {
	return &Policy{
		Geolocation:    StateGrant,
		Notifications:  StateGrant,
		Microphone:     StateGrant,
		Camera:         StateGrant,
		Clipboard:      StateGrant,
		FullScreen:     StateGrant,
		PaymentHandler: StateGrant,
		MIDI:           StateGrant,
		USB:            StateGrant,
		Bluetooth:      StateGrant,
	}
}

// Grant returns a copy of p with the named permission set to StateGrant.
func Grant(p *Policy, name string) *Policy {
	cp := *p
	switch name {
	case "geolocation":
		cp.Geolocation = StateGrant
	case "notifications":
		cp.Notifications = StateGrant
	case "microphone":
		cp.Microphone = StateGrant
	case "camera":
		cp.Camera = StateGrant
	case "clipboard":
		cp.Clipboard = StateGrant
	case "full_screen":
		cp.FullScreen = StateGrant
	case "payment_handler":
		cp.PaymentHandler = StateGrant
	case "midi":
		cp.MIDI = StateGrant
	case "usb":
		cp.USB = StateGrant
	case "bluetooth":
		cp.Bluetooth = StateGrant
	}
	return &cp
}

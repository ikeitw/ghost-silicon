// internal/policy/permissions/deny.go
// Package permissions — deny helper.
// Provides a convenience builder for constructing restrictive policies.
package permissions

// DenyAll returns a Policy that denies every permission unconditionally.
func DenyAll() *Policy {
	return &Policy{
		Geolocation:    StateDeny,
		Notifications:  StateDeny,
		Microphone:     StateDeny,
		Camera:         StateDeny,
		Clipboard:      StateDeny,
		FullScreen:     StateDeny,
		PaymentHandler: StateDeny,
		MIDI:           StateDeny,
		USB:            StateDeny,
		Bluetooth:      StateDeny,
	}
}

// Deny returns a copy of p with the named permission set to StateDeny.
func Deny(p *Policy, name string) *Policy {
	cp := *p
	switch name {
	case "geolocation":
		cp.Geolocation = StateDeny
	case "notifications":
		cp.Notifications = StateDeny
	case "microphone":
		cp.Microphone = StateDeny
	case "camera":
		cp.Camera = StateDeny
	case "clipboard":
		cp.Clipboard = StateDeny
	case "full_screen":
		cp.FullScreen = StateDeny
	case "payment_handler":
		cp.PaymentHandler = StateDeny
	case "midi":
		cp.MIDI = StateDeny
	case "usb":
		cp.USB = StateDeny
	case "bluetooth":
		cp.Bluetooth = StateDeny
	}
	return &cp
}

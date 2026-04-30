// internal/ipc/messages/permissions.go
// Package messages defines request/response types for the permissions subsystem.
package messages

// PermissionName is a typed browser permission identifier.
type PermissionName string

const (
	PermGeolocation   PermissionName = "geolocation"
	PermNotifications PermissionName = "notifications"
	PermMicrophone    PermissionName = "microphone"
	PermCamera        PermissionName = "camera"
	PermClipboard     PermissionName = "clipboard"
	PermFullScreen    PermissionName = "full_screen"
	PermPayment       PermissionName = "payment_handler"
	PermMIDI          PermissionName = "midi"
	PermUSB           PermissionName = "usb"
	PermBluetooth     PermissionName = "bluetooth"
)

// AllPermissions is the ordered list of all supported permission names.
var AllPermissions = []PermissionName{
	PermGeolocation, PermNotifications, PermMicrophone, PermCamera,
	PermClipboard, PermFullScreen, PermPayment, PermMIDI, PermUSB, PermBluetooth,
}

// AllPermissionsRequest is the request body for permissions.getAll.
// No fields required — returns the complete permissions map.
type AllPermissionsRequest struct{}

// AllPermissionsResponse is returned by permissions.getAll.
// Keys are PermissionName values; values are "deny", "prompt", or "grant".
type AllPermissionsResponse struct {
	Permissions map[string]string `json:"permissions"`
}

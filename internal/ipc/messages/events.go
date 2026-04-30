// internal/ipc/messages/events.go
// Package messages defines the event notification types sent from the
// renderer process back to the supervisor over the named pipe bridge.
package messages

// EventType classifies a renderer-originated event notification.
type EventType string

const (
	// EventRendererReady signals that the renderer has finished initialising
	// and is ready to accept navigation commands.
	EventRendererReady EventType = "renderer.ready"

	// EventRendererCrash signals an internal renderer crash (before process exit).
	EventRendererCrash EventType = "renderer.crash"

	// EventNavigationStart is fired when a top-level navigation begins.
	EventNavigationStart EventType = "navigation.start"

	// EventNavigationComplete is fired when a top-level navigation finishes.
	EventNavigationComplete EventType = "navigation.complete"

	// EventNavigationError is fired when navigation fails.
	EventNavigationError EventType = "navigation.error"

	// EventScriptError is fired when an uncaught JavaScript exception occurs.
	EventScriptError EventType = "script.error"

	// EventPermissionRequest is fired when the page requests a browser permission.
	EventPermissionRequest EventType = "permission.request"

	// EventStorageExceeded is fired when a storage quota is exceeded.
	EventStorageExceeded EventType = "storage.exceeded"
)

// RendererEvent is the notification body sent from the renderer to the supervisor.
type RendererEvent struct {
	Type      EventType         `json:"type"`
	SessionID string            `json:"session_id"`
	URL       string            `json:"url,omitempty"`
	Message   string            `json:"message,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
}

// NavigationPayload carries URL and status for navigation events.
type NavigationPayload struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	ErrorMsg   string `json:"error,omitempty"`
}

// PermissionRequestPayload carries the permission name the page is requesting.
type PermissionRequestPayload struct {
	Permission string `json:"permission"`
	Origin     string `json:"origin"`
}

// MethodRendererEvent is the RPC method name for renderer-to-supervisor events.
const MethodRendererEvent = "renderer.event"

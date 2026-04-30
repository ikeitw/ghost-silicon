// pkg/renderer/interface.go
// Package renderer defines the generic rendering engine adapter contract.
// Ghost-Silicon does not implement a browser engine — it wraps one.
// Any engine (Chromium-based, WebKit-based, or a mock) implements Adapter.
package renderer

import "context"

// Adapter is the contract a rendering engine backend must satisfy.
// The supervisor calls these methods; the implementation handles the
// engine-specific process lifecycle.
type Adapter interface {
	// Start launches the renderer process with the given options.
	// It must return only after the process is running (or fail fast).
	Start(ctx context.Context, opts StartOptions) (Process, error)

	// Name returns a human-readable identifier for this adapter,
	// e.g. "chromium", "webkit", "mock".
	Name() string
}

// Process represents a running renderer process managed by an Adapter.
type Process interface {
	// PID returns the operating system process identifier.
	PID() uint32

	// Wait blocks until the process exits and returns its exit code.
	Wait(ctx context.Context) (uint32, error)

	// Terminate sends a hard kill signal to the process.
	Terminate() error

	// IsRunning reports whether the process is still alive.
	IsRunning() bool
}

// StartOptions carries everything the adapter needs to launch the renderer.
type StartOptions struct {
	// ProfileID is the identity profile assigned to this session.
	ProfileID string

	// SessionID is the unique session identifier.
	SessionID string

	// PipeName is the Windows named pipe path the renderer should connect to.
	PipeName string

	// UserDataDir is the isolated session directory for this renderer instance.
	UserDataDir string

	// CacheDir is the HTTP cache directory.
	CacheDir string

	// ExtraArgs are additional command-line arguments for the renderer binary.
	ExtraArgs []string

	// Env is the environment block for the renderer process.
	// If nil the current process environment is used.
	Env []string
}

// HealthCheck is implemented by adapters that support active health probing.
type HealthCheck interface {
	// Healthy returns nil when the renderer is responding to probes.
	Healthy(ctx context.Context) error
}

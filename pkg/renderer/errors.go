// pkg/renderer/errors.go
// Package renderer — typed error values for the renderer lifecycle.
package renderer

import (
	"errors"
	"fmt"
)

// Sentinel errors — use errors.Is() to test.
var (
	// ErrNotRunning is returned when an operation requires the renderer to be
	// active but it is stopped or crashed.
	ErrNotRunning = errors.New("renderer: process is not running")

	// ErrAlreadyRunning is returned when Start is called on an active session.
	ErrAlreadyRunning = errors.New("renderer: process already running")

	// ErrStartTimeout is returned when the renderer does not become ready
	// within the configured start timeout.
	ErrStartTimeout = errors.New("renderer: timed out waiting for ready signal")

	// ErrCrashLimitExceeded is returned when the restart policy gives up.
	ErrCrashLimitExceeded = errors.New("renderer: crash restart limit exceeded")

	// ErrAdapterNotFound is returned when no adapter is registered for the
	// configured engine name.
	ErrAdapterNotFound = errors.New("renderer: no adapter registered for engine")
)

// LaunchError wraps an OS-level error that occurred during process launch.
type LaunchError struct {
	Executable string
	Cause      error
}

func (e *LaunchError) Error() string {
	return fmt.Sprintf("renderer: failed to launch %q: %v", e.Executable, e.Cause)
}

func (e *LaunchError) Unwrap() error { return e.Cause }

// ExitError records the exit code of a renderer that exited unexpectedly.
type ExitError struct {
	PID      uint32
	ExitCode uint32
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("renderer: PID %d exited with code %d", e.PID, e.ExitCode)
}

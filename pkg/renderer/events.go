// pkg/renderer/events.go
// Package renderer — event types emitted by the renderer process.
// These mirror the IPC message types but live in the pkg layer so that
// higher-level components (engine, supervisor) can import them without
// pulling in the full IPC package.
package renderer

import (
	"context"
	"time"
)

// Event is a lifecycle or navigation event emitted by the renderer.
type Event struct {
	// Type classifies the event.
	Type string

	// SessionID identifies the source session.
	SessionID string

	// At is when the event occurred.
	At time.Time

	// URL is the page URL relevant to the event (navigation events).
	URL string

	// Message carries human-readable detail (errors, crash reasons).
	Message string

	// ExitCode is populated for process-exit events.
	ExitCode uint32
}

// Well-known event type strings — kept in sync with ipc/messages/events.go.
const (
	EvtReady              = "renderer.ready"
	EvtCrash              = "renderer.crash"
	EvtNavigationStart    = "navigation.start"
	EvtNavigationComplete = "navigation.complete"
	EvtNavigationError    = "navigation.error"
	EvtScriptError        = "script.error"
	EvtPermissionRequest  = "permission.request"
	EvtStorageExceeded    = "storage.exceeded"
	EvtProcessExited      = "process.exited"
)

// MockProcessHandle is a test double for renderer.Process.
// Export it so external test packages can use it.
type MockProcessHandle struct {
	PIDVal  uint32
	Running bool
}

func (m *MockProcessHandle) PID() uint32                            { return m.PIDVal }
func (m *MockProcessHandle) IsRunning() bool                        { return m.Running }
func (m *MockProcessHandle) Terminate() error                       { m.Running = false; return nil }
func (m *MockProcessHandle) Wait(_ context.Context) (uint32, error) { return 0, nil }

// NewMockProcess creates a MockProcessHandle with the given PID that reports as running.
func NewMockProcess(pid uint32) *MockProcessHandle {
	return &MockProcessHandle{PIDVal: pid, Running: true}
}

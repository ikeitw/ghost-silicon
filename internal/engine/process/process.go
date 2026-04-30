// internal/engine/process/process.go
// Package process wraps the platform-level process handle into the
// renderer.Process interface used by the engine layer.
package process

import (
	"context"

	"ghost-silicon/pkg/renderer"
)

// Handle wraps a running OS process and satisfies renderer.Process.
// On Windows the concrete implementation lives in
// internal/platform/windows/process; this file provides the shared interface
// glue so the engine layer stays platform-neutral.
type Handle struct {
	pid       uint32
	isRunning func() bool
	wait      func(ctx context.Context) (uint32, error)
	terminate func() error
}

// New wraps raw OS callbacks into a renderer.Process.
func New(
	pid uint32,
	isRunning func() bool,
	wait func(context.Context) (uint32, error),
	terminate func() error,
) renderer.Process {
	return &Handle{
		pid:       pid,
		isRunning: isRunning,
		wait:      wait,
		terminate: terminate,
	}
}

func (h *Handle) PID() uint32                              { return h.pid }
func (h *Handle) IsRunning() bool                          { return h.isRunning() }
func (h *Handle) Wait(ctx context.Context) (uint32, error) { return h.wait(ctx) }
func (h *Handle) Terminate() error                         { return h.terminate() }

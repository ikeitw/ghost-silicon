// pkg/sandbox/process.go
// Package sandbox — cross-platform sandbox interface.
// Defines the Sandbox interface that all platform backends implement,
// and the Options type used to configure isolation per session.
package sandbox

import (
	"context"

	"ghost-silicon/pkg/renderer"
)

// Sandbox launches and supervises renderer processes with platform-specific
// isolation applied.
type Sandbox interface {
	// Start launches the renderer with sandbox settings from opts.
	Start(ctx context.Context, opts Options) (renderer.Process, error)

	// Close releases any OS resources held by the sandbox (Job Objects, etc.).
	Close() error
}

// Options carries everything needed to launch a sandboxed renderer process.
type Options struct {
	// Executable is the full path to the renderer binary.
	Executable string

	// Args are the renderer command-line arguments.
	Args []string

	// Env is the environment block. Nil means inherit the current environment.
	Env []string

	// WorkDir is the renderer working directory.
	WorkDir string

	// SessionID identifies the session for log correlation.
	SessionID string

	// ProfileID identifies the active identity profile.
	ProfileID string

	// PipeName is the named pipe the renderer connects to for IPC.
	PipeName string

	// UserDataDir is the isolated filesystem root for this session.
	UserDataDir string

	// MemoryLimitMB caps the renderer's memory via the OS isolation mechanism.
	// 0 means no limit.
	MemoryLimitMB int64

	// CPURatePercent caps the renderer's CPU usage (1–100, 0 = no limit).
	CPURatePercent int
}

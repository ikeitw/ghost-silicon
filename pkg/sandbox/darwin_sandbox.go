// pkg/sandbox/darwin_sandbox.go
//go:build darwin

// Package sandbox — macOS sandbox stub.
// Phase 1: plain process launch. Future phases will use the macOS sandbox
// profile mechanism (sandbox-exec / Seatbelt).
package sandbox

import (
	"context"
	"fmt"
	"os/exec"

	"ghost-silicon/pkg/renderer"
)

// DarwinSandbox is the macOS implementation of Sandbox.
type DarwinSandbox struct{}

// NewDarwinSandbox creates a DarwinSandbox.
func NewDarwinSandbox() *DarwinSandbox { return &DarwinSandbox{} }

// Start launches the renderer as a plain child process on macOS.
func (s *DarwinSandbox) Start(_ context.Context, opts Options) (renderer.Process, error) {
	cmd := exec.Command(opts.Executable, opts.Args...) //nolint:gosec
	cmd.Dir = opts.WorkDir
	cmd.Env = opts.Env
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sandbox/darwin: start: %w", err)
	}
	return &darwinProcess{cmd: cmd}, nil
}

// Close is a no-op for the basic macOS sandbox.
func (s *DarwinSandbox) Close() error { return nil }

// darwinProcess wraps exec.Cmd as renderer.Process.
type darwinProcess struct{ cmd *exec.Cmd }

func (p *darwinProcess) PID() uint32 {
	if p.cmd.Process == nil {
		return 0
	}
	return uint32(p.cmd.Process.Pid)
}
func (p *darwinProcess) IsRunning() bool {
	return p.cmd.Process != nil && p.cmd.ProcessState == nil
}
func (p *darwinProcess) Terminate() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}
func (p *darwinProcess) Wait(_ context.Context) (uint32, error) {
	if err := p.cmd.Wait(); err != nil {
		if p.cmd.ProcessState != nil {
			return uint32(p.cmd.ProcessState.ExitCode()), nil
		}
		return 1, fmt.Errorf("sandbox/darwin: wait: %w", err)
	}
	return 0, nil
}

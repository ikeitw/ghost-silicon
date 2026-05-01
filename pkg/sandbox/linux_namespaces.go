// pkg/sandbox/linux_namespaces.go
//go:build linux

// Package sandbox — Linux namespace sandbox implementation.
// Future backend: uses Linux namespaces for process isolation.
// Currently a stub that launches the renderer without namespace isolation;
// full implementation follows in a later phase.
package sandbox

import (
	"context"
	"fmt"
	"os/exec"

	"ghost-silicon/pkg/renderer"
)

// LinuxSandbox is the Linux implementation of Sandbox.
// Phase 1: plain process launch without namespaces (safe baseline).
// Phase 2: add UTS, net, mount, user, and PID namespace isolation.
type LinuxSandbox struct{}

// NewLinuxSandbox creates a LinuxSandbox.
func NewLinuxSandbox() *LinuxSandbox { return &LinuxSandbox{} }

// Start launches the renderer as a plain child process on Linux.
func (s *LinuxSandbox) Start(_ context.Context, opts Options) (renderer.Process, error) {
	cmd := exec.Command(opts.Executable, opts.Args...) //nolint:gosec
	cmd.Dir = opts.WorkDir
	cmd.Env = opts.Env

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sandbox/linux: start renderer: %w", err)
	}

	return &linuxProcess{cmd: cmd}, nil
}

// Close is a no-op on Linux for the basic sandbox.
func (s *LinuxSandbox) Close() error { return nil }

// linuxProcess wraps exec.Cmd as renderer.Process.
type linuxProcess struct{ cmd *exec.Cmd }

func (p *linuxProcess) PID() uint32 {
	if p.cmd.Process == nil {
		return 0
	}
	return uint32(p.cmd.Process.Pid)
}

func (p *linuxProcess) IsRunning() bool {
	if p.cmd.Process == nil || p.cmd.ProcessState != nil {
		return false
	}
	return true
}

func (p *linuxProcess) Terminate() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

func (p *linuxProcess) Wait(_ context.Context) (uint32, error) {
	if err := p.cmd.Wait(); err != nil {
		if p.cmd.ProcessState != nil {
			return uint32(p.cmd.ProcessState.ExitCode()), nil
		}
		return 1, fmt.Errorf("sandbox/linux: wait: %w", err)
	}
	return 0, nil
}

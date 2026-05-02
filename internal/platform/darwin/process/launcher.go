// internal/platform/darwin/process/launcher.go
//go:build darwin

// Package process provides the macOS renderer process launcher.
// Phase 1: plain exec.Cmd launch. Future phases will add macOS sandbox profiles.
package process

import (
	"context"
	"fmt"
	"os/exec"
)

// LaunchOptions configures the renderer process on macOS.
type LaunchOptions struct {
	Executable string
	Args       []string
	WorkDir    string
	Env        []string
}

// Process wraps a running macOS renderer process.
type Process struct {
	cmd *exec.Cmd
}

// Launch starts the renderer process on macOS.
func Launch(opts LaunchOptions) (*Process, error) {
	if opts.Executable == "" {
		return nil, fmt.Errorf("darwin/process: executable must not be empty")
	}
	cmd := exec.Command(opts.Executable, opts.Args...) //nolint:gosec
	cmd.Dir = opts.WorkDir
	cmd.Env = opts.Env

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("darwin/process: start %q: %w", opts.Executable, err)
	}
	return &Process{cmd: cmd}, nil
}

// PID returns the OS process identifier.
func (p *Process) PID() uint32 {
	if p.cmd.Process == nil {
		return 0
	}
	return uint32(p.cmd.Process.Pid)
}

// IsRunning reports whether the process is still alive.
func (p *Process) IsRunning() bool {
	return p.cmd.Process != nil && p.cmd.ProcessState == nil
}

// Terminate sends SIGKILL to the process.
func (p *Process) Terminate() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

// Wait blocks until the process exits and returns its exit code.
func (p *Process) Wait(_ context.Context) (uint32, error) {
	if err := p.cmd.Wait(); err != nil {
		if p.cmd.ProcessState != nil {
			return uint32(p.cmd.ProcessState.ExitCode()), nil
		}
		return 1, fmt.Errorf("darwin/process: wait: %w", err)
	}
	return 0, nil
}

//go:build windows

package process

import (
	"context"
	"fmt"

	"golang.org/x/sys/windows"
)

// Wait blocks until the process exits or ctx is cancelled.
// Returns the exit code on success.
func (p *Process) Wait(ctx context.Context) (uint32, error) {
	done := make(chan struct {
		code uint32
		err  error
	}, 1)

	go func() {
		event, err := windows.WaitForSingleObject(p.handle, windows.INFINITE)
		if err != nil {
			done <- struct {
				code uint32
				err  error
			}{0, fmt.Errorf("process/wait: WaitForSingleObject: %w", err)}
			return
		}
		if event != windows.WAIT_OBJECT_0 {
			done <- struct {
				code uint32
				err  error
			}{0, fmt.Errorf("process/wait: unexpected wait result: %v", event)}
			return
		}
		var code uint32
		if err := windows.GetExitCodeProcess(p.handle, &code); err != nil {
			done <- struct {
				code uint32
				err  error
			}{0, fmt.Errorf("process/wait: GetExitCodeProcess: %w", err)}
			return
		}
		done <- struct {
			code uint32
			err  error
		}{code, nil}
	}()

	select {
	case <-ctx.Done():
		_ = windows.TerminateProcess(p.handle, 1)
		return 0, ctx.Err()
	case result := <-done:
		return result.code, result.err
	}
}

// Terminate sends a hard termination signal to the process.
func (p *Process) Terminate(exitCode uint32) error {
	if err := windows.TerminateProcess(p.handle, exitCode); err != nil {
		return fmt.Errorf("process/terminate: %w", err)
	}
	return nil
}

// IsRunning reports whether the process is still alive.
func (p *Process) IsRunning() bool {
	var code uint32
	err := windows.GetExitCodeProcess(p.handle, &code)
	if err != nil {
		return false
	}
	return code == stillActive
}

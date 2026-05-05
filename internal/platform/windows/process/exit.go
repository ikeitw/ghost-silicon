// internal/platform/windows/process/exit.go
//go:build windows

package process

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// stillActive is the STILL_ACTIVE exit code (259) returned by
// GetExitCodeProcess when the process has not yet terminated.
const stillActive = 259

// ExitCode retrieves the exit code of a process handle.
// Returns 259 (STILL_ACTIVE) when the process has not yet exited.
func ExitCode(h windows.Handle) (uint32, error) {
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return 0, fmt.Errorf("process/exit: GetExitCodeProcess: %w", err)
	}
	return code, nil
}

// IsStillActive returns true when the process has not yet exited.
func IsStillActive(h windows.Handle) bool {
	code, err := ExitCode(h)
	if err != nil {
		return false
	}
	return code == stillActive
}

// ExitCodeString returns a human-readable description of a Windows exit code.
func ExitCodeString(code uint32) string {
	switch code {
	case 0:
		return "success (0)"
	case 1:
		return "generic error (1)"
	case stillActive:
		return "still running"
	case 0xC0000005:
		return "access violation (0xC0000005)"
	case 0xC0000409:
		return "stack buffer overrun (0xC0000409)"
	case 0xC000001D:
		return "illegal instruction (0xC000001D)"
	case 0xC0000374:
		return "heap corruption (0xC0000374)"
	default:
		return fmt.Sprintf("0x%08X", code)
	}
}

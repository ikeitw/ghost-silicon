// internal/platform/windows/process/exit.go
//go:build windows

package process

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func ExitCode(h windows.Handle) (uint32, error) {
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return 0, fmt.Errorf("process/exit: GetExitCodeProcess: %w", err)
	}
	return code, nil
}

func IsStillActive(h windows.Handle) bool {
	code, err := ExitCode(h)
	if err != nil {
		return false
	}
	return code == windows.STILL_ACTIVE
}

func ExitCodeString(code uint32) string {
	switch code {
	case 0:
		return "success (0)"
	case 1:
		return "generic error (1)"
	case windows.STILL_ACTIVE:
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

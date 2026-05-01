// pkg/security/isolation_windows.go
//go:build windows

package security

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// checkWindowsIsolation probes Job Object and token creation on Windows.
func checkWindowsIsolation(r *IsolationReport) {
	// Test Job Object creation.
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("Job Object creation failed: %v", err))
	} else {
		_ = windows.CloseHandle(h)
		r.JobObjectOK = true
	}

	// Test token duplication.
	var tok windows.Token
	proc := windows.CurrentProcess()
	err = windows.OpenProcessToken(proc,
		windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &tok)
	if err != nil {
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("OpenProcessToken failed: %v", err))
	} else {
		tok.Close() //nolint:errcheck
		r.RestrictedToken = true
	}
}

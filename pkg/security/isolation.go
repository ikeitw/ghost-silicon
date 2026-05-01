// pkg/security/isolation.go
// Package security — isolation boundary checks.
// Verifies at startup that the required OS-level isolation primitives are
// available before the supervisor tries to use them.
package security

import (
	"fmt"
	"runtime"
)

// IsolationReport describes which isolation features are available.
type IsolationReport struct {
	Platform        string
	JobObjectOK     bool
	RestrictedToken bool
	AppContainer    bool
	Warnings        []string
}

// CheckIsolation probes the current environment and returns a report.
// On Windows it confirms that the process can create Job Objects and
// duplicate tokens. On other platforms it returns placeholder values.
func CheckIsolation() *IsolationReport {
	r := &IsolationReport{Platform: runtime.GOOS}
	switch runtime.GOOS {
	case "windows":
		checkWindowsIsolation(r)
	default:
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("isolation checks not implemented for %s", runtime.GOOS))
	}
	return r
}

// Describe returns a human-readable summary of the isolation report.
func (r *IsolationReport) Describe() string {
	return fmt.Sprintf(
		"platform=%s job_object=%v restricted_token=%v app_container=%v warnings=%d",
		r.Platform, r.JobObjectOK, r.RestrictedToken, r.AppContainer, len(r.Warnings),
	)
}

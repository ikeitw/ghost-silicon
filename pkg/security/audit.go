// pkg/security/audit.go
// Package security — security audit helpers.
// Wraps the telemetry audit logger with security-specific event constructors.
package security

import (
	"ghost-silicon/internal/telemetry/audit"
)

// AuditSecretAccess logs a secret access event.
func AuditSecretAccess(a *audit.Logger, actor, key, outcome string) {
	a.Log(audit.Event{
		Type:    "secret.access",
		Actor:   actor,
		Target:  key,
		Outcome: outcome,
	})
}

// AuditIsolationCheck logs the result of a platform isolation probe.
func AuditIsolationCheck(a *audit.Logger, report *IsolationReport) {
	outcome := "ok"
	if len(report.Warnings) > 0 {
		outcome = "warning"
	}
	a.Log(audit.Event{
		Type:    "isolation.check",
		Actor:   "security",
		Target:  report.Platform,
		Outcome: outcome,
		Message: report.Describe(),
	})
}

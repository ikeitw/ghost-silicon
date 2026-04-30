// internal/policy/profiles/consistency.go
// Package profiles — consistency gate used at profile activation time.
package profiles

import (
	"fmt"
	"strings"

	"ghost-silicon/pkg/identity"
)

// ConsistencyGate runs all consistency checks on p and returns a combined
// error when any hard errors are found.  Warnings are collected and returned
// separately so callers can log them without blocking activation.
func ConsistencyGate(p *identity.Profile) (warnings []string, err error) {
	result := identity.CheckConsistency(p)

	for _, w := range result.Warnings {
		warnings = append(warnings, w.Error())
	}

	if len(result.Errors) == 0 {
		return warnings, nil
	}

	msgs := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		msgs[i] = e.Error()
	}
	return warnings, fmt.Errorf("profile %q failed consistency checks:\n  %s",
		p.ID, strings.Join(msgs, "\n  "))
}

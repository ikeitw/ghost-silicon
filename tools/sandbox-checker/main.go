// tools/sandbox-checker/main.go
// Sandbox Checker — verifies that the Windows isolation primitives required
// by ghost-silicon are available on the current machine.
// Usage: sandbox-checker
package main

import (
	"fmt"
	"os"

	"ghost-silicon/pkg/security"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "sandbox-checker: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	checker := NewChecker()
	report := checker.Run()
	PrintReport(report)
	if !report.OK() {
		return fmt.Errorf("one or more checks failed")
	}
	return nil
}

// Checker runs a series of isolation environment checks.
type Checker struct {
	isolation *security.IsolationReport
}

// NewChecker creates a Checker and probes the current environment.
func NewChecker() *Checker {
	return &Checker{isolation: security.CheckIsolation()}
}

// Run executes all checks and returns a CheckReport.
func (c *Checker) Run() *CheckReport {
	r := &CheckReport{}

	r.Add("platform", c.isolation.Platform, true, "")
	r.Add("job_object", fmt.Sprintf("%v", c.isolation.JobObjectOK),
		c.isolation.JobObjectOK,
		"Job Objects are required for renderer process tree cleanup")
	r.Add("restricted_token", fmt.Sprintf("%v", c.isolation.RestrictedToken),
		c.isolation.RestrictedToken,
		"Restricted tokens are required for privilege reduction")

	for _, w := range c.isolation.Warnings {
		r.Warnings = append(r.Warnings, w)
	}

	return r
}

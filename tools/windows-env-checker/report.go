// tools/windows-env-checker/report.go
// Report types and printer for the Windows environment checker.
package main

import (
	"fmt"
	"strings"
)

// EnvCheck is one environment check result.
type EnvCheck struct {
	Name    string
	Value   string
	Passed  bool
	Message string
}

// EnvReport collects all check results.
type EnvReport struct {
	Checks []EnvCheck
}

// Add appends a check result.
func (r *EnvReport) Add(c EnvCheck) { r.Checks = append(r.Checks, c) }

// AllPassed returns true when every check passed.
func (r *EnvReport) AllPassed() bool {
	for _, c := range r.Checks {
		if !c.Passed {
			return false
		}
	}
	return true
}

// PrintReport writes the report to stdout.
func PrintReport(r *EnvReport) {
	passed := 0
	for _, c := range r.Checks {
		if c.Passed {
			passed++
		}
	}

	fmt.Printf("\n%-25s  %-6s  %s\n", "Check", "Status", "Value")
	fmt.Println(strings.Repeat("─", 70))

	for _, c := range r.Checks {
		status := "PASS"
		if !c.Passed {
			status = "FAIL"
		}
		fmt.Printf("%-25s  %-6s  %s\n", c.Name, status, c.Value)
		if !c.Passed && c.Message != "" {
			fmt.Printf("%-25s         → %s\n", "", c.Message)
		}
	}

	fmt.Println()
	fmt.Printf("Result: %d/%d checks passed.\n", passed, len(r.Checks))
}

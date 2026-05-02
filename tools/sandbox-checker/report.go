// tools/sandbox-checker/report.go
// Report printer for the sandbox checker.
package main

import "fmt"

// PrintReport writes the check report to stdout.
func PrintReport(r *CheckReport) {
	fmt.Println("Ghost-Silicon Sandbox Checker")
	fmt.Println("─────────────────────────────")

	for _, item := range r.Items {
		status := "✓"
		if !item.Passed {
			status = "✗"
		}
		fmt.Printf("  %s  %-25s %s\n", status, item.Name, item.Value)
		if !item.Passed && item.Description != "" {
			fmt.Printf("        → %s\n", item.Description)
		}
	}

	if len(r.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, w := range r.Warnings {
			fmt.Printf("  ⚠  %s\n", w)
		}
	}

	fmt.Println()
	if r.OK() {
		fmt.Println("Result: All checks passed.")
	} else {
		fmt.Println("Result: Some checks failed — review the output above.")
	}
}

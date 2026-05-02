// tools/windows-env-checker/main.go
// Windows Environment Checker — verifies that the host machine meets all
// prerequisites for running ghost-silicon on Windows 11.
// Usage: windows-env-checker
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "windows-env-checker: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("Ghost-Silicon Windows Environment Checker")
	fmt.Println("==========================================")

	checks := NewChecks()
	report := checks.RunAll()
	PrintReport(report)

	if !report.AllPassed() {
		return fmt.Errorf("one or more environment checks failed")
	}
	return nil
}

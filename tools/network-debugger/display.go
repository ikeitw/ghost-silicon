// tools/network-debugger/display.go
// Package main — display helpers for the network debugger output.
package main

import (
	"fmt"
	"strings"
)

// PrintCapture formats a CapturedRequest to stdout.
func PrintCapture(c *CapturedRequest) {
	fmt.Printf("\n%s %s\n", c.Method, c.URL)
	fmt.Println(strings.Repeat("─", 60))

	if c.Error != nil {
		fmt.Printf("ERROR: %v\n", c.Error)
		fmt.Printf("Latency: %dms\n", c.Latency.Milliseconds())
		return
	}

	fmt.Printf("Status:  %s\n", c.Status)
	fmt.Printf("Latency: %dms\n", c.Latency.Milliseconds())

	if len(c.Headers) > 0 {
		fmt.Println("\nResponse Headers:")
		for k, vs := range c.Headers {
			for _, v := range vs {
				fmt.Printf("  %-30s %s\n", k+":", v)
			}
		}
	}

	if c.BodyPrefix != "" {
		fmt.Println("\nBody (first 512 bytes):")
		fmt.Println(strings.Repeat("─", 40))
		fmt.Println(c.BodyPrefix)
	}
}

// PrintSummaryTable prints a compact table of multiple captures.
func PrintSummaryTable(captures []*CapturedRequest) {
	fmt.Printf("\n%-6s  %-8s  %s\n", "Status", "Latency", "URL")
	fmt.Println(strings.Repeat("─", 70))
	for _, c := range captures {
		if c.Error != nil {
			fmt.Printf("%-6s  %-8s  %s\n", "ERROR",
				fmt.Sprintf("%dms", c.Latency.Milliseconds()), c.URL)
		} else {
			fmt.Printf("%-6d  %-8s  %s\n", c.StatusCode,
				fmt.Sprintf("%dms", c.Latency.Milliseconds()), c.URL)
		}
	}
}

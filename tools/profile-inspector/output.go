// tools/profile-inspector/output.go
// Output formatting helpers for the profile inspector.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"ghost-silicon/pkg/identity"
)

// exportJSON writes the profile as formatted JSON to stdout.
func exportJSON(p *identity.Profile) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}

// summaryTable prints a compact multi-profile summary table.
func summaryTable(profiles []*identity.Profile) {
	fmt.Printf("%-36s  %-20s  %5s  %7s  %s\n",
		"ID", "Name", "Cores", "RAM MB", "Platform")
	fmt.Println(repeatStr("-", 85))
	for _, p := range profiles {
		fmt.Printf("%-36s  %-20s  %5d  %7d  %s\n",
			p.ID, p.Name, p.Hardware.CPUCores, p.Hardware.RAMMb, p.Hardware.Platform)
	}
}

func repeatStr(s string, n int) string {
	out := make([]byte, n*len(s))
	for i := range out {
		out[i] = s[i%len(s)]
	}
	return string(out)
}

// examples/basic-profile/main.go
// Demonstrates creating, validating, and saving a ghost-silicon profile.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"ghost-silicon/pkg/identity"
)

func main() {
	// 1. Create a profile from the built-in Windows 11 desktop template.
	p := identity.Windows11DesktopTemplate()
	fmt.Printf("Created profile: %s (%s)\n", p.Name, p.ID)
	fmt.Printf("  CPU cores:  %d\n", p.Hardware.CPUCores)
	fmt.Printf("  RAM:        %d MB (deviceMemory: %.1f GB)\n",
		p.Hardware.RAMMb, p.Hardware.DeviceMemoryGB())
	fmt.Printf("  GPU vendor: %s\n", p.Hardware.GPUVendor)
	fmt.Printf("  Platform:   %s\n", p.Hardware.Platform)
	fmt.Printf("  User-Agent: %s\n\n", p.Browser.UserAgent)

	// 2. Validate the profile.
	result := identity.Validate(p)
	if !result.Valid() {
		fmt.Fprintf(os.Stderr, "validation failed:\n%s\n", result.Error())
		os.Exit(1)
	}
	fmt.Println("Validation: PASS")

	// 3. Run consistency checks.
	cr := identity.CheckConsistency(p)
	if len(cr.Warnings) > 0 {
		for _, w := range cr.Warnings {
			fmt.Printf("Warning: %s\n", w.Error())
		}
	}
	if !cr.Clean() {
		fmt.Fprintf(os.Stderr, "consistency check failed\n")
		os.Exit(1)
	}
	fmt.Println("Consistency: PASS")

	// 4. Save the profile to a JSON file.
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}
	path := p.ID + ".profile.json"
	if err := os.WriteFile(path, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nSaved profile to: %s\n", path)
}

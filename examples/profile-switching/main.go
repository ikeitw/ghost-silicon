// examples/profile-switching/main.go
// Demonstrates loading multiple profiles and switching between them
// using the RotationState.
package main

import (
	"fmt"
	"os"
	"time"

	"ghost-silicon/pkg/identity"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "profile-switching: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Load profiles from the bundled profiles directory.
	result, err := identity.ImportFromDirectory("../../profiles")
	if err != nil {
		return fmt.Errorf("import profiles: %w", err)
	}
	if len(result.Imported) == 0 {
		return fmt.Errorf("no profiles found in ../../profiles")
	}

	fmt.Printf("Loaded %d profiles\n\n", len(result.Imported))
	for _, p := range result.Imported {
		fmt.Printf("  • %-25s  CPU=%-2d  RAM=%-6d  TZ=%s\n",
			p.Name, p.Hardware.CPUCores, p.Hardware.RAMMb, p.Network.Timezone)
	}

	// 2. Set up a rotation policy that rotates on every simulated session.
	base := result.Imported[0]
	policy := identity.RotationPolicy{
		Trigger:        identity.RotationOnSession,
		TemplateSource: identity.TemplateWindows11Desktop,
	}

	state, err := identity.NewRotationState(base, policy)
	if err != nil {
		return fmt.Errorf("rotation state: %w", err)
	}

	// 3. Simulate 3 session starts and observe the profile changing.
	fmt.Printf("\nSimulating 3 session rotations:\n")
	for i := 1; i <= 3; i++ {
		next, rotated := state.NotifyNewSession()
		if rotated {
			fmt.Printf("  Session %d: rotated → profile %s (CPU=%d RAM=%d)\n",
				i, next.ID[:8], next.Hardware.CPUCores, next.Hardware.RAMMb)
		} else {
			fmt.Printf("  Session %d: no rotation\n", i)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 4. Show the currently active profile.
	current := state.Current()
	fmt.Printf("\nActive profile: %s\n", current.Name)
	fmt.Printf("  GPU: %s\n", current.Hardware.GPUVendor)
	fmt.Printf("  UA:  %s\n", current.Browser.UserAgent)

	return nil
}

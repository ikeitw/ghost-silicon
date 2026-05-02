// tools/profile-inspector/main.go
// Profile Inspector — reads and displays ghost-silicon profile JSON files.
// Usage: profile-inspector <path-to-profile.json>
//
//	profile-inspector --dir <profiles-dir>
//	profile-inspector --validate <path-to-profile.json>
package main

import (
	"flag"
	"fmt"
	"os"

	"ghost-silicon/pkg/identity"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "profile-inspector: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		dir      = flag.String("dir", "", "inspect all profiles in directory")
		validate = flag.Bool("validate", false, "run validation checks")
		check    = flag.Bool("consistency", false, "run consistency checks")
	)
	flag.Parse()

	if *dir != "" {
		return inspectDirectory(*dir, *validate, *check)
	}

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		return fmt.Errorf("provide a profile path or --dir")
	}

	for _, path := range args {
		if err := inspectFile(path, *validate, *check); err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
		}
	}
	return nil
}

func inspectDirectory(dir string, validate, check bool) error {
	result, err := identity.ImportFromDirectory(dir)
	if err != nil {
		return fmt.Errorf("read directory %q: %w", dir, err)
	}
	fmt.Printf("Directory: %s\n", dir)
	fmt.Printf("Loaded: %d  Skipped: %d\n\n", len(result.Imported), len(result.Skipped))
	for _, skip := range result.Skipped {
		fmt.Printf("  SKIP %s: %s\n", skip.Source, skip.Reason)
	}
	for _, p := range result.Imported {
		printProfile(p)
		if validate {
			runValidation(p)
		}
		if check {
			runConsistency(p)
		}
		fmt.Println()
	}
	return nil
}

func inspectFile(path string, validate, check bool) error {
	p, err := identity.ImportFromFile(path)
	if err != nil {
		return fmt.Errorf("load %q: %w", path, err)
	}
	printProfile(p)
	if validate {
		runValidation(p)
	}
	if check {
		runConsistency(p)
	}
	return nil
}

func printProfile(p *identity.Profile) {
	fmt.Printf("Profile: %s (%s)\n", p.Name, p.ID)
	fmt.Printf("  Schema:   v%d\n", p.SchemaVersion)
	fmt.Printf("  Tags:     %v\n", p.Tags)
	fmt.Printf("  CPU:      %d cores\n", p.Hardware.CPUCores)
	fmt.Printf("  RAM:      %d MB (deviceMemory: %.2f GB)\n",
		p.Hardware.RAMMb, p.Hardware.DeviceMemoryGB())
	fmt.Printf("  GPU:      %s\n", p.Hardware.GPUVendor)
	fmt.Printf("  Platform: %s\n", p.Hardware.Platform)
	fmt.Printf("  Screen:   %dx%d @ %.1fx DPI\n",
		p.Screen.Width, p.Screen.Height, p.Screen.DevicePixelRatio)
	fmt.Printf("  UA:       %s\n", p.Browser.UserAgent)
	fmt.Printf("  TZ:       %s\n", p.Network.Timezone)
	fmt.Printf("  Noise:    canvas=%d audio=%d webgl=%d font=%d\n",
		p.Noise.CanvasSeed, p.Noise.AudioSeed, p.Noise.WebGLSeed, p.Noise.FontSeed)
}

func runValidation(p *identity.Profile) {
	result := identity.Validate(p)
	if result.Valid() {
		fmt.Println("  Validation: PASS")
	} else {
		fmt.Printf("  Validation: FAIL (%d error(s))\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("    • %s\n", e.Error())
		}
	}
}

func runConsistency(p *identity.Profile) {
	cr := identity.CheckConsistency(p)
	if cr.Clean() && len(cr.Warnings) == 0 {
		fmt.Println("  Consistency: PASS")
		return
	}
	for _, w := range cr.Warnings {
		fmt.Printf("  Consistency WARN: %s\n", w.Error())
	}
	for _, e := range cr.Errors {
		fmt.Printf("  Consistency FAIL: %s\n", e.Error())
	}
}

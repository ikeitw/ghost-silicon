// examples/isolated-renderer-windows/main.go
// Demonstrates the Windows sandbox configuration — shows how the supervisor
// would wire a Job Object, restricted token, and session directory together
// before launching a renderer process.
package main

import (
	"fmt"
	"os"
	"runtime"

	"ghost-silicon/internal/policy/sandbox"
	"ghost-silicon/pkg/identity"
	sbx "ghost-silicon/pkg/sandbox"
	"ghost-silicon/pkg/security"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "isolated-renderer-windows: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Printf("Platform: %s/%s\n\n", runtime.GOOS, runtime.GOARCH)

	// 1. Check isolation primitives are available.
	report := security.CheckIsolation()
	fmt.Println("Isolation check:")
	fmt.Printf("  Job Objects:      %v\n", report.JobObjectOK)
	fmt.Printf("  Restricted token: %v\n", report.RestrictedToken)
	if len(report.Warnings) > 0 {
		for _, w := range report.Warnings {
			fmt.Printf("  Warning: %s\n", w)
		}
	}
	fmt.Println()

	// 2. Load the sandbox policy.
	policy := sandbox.DefaultPolicy()
	policy.MemoryLimitMB = 2048
	policy.CPURatePercent = 80
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("sandbox policy: %w", err)
	}
	fmt.Println("Sandbox policy:")
	fmt.Printf("  Job Object:       %v\n", policy.EnableJobObject)
	fmt.Printf("  Restricted token: %v\n", policy.EnableRestrictedToken)
	fmt.Printf("  Integrity level:  %s\n", policy.IntegrityLevel)
	limits := sandbox.FromPolicy(policy)
	fmt.Printf("  Limits:           %s\n\n", limits.Describe())

	// 3. Load the identity profile.
	profile := identity.Windows11DesktopTemplate()
	fmt.Printf("Profile: %s\n", profile.Name)
	fmt.Printf("  CPU:   %d cores\n", profile.Hardware.CPUCores)
	fmt.Printf("  RAM:   %d MB\n", profile.Hardware.RAMMb)
	fmt.Printf("  GPU:   %s\n\n", profile.Hardware.GPUVendor)

	// 4. Show sandbox options that would be passed to the launcher.
	opts := sbx.Options{
		Executable:     `C:\path\to\renderer.exe`,
		SessionID:      "example-session-001",
		ProfileID:      profile.ID,
		PipeName:       `\\.\pipe\ghost-silicon-bridge`,
		UserDataDir:    `C:\Users\user\AppData\Roaming\ghost-silicon\sessions\example-session-001`,
		MemoryLimitMB:  policy.MemoryLimitMB,
		CPURatePercent: policy.CPURatePercent,
	}

	fmt.Println("Renderer launch options:")
	fmt.Printf("  Executable:    %s\n", opts.Executable)
	fmt.Printf("  Session ID:    %s\n", opts.SessionID)
	fmt.Printf("  Profile ID:    %s\n", opts.ProfileID)
	fmt.Printf("  Pipe:          %s\n", opts.PipeName)
	fmt.Printf("  User data dir: %s\n", opts.UserDataDir)
	fmt.Printf("  Memory limit:  %d MB\n", opts.MemoryLimitMB)
	fmt.Printf("  CPU cap:       %d%%\n\n", opts.CPURatePercent)

	fmt.Println("In a full deployment, ghost-silicon would now:")
	fmt.Println("  1. Create a Windows Job Object")
	fmt.Println("  2. Build a restricted process token at medium integrity")
	fmt.Println("  3. Launch the renderer via CreateProcessAsUser")
	fmt.Println("  4. Assign the process to the Job Object")
	fmt.Println("  5. Start the named pipe bridge")
	fmt.Println("  6. Serve profile-backed responses to the renderer")

	return nil
}

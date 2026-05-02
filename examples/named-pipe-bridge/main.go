// examples/named-pipe-bridge/main.go
// Demonstrates the IPC bridge: starts a JSON-RPC server over a named pipe
// and handles a few profile requests using the mock adapter.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"ghost-silicon/internal/ipc/jsonrpc"
	"ghost-silicon/internal/telemetry/audit"
	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/bridge"
	"ghost-silicon/pkg/identity"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "named-pipe-bridge: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	log, err := logging.New(&logging.Options{Level: "debug", Format: "text"})
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}

	auditor := audit.New(log)

	// Load the default profile.
	p := identity.Windows11DesktopTemplate()
	fmt.Printf("Bridge profile: %s\n", p.Name)
	fmt.Printf("  CPU cores:  %d\n", p.Hardware.CPUCores)
	fmt.Printf("  GPU:        %s\n", p.Hardware.GPUVendor)
	fmt.Printf("  User-Agent: %s\n\n", p.Browser.UserAgent)

	// Build the JSON-RPC server and register the bridge.
	srv := jsonrpc.NewServer(log)
	br := bridge.New(p, "example-session", log, auditor)
	br.Register(srv)

	// Demonstrate calling handlers directly (without a real pipe connection).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	methods := []string{
		"hardware.getCPUCores",
		"hardware.getRAM",
		"hardware.getGPU",
		"navigator.getProfile",
		"screen.getProfile",
		"noise.getCanvasSeed",
		"storage.getPolicy",
		"session.getInfo",
	}

	fmt.Println("Calling bridge methods directly:")
	fmt.Println("─────────────────────────────────")
	for _, method := range methods {
		result, err := srv.Call(ctx, method, nil)
		if err != nil {
			fmt.Printf("  ✗ %-35s %v\n", method, err)
			continue
		}
		fmt.Printf("  ✓ %-35s %s\n", method, string(result))
	}

	fmt.Println("\nIn a real deployment the JSON-RPC server would listen on:")
	fmt.Println(`  \\.\pipe\ghost-silicon-bridge`)
	fmt.Println("and the renderer process would connect and call these methods.")

	return nil
}

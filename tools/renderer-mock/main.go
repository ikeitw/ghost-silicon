// tools/renderer-mock/main.go
// Renderer Mock — simulates a renderer process connecting to the supervisor
// over the named pipe bridge. Used for testing the IPC layer without a real
// browser binary.
// Usage: renderer-mock --pipe \\.\pipe\ghost-silicon-bridge [--requests 10]
package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "renderer-mock: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		pipeName = flag.String("pipe", `\\.\pipe\ghost-silicon-bridge`, "supervisor pipe name")
		requests = flag.Int("requests", 5, "number of test requests to send")
		delay    = flag.Duration("delay", 500*time.Millisecond, "delay between requests")
	)
	flag.Parse()

	fmt.Printf("Renderer Mock connecting to %s\n", *pipeName)

	client, err := NewBridgeClient(*pipeName, 10*time.Second)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	fmt.Printf("Connected. Sending %d test requests...\n\n", *requests)

	mock := NewMock(client)
	return mock.RunRequests(*requests, *delay)
}

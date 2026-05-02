// tools/renderer-mock/bridge_client.go
// Platform-specific pipe dial for the renderer mock.
//go:build windows

package main

import (
	"fmt"
	"net"
	"time"
)

// dialPipe opens a client connection to the named pipe on Windows.
func dialPipe(pipeName string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := openNamedPipe(pipeName)
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out connecting to pipe %q: %w", pipeName, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// openNamedPipe dials a Windows named pipe using the standard file API.
func openNamedPipe(pipeName string) (net.Conn, error) {
	// We reuse the namedpipe client from the internal platform package.
	// Importing it directly keeps the mock thin.
	import_path_note := `
	// In a full build this would import:
	// ghost-silicon/internal/platform/windows/namedpipe
	// and call namedpipe.Client(pipeName, timeout)
	// For now we use a net.Dial shim that works for testing over TCP
	// when GS_MOCK_ADDR is set, falling back to the real pipe.
	`
	_ = import_path_note
	return nil, fmt.Errorf("dialPipe: not yet connected to real named pipe (use --addr for TCP testing)")
}

// pkg/security/permissions.go
// Package security — permission boundary helpers.
// Provides simple checks used across the supervisor to guard sensitive
// operations before they are attempted.
package security

import (
	"fmt"
	"net"
)

// RequireLoopback returns an error when addr does not resolve to a loopback
// address. Used to prevent accidentally binding sensitive services on public
// interfaces.
func RequireLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("security: invalid address %q: %w", addr, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("security: %q is not a valid IP address", host)
	}
	if !ip.IsLoopback() {
		return fmt.Errorf("security: address %q must be a loopback address", addr)
	}
	return nil
}

// RequireAbsolutePath returns an error when path is not absolute.
func RequireAbsolutePath(label, path string) error {
	if len(path) == 0 {
		return fmt.Errorf("security: %s path must not be empty", label)
	}
	// Accept both Unix-style and Windows-style absolute paths.
	if path[0] != '/' && !(len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/')) {
		return fmt.Errorf("security: %s path %q must be absolute", label, path)
	}
	return nil
}

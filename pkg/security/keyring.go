// pkg/security/keyring.go
//go:build windows

// Package security — Windows keyring wrapper.
// On Windows the DPAPI store is used as the keyring backend.
// This file exposes a NewKeyring constructor that returns the right
// SecretStore for the current platform.
package security

import (
	"fmt"
	"os"
	"path/filepath"
)

// NewKeyring returns a platform-appropriate SecretStore.
// On Windows this is the DPAPI-backed store rooted at dataDir/secrets.
func NewKeyring(dataDir string) (SecretStore, error) {
	dir := filepath.Join(dataDir, "secrets")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("security/keyring: mkdir %q: %w", dir, err)
	}
	return NewDPAPIStore(dir)
}

// pkg/security/keyring_linux.go
//go:build linux

// Package security — Linux keyring wrapper.
// Falls back to the in-memory store for now; a future iteration can
// integrate with the libsecret / D-Bus Secret Service.
package security

import "fmt"

// NewKeyring returns a MemoryStore on Linux.
// Replace with a libsecret-backed implementation when persistent secret
// storage is required on Linux.
func NewKeyring(_ string) (SecretStore, error) {
	fmt.Println("security/keyring: using in-memory store on Linux (no persistent keyring)")
	return NewMemoryStore(), nil
}

// pkg/security/secrets.go
// Package security — secret management abstraction.
// Provides a platform-neutral interface for storing and retrieving secrets
// (encryption keys, tokens). Backed by DPAPI on Windows and the system
// keyring on Linux.
package security

import "fmt"

// SecretStore is the platform-neutral interface for secret storage.
type SecretStore interface {
	// Set stores value under key.
	Set(key, value string) error

	// Get retrieves the value for key.
	// Returns ("", ErrSecretNotFound) when key does not exist.
	Get(key string) (string, error)

	// Delete removes the secret for key.
	Delete(key string) error
}

// ErrSecretNotFound is returned when a requested secret key does not exist.
var ErrSecretNotFound = fmt.Errorf("security: secret not found")

// MemoryStore is an in-process secret store used in tests and when no
// platform keyring is available.
type MemoryStore struct {
	data map[string]string
}

// NewMemoryStore creates an empty in-memory secret store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{data: make(map[string]string)}
}

// Set stores value under key.
func (m *MemoryStore) Set(key, value string) error {
	m.data[key] = value
	return nil
}

// Get retrieves the value for key.
func (m *MemoryStore) Get(key string) (string, error) {
	v, ok := m.data[key]
	if !ok {
		return "", ErrSecretNotFound
	}
	return v, nil
}

// Delete removes the secret for key.
func (m *MemoryStore) Delete(key string) error {
	delete(m.data, key)
	return nil
}

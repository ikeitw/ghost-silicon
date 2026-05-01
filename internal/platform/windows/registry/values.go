// internal/platform/windows/registry/values.go
//go:build windows

// Package registry — value read helpers.
package registry

import "fmt"

// GetString reads a string value from the key.
// Returns ("", nil) when the value does not exist.
func (k *Key) GetString(name string) (string, error) {
	v, _, err := k.handle.GetStringValue(name)
	if err != nil {
		// Treat missing value as empty, not an error.
		return "", nil
	}
	return v, nil
}

// GetUint64 reads a DWORD or QWORD value from the key.
// Returns (0, nil) when the value does not exist.
func (k *Key) GetUint64(name string) (uint64, error) {
	v, _, err := k.handle.GetIntegerValue(name)
	if err != nil {
		return 0, nil
	}
	return v, nil
}

// MustGetString reads a string value and returns an error when it is missing
// or empty.
func (k *Key) MustGetString(name string) (string, error) {
	v, err := k.GetString(name)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", fmt.Errorf("registry: value %q\\%q is empty or missing", k.path, name)
	}
	return v, nil
}

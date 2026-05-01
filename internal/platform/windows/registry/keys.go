// internal/platform/windows/registry/keys.go
//go:build windows

// Package registry — sub-key enumeration helpers.
package registry

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// SubKeyNames returns the names of all sub-keys under k.
func (k *Key) SubKeyNames() ([]string, error) {
	names, err := k.handle.ReadSubKeyNames(-1)
	if err != nil {
		return nil, fmt.Errorf("registry: read subkeys of %q: %w", k.path, err)
	}
	return names, nil
}

// OpenSubKey opens a child key relative to k for reading.
func (k *Key) OpenSubKey(name string) (*Key, error) {
	sub, err := registry.OpenKey(k.handle, name, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("registry: open subkey %q\\%q: %w", k.path, name, err)
	}
	return &Key{handle: sub, path: k.path + `\` + name}, nil
}

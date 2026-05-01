// internal/platform/windows/registry/registry.go
//go:build windows

// Package registry provides helpers for reading Windows registry values
// used by the supervisor to detect system configuration (proxy, DNS, etc.).
package registry

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// Root is a Windows registry hive identifier.
type Root uint32

const (
	RootLocalMachine Root = Root(registry.LOCAL_MACHINE)
	RootCurrentUser  Root = Root(registry.CURRENT_USER)
	RootClassesRoot  Root = Root(registry.CLASSES_ROOT)
)

// Key wraps a registry.Key handle with a path for error messages.
type Key struct {
	handle registry.Key
	path   string
}

// OpenKey opens a registry key for reading.
// Call Close() on the returned Key when done.
func OpenKey(root Root, path string) (*Key, error) {
	k, err := registry.OpenKey(registry.Key(root), path, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("registry: open %q: %w", path, err)
	}
	return &Key{handle: k, path: path}, nil
}

// Close releases the registry key handle.
func (k *Key) Close() error { return k.handle.Close() }

// Path returns the registry path this key was opened at.
func (k *Key) Path() string { return k.path }

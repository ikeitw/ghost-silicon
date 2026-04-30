// pkg/storage/linux_paths.go
//go:build linux

// Package storage — Linux path resolution.
// Returns XDG-compliant paths for all ghost-silicon storage roots on Linux.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// LinuxPaths resolves ghost-silicon storage directories on Linux.
// Follows the XDG Base Directory specification.
type LinuxPaths struct {
	Base string
}

// NewLinuxPaths creates a LinuxPaths resolver.
// If base is empty, $XDG_DATA_HOME/ghost-silicon or ~/.local/share/ghost-silicon is used.
func NewLinuxPaths(base string) (*LinuxPaths, error) {
	if base == "" {
		xdg := os.Getenv("XDG_DATA_HOME")
		if xdg == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("storage/linux_paths: resolve home dir: %w", err)
			}
			xdg = filepath.Join(home, ".local", "share")
		}
		base = filepath.Join(xdg, "ghost-silicon")
	}
	return &LinuxPaths{Base: base}, nil
}

// ProfilesDir returns the path for profile JSON files.
func (p *LinuxPaths) ProfilesDir() string { return filepath.Join(p.Base, "profiles") }

// SessionsDir returns the base path for per-session renderer directories.
func (p *LinuxPaths) SessionsDir() string { return filepath.Join(p.Base, "sessions") }

// LogsDir returns the path for supervisor log files.
func (p *LinuxPaths) LogsDir() string { return filepath.Join(p.Base, "logs") }

// CacheDir returns the HTTP cache path.
func (p *LinuxPaths) CacheDir() string { return filepath.Join(p.Base, "cache") }

// EnsureAll creates all storage directories if they do not already exist.
func (p *LinuxPaths) EnsureAll() error {
	for _, d := range []string{p.ProfilesDir(), p.SessionsDir(), p.LogsDir(), p.CacheDir()} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return fmt.Errorf("storage/linux_paths: create %q: %w", d, err)
		}
	}
	return nil
}

// pkg/storage/windows_paths.go
//go:build windows

// Package storage — Windows path resolution.
// Returns the correct AppData-based paths for all ghost-silicon storage roots.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// WindowsPaths resolves ghost-silicon storage directories on Windows.
// Base is %APPDATA%\ghost-silicon unless overridden.
type WindowsPaths struct {
	Base string
}

// NewWindowsPaths creates a WindowsPaths resolver.
// If base is empty, %APPDATA%\ghost-silicon is used.
func NewWindowsPaths(base string) (*WindowsPaths, error) {
	if base == "" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return nil, fmt.Errorf("storage/windows_paths: APPDATA env var not set")
		}
		base = filepath.Join(appData, "ghost-silicon")
	}
	return &WindowsPaths{Base: base}, nil
}

// ProfilesDir returns the path for profile JSON files.
func (p *WindowsPaths) ProfilesDir() string {
	return filepath.Join(p.Base, "profiles")
}

// SessionsDir returns the base path for per-session renderer directories.
func (p *WindowsPaths) SessionsDir() string {
	return filepath.Join(p.Base, "sessions")
}

// LogsDir returns the path for supervisor log files.
func (p *WindowsPaths) LogsDir() string {
	return filepath.Join(p.Base, "logs")
}

// CacheDir returns the HTTP cache path.
func (p *WindowsPaths) CacheDir() string {
	return filepath.Join(p.Base, "cache")
}

// EnsureAll creates all storage directories if they do not already exist.
func (p *WindowsPaths) EnsureAll() error {
	dirs := []string{
		p.ProfilesDir(),
		p.SessionsDir(),
		p.LogsDir(),
		p.CacheDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return fmt.Errorf("storage/windows_paths: create %q: %w", d, err)
		}
	}
	return nil
}

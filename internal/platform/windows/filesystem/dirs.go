//go:build windows

// Package filesystem creates and manages per-session isolated directory trees
// under the Windows AppData folder.  Each session gets its own root so that
// profiles cannot leak cookies, cache, or local storage between sessions.
package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
)

// SessionLayout describes the directory structure for one renderer session.
type SessionLayout struct {
	Root       string // sessions/<session-id>/
	Cache      string // sessions/<session-id>/cache/
	Cookies    string // sessions/<session-id>/cookies/
	LocalData  string // sessions/<session-id>/local_data/
	Downloads  string // sessions/<session-id>/downloads/
	Extensions string // sessions/<session-id>/extensions/
	Logs       string // sessions/<session-id>/logs/
}

// CreateSessionLayout creates the full directory tree for sessionID under
// baseDir and returns the layout struct.
// baseDir is typically cfg.Storage.BaseDir or cfg.App.DataDir/sessions.
func CreateSessionLayout(baseDir, sessionID string) (*SessionLayout, error) {
	root := filepath.Join(baseDir, sanitiseID(sessionID))
	dirs := []string{
		root,
		filepath.Join(root, "cache"),
		filepath.Join(root, "cookies"),
		filepath.Join(root, "local_data"),
		filepath.Join(root, "downloads"),
		filepath.Join(root, "extensions"),
		filepath.Join(root, "logs"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, fmt.Errorf("filesystem: create %q: %w", d, err)
		}
	}

	return &SessionLayout{
		Root:       root,
		Cache:      filepath.Join(root, "cache"),
		Cookies:    filepath.Join(root, "cookies"),
		LocalData:  filepath.Join(root, "local_data"),
		Downloads:  filepath.Join(root, "downloads"),
		Extensions: filepath.Join(root, "extensions"),
		Logs:       filepath.Join(root, "logs"),
	}, nil
}

// RemoveSessionLayout deletes the entire session directory tree.
// Should be called when a session is stopped and data retention is disabled.
func RemoveSessionLayout(layout *SessionLayout) error {
	if layout == nil || layout.Root == "" {
		return nil
	}
	if err := os.RemoveAll(layout.Root); err != nil {
		return fmt.Errorf("filesystem: remove session %q: %w", layout.Root, err)
	}
	return nil
}

// sanitiseID strips characters that are not safe for directory names.
func sanitiseID(id string) string {
	safe := make([]byte, 0, len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' {
			safe = append(safe, c)
		} else {
			safe = append(safe, '_')
		}
	}
	return string(safe)
}

// pkg/storage/session_store.go
// Package storage — session store.
// Tracks active and past sessions by persisting lightweight session metadata
// alongside the renderer's isolated data directory.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionMeta is the lightweight record written for each session.
type SessionMeta struct {
	ID          string    `json:"id"`
	ProfileID   string    `json:"profile_id"`
	StartedAt   time.Time `json:"started_at"`
	StoppedAt   time.Time `json:"stopped_at,omitempty"`
	UserDataDir string    `json:"user_data_dir"`
	State       string    `json:"state"` // "active" | "stopped" | "crashed"
}

// SessionStore persists session metadata under baseDir.
// One JSON file per session: <baseDir>/<session-id>/session.json
type SessionStore struct {
	mu      sync.RWMutex
	baseDir string
}

// NewSessionStore creates a SessionStore rooted at baseDir.
func NewSessionStore(baseDir string) (*SessionStore, error) {
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, fmt.Errorf("storage/session_store: mkdir %q: %w", baseDir, err)
	}
	return &SessionStore{baseDir: baseDir}, nil
}

// Save writes or overwrites the session metadata record.
func (s *SessionStore) Save(m *SessionMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Join(s.baseDir, sanitise(m.ID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("session_store: mkdir: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("session_store: marshal: %w", err)
	}

	path := filepath.Join(dir, "session.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("session_store: write: %w", err)
	}
	return os.Rename(tmp, path)
}

// Load reads the metadata for sessionID.
func (s *SessionStore) Load(sessionID string) (*SessionMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.baseDir, sanitise(sessionID), "session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("session_store: session %q not found", sessionID)
		}
		return nil, fmt.Errorf("session_store: read: %w", err)
	}

	var m SessionMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("session_store: unmarshal: %w", err)
	}
	return &m, nil
}

// List returns metadata for all stored sessions.
func (s *SessionStore) List() ([]*SessionMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("session_store: read dir: %w", err)
	}

	var out []*SessionMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(s.baseDir, e.Name(), "session.json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var m SessionMeta
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		out = append(out, &m)
	}
	return out, nil
}

// sanitise strips unsafe characters from an ID used as a directory name.
func sanitise(id string) string {
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

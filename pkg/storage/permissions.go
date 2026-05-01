// pkg/storage/permissions.go
// Package storage — persisted permission decisions.
// Stores the user's per-origin permission choices so they survive across
// sessions when the profile allows persistent cookies/storage.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// PermissionRecord stores a resolved permission state for one origin.
type PermissionRecord struct {
	Origin     string `json:"origin"`
	Permission string `json:"permission"`
	State      string `json:"state"` // "deny" | "prompt" | "grant"
}

// PermissionStore persists permission decisions in a JSON file.
type PermissionStore struct {
	mu      sync.RWMutex
	path    string
	records map[string]map[string]string // origin → permission → state
}

// NewPermissionStore creates a PermissionStore backed by path.
func NewPermissionStore(path string) (*PermissionStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("storage/permissions: mkdir: %w", err)
	}
	s := &PermissionStore{
		path:    path,
		records: make(map[string]map[string]string),
	}
	if err := s.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return s, nil
}

// Set stores the state for origin+permission and flushes to disk.
func (s *PermissionStore) Set(origin, permission, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records[origin] == nil {
		s.records[origin] = make(map[string]string)
	}
	s.records[origin][permission] = state
	return s.flush()
}

// Get returns the stored state for origin+permission, or "" if not set.
func (s *PermissionStore) Get(origin, permission string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if m, ok := s.records[origin]; ok {
		return m[permission]
	}
	return ""
}

// Clear removes all stored permission decisions and flushes to disk.
func (s *PermissionStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make(map[string]map[string]string)
	return s.flush()
}

func (s *PermissionStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.records)
}

func (s *PermissionStore) flush() error {
	data, err := json.MarshalIndent(s.records, "", "  ")
	if err != nil {
		return fmt.Errorf("storage/permissions: marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("storage/permissions: write: %w", err)
	}
	return os.Rename(tmp, s.path)
}

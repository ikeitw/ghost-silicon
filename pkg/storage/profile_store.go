// pkg/storage/profile_store.go
// Package storage — profile store wrapper.
// Re-exports identity.FileStore with path resolution so callers
// only need to import pkg/storage.
package storage

import (
	"fmt"
	"os"

	"ghost-silicon/pkg/identity"
)

// ProfileStore wraps identity.FileStore with a resolved directory path.
type ProfileStore struct {
	*identity.FileStore
	dir string
}

// NewProfileStore creates a ProfileStore rooted at dir.
// The directory is created if it does not exist.
func NewProfileStore(dir string) (*ProfileStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("storage/profile_store: mkdir %q: %w", dir, err)
	}
	fs, err := identity.NewFileStore(dir)
	if err != nil {
		return nil, fmt.Errorf("storage/profile_store: %w", err)
	}
	return &ProfileStore{FileStore: fs, dir: dir}, nil
}

// Dir returns the directory this store is rooted at.
func (s *ProfileStore) Dir() string { return s.dir }

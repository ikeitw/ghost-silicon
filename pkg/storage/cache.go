// pkg/storage/cache.go
// Package storage — HTTP cache storage helpers.
// Manages the per-session cache directory lifecycle.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// CacheStore manages a renderer's HTTP cache directory.
type CacheStore struct {
	dir string
}

// NewCacheStore creates a CacheStore for the given directory.
// The directory is created if it does not exist.
func NewCacheStore(dir string) (*CacheStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("storage/cache: mkdir %q: %w", dir, err)
	}
	return &CacheStore{dir: dir}, nil
}

// Dir returns the cache directory path.
func (c *CacheStore) Dir() string { return c.dir }

// Clear removes all files inside the cache directory without removing
// the directory itself, so the renderer can continue using it.
func (c *CacheStore) Clear() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return fmt.Errorf("storage/cache: read dir: %w", err)
	}
	for _, e := range entries {
		target := filepath.Join(c.dir, e.Name())
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("storage/cache: remove %q: %w", target, err)
		}
	}
	return nil
}

// SizeMB returns the total cache size in megabytes.
func (c *CacheStore) SizeMB() (int64, error) {
	var total int64
	err := filepath.Walk(c.dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total / (1024 * 1024), err
}

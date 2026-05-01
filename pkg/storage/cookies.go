// pkg/storage/cookies.go
// Package storage — cookie storage helpers.
// Manages the per-session cookie directory and provides a size cap check.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// CookieStore manages the renderer's cookie storage directory.
type CookieStore struct {
	dir       string
	maxSizeMB int
}

// NewCookieStore creates a CookieStore rooted at dir.
// maxSizeMB == 0 means no limit.
func NewCookieStore(dir string, maxSizeMB int) (*CookieStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("storage/cookies: mkdir %q: %w", dir, err)
	}
	return &CookieStore{dir: dir, maxSizeMB: maxSizeMB}, nil
}

// Dir returns the cookie directory path.
func (c *CookieStore) Dir() string { return c.dir }

// CheckQuota returns an error when the cookie directory exceeds the cap.
func (c *CookieStore) CheckQuota() error {
	if c.maxSizeMB <= 0 {
		return nil
	}
	var total int64
	_ = filepath.Walk(c.dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	usedMB := total / (1024 * 1024)
	if usedMB > int64(c.maxSizeMB) {
		return fmt.Errorf("storage/cookies: quota exceeded (%d MB > %d MB)", usedMB, c.maxSizeMB)
	}
	return nil
}

// Clear removes all files in the cookie directory.
func (c *CookieStore) Clear() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return fmt.Errorf("storage/cookies: read dir: %w", err)
	}
	for _, e := range entries {
		target := filepath.Join(c.dir, e.Name())
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("storage/cookies: remove %q: %w", target, err)
		}
	}
	return nil
}

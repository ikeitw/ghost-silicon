// internal/policy/storage/quota.go
// Package storage — quota enforcement helpers.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// QuotaChecker checks whether a session directory is within its storage quota.
type QuotaChecker struct {
	maxMB int
}

// NewQuotaChecker creates a checker with a cap of maxMB megabytes.
// maxMB == 0 means no limit.
func NewQuotaChecker(maxMB int) *QuotaChecker {
	return &QuotaChecker{maxMB: maxMB}
}

// Check returns an error when the directory at path exceeds the quota.
func (q *QuotaChecker) Check(path string) error {
	if q.maxMB <= 0 {
		return nil
	}
	used, err := dirSizeMB(path)
	if err != nil {
		return fmt.Errorf("quota: measure %q: %w", path, err)
	}
	if used > int64(q.maxMB) {
		return fmt.Errorf("quota: %q uses %d MB, limit is %d MB", path, used, q.maxMB)
	}
	return nil
}

// dirSizeMB returns the total size of all files under dir in megabytes.
func dirSizeMB(dir string) (int64, error) {
	var total int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total / (1024 * 1024), err
}

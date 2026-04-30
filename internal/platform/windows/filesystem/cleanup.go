//go:build windows

package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CleanStaleSessions removes session directories in baseDir that are older
// than maxAge.  A maxAge of 0 means keep all sessions.
// This is called at supervisor startup to reclaim disk space.
func CleanStaleSessions(baseDir string, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, nil
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("filesystem/cleanup: read %q: %w", baseDir, err)
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			full := filepath.Join(baseDir, e.Name())
			if err := os.RemoveAll(full); err != nil {
				// Log and continue — don't abort the whole cleanup.
				fmt.Fprintf(os.Stderr, "filesystem/cleanup: remove %q: %v\n", full, err)
				continue
			}
			removed++
		}
	}
	return removed, nil
}

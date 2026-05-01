// pkg/storage/migrations.go
// Package storage — storage schema migrations.
// Runs any pending migrations on the data directory when the supervisor
// starts. Each migration is identified by a monotonically increasing version
// number stored in a version file at <dataDir>/.storage_version.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const currentStorageVersion = 1

// Migrate runs all pending storage migrations for dataDir.
// It is safe to call on every startup — already-applied migrations are skipped.
func Migrate(dataDir string) error {
	version, err := readVersion(dataDir)
	if err != nil {
		return fmt.Errorf("storage/migrate: read version: %w", err)
	}

	for v := version + 1; v <= currentStorageVersion; v++ {
		if err := applyMigration(dataDir, v); err != nil {
			return fmt.Errorf("storage/migrate: apply v%d: %w", v, err)
		}
		if err := writeVersion(dataDir, v); err != nil {
			return fmt.Errorf("storage/migrate: write version v%d: %w", v, err)
		}
	}
	return nil
}

// applyMigration runs the migration for the given version number.
func applyMigration(dataDir string, version int) error {
	switch version {
	case 1:
		// v1: ensure all standard subdirectories exist.
		dirs := []string{"profiles", "sessions", "logs", "cache"}
		for _, d := range dirs {
			if err := os.MkdirAll(filepath.Join(dataDir, d), 0o700); err != nil {
				return fmt.Errorf("mkdir %s: %w", d, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown migration version %d", version)
	}
}

func readVersion(dataDir string) (int, error) {
	path := filepath.Join(dataDir, ".storage_version")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse version file: %w", err)
	}
	return v, nil
}

func writeVersion(dataDir string, version int) error {
	path := filepath.Join(dataDir, ".storage_version")
	return os.WriteFile(path, []byte(strconv.Itoa(version)+"\n"), 0o600)
}

package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ImportResult is returned by bulk import operations.
type ImportResult struct {
	Imported []*Profile
	Skipped  []ImportSkip
}

// ImportSkip describes a single file that was not imported and why.
type ImportSkip struct {
	Source string
	Reason string
}

// ImportFromBytes decodes and migrates a single profile from raw JSON bytes.
// The caller receives a fully migrated *Profile at CurrentSchemaVersion.
func ImportFromBytes(data []byte) (*Profile, error) {
	p, err := Migrate(data)
	if err != nil {
		return nil, fmt.Errorf("identity/import: %w", err)
	}
	return p, nil
}

// ImportFromFile reads a .profile.json file and returns the decoded profile.
func ImportFromFile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("identity/import: read %q: %w", path, err)
	}
	p, err := ImportFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("identity/import: parse %q: %w", path, err)
	}
	return p, nil
}

// ImportFromDirectory reads all *.profile.json files in dir (non-recursive).
// Files that fail to parse are recorded in ImportResult.Skipped rather than
// causing the whole import to fail.
func ImportFromDirectory(dir string) (*ImportResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("identity/import: read directory %q: %w", dir, err)
	}

	result := &ImportResult{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".profile.json") {
			continue
		}

		fullPath := filepath.Join(dir, e.Name())
		p, err := ImportFromFile(fullPath)
		if err != nil {
			result.Skipped = append(result.Skipped, ImportSkip{
				Source: fullPath,
				Reason: err.Error(),
			})
			continue
		}
		result.Imported = append(result.Imported, p)
	}
	return result, nil
}

// ImportFromMap decodes a profile from a map[string]any (e.g. from a
// parsed YAML or TOML document). The map is round-tripped through JSON.
func ImportFromMap(m map[string]any) (*Profile, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("identity/import: marshal map: %w", err)
	}
	return ImportFromBytes(data)
}

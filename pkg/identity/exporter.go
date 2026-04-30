package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ExportOptions configures how a profile is serialised.
type ExportOptions struct {
	// Pretty enables indented JSON output (default: true).
	Pretty bool
}

var defaultExportOptions = ExportOptions{Pretty: true}

// ExportToBytes serialises a profile to JSON bytes.
// If opts is nil, pretty-printing is enabled.
func ExportToBytes(p *Profile, opts *ExportOptions) ([]byte, error) {
	if opts == nil {
		opts = &defaultExportOptions
	}
	var (
		data []byte
		err  error
	)
	if opts.Pretty {
		data, err = json.MarshalIndent(p, "", "  ")
	} else {
		data, err = json.Marshal(p)
	}
	if err != nil {
		return nil, fmt.Errorf("identity/export: marshal profile %q: %w", p.ID, err)
	}
	return data, nil
}

// ExportToFile writes a profile to path as JSON.
// Parent directories are created automatically.
// The file is written atomically: temp-file → rename.
func ExportToFile(p *Profile, path string, opts *ExportOptions) error {
	data, err := ExportToBytes(p, opts)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("identity/export: create directories for %q: %w", path, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("identity/export: write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("identity/export: atomic rename to %q: %w", path, err)
	}
	return nil
}

// ExportBatch writes multiple profiles to dir, one file per profile.
// File names follow the pattern <id>.profile.json.
func ExportBatch(profiles []*Profile, dir string, opts *ExportOptions) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("identity/export: create directory %q: %w", dir, err)
	}
	for _, p := range profiles {
		path := filepath.Join(dir, p.ID+".profile.json")
		if err := ExportToFile(p, path, opts); err != nil {
			return err
		}
	}
	return nil
}

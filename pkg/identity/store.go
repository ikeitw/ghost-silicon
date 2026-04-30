package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrProfileNotFound is returned when a requested profile ID does not exist.
var ErrProfileNotFound = errors.New("identity: profile not found")

// ErrProfileExists is returned when creating a profile with a duplicate ID.
var ErrProfileExists = errors.New("identity: profile already exists")

// Store is the interface for profile persistence.
type Store interface {
	// Save persists a profile. If a profile with the same ID exists it is
	// overwritten.
	Save(p *Profile) error

	// Load retrieves a profile by ID.
	Load(id string) (*Profile, error)

	// Delete removes a profile by ID. Returns ErrProfileNotFound if missing.
	Delete(id string) error

	// List returns metadata for all stored profiles (ID, Name, UpdatedAt).
	// The full profile data is not loaded.
	List() ([]*ProfileMeta, error)

	// Exists reports whether a profile with the given ID is stored.
	Exists(id string) bool
}

// ProfileMeta holds lightweight metadata without the full profile payload.
type ProfileMeta struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Tags      []string `json:"tags,omitempty"`
}

// FileStore implements Store using a directory of JSON files.
// One file per profile; file names are <id>.profile.json.
// All public methods are safe for concurrent use.
type FileStore struct {
	dir string
	mu  sync.RWMutex
}

// NewFileStore creates a FileStore rooted at dir.
// The directory is created if it does not exist.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("identity/store: create directory %q: %w", dir, err)
	}
	return &FileStore{dir: dir}, nil
}

// Save writes the profile as JSON to <dir>/<id>.profile.json.
// Writes are atomic: data is flushed to a temp file then renamed.
func (s *FileStore) Save(p *Profile) error {
	if err := validateForStore(p); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("identity/store: marshal profile %q: %w", p.ID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dest := s.filePath(p.ID)
	tmp := dest + ".tmp"

	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("identity/store: write temp file: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("identity/store: atomic rename: %w", err)
	}
	return nil
}

// Load reads and migrates a profile by ID.
func (s *FileStore) Load(id string) (*Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.filePath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("identity/store: read profile %q: %w", id, err)
	}

	p, err := Migrate(data)
	if err != nil {
		return nil, fmt.Errorf("identity/store: migrate profile %q: %w", id, err)
	}
	return p, nil
}

// Delete removes the profile file. Returns ErrProfileNotFound if missing.
func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.filePath(id))
	if errors.Is(err, os.ErrNotExist) {
		return ErrProfileNotFound
	}
	return err
}

// List returns lightweight metadata for all stored profiles.
func (s *FileStore) List() ([]*ProfileMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("identity/store: read directory: %w", err)
	}

	var out []*ProfileMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".profile.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue // skip unreadable files
		}

		// Decode only the metadata fields to keep List() fast.
		var m struct {
			ID        string   `json:"id"`
			Name      string   `json:"name"`
			CreatedAt string   `json:"created_at"`
			UpdatedAt string   `json:"updated_at"`
			Tags      []string `json:"tags,omitempty"`
		}
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		out = append(out, &ProfileMeta{
			ID:        m.ID,
			Name:      m.Name,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
			Tags:      m.Tags,
		})
	}
	return out, nil
}

// Exists reports whether the given profile ID has a stored file.
func (s *FileStore) Exists(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, err := os.Stat(s.filePath(id))
	return err == nil
}

// filePath returns the full path for a profile file. Must be called with mu held.
func (s *FileStore) filePath(id string) string {
	// Sanitise ID to prevent path traversal.
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, id)
	return filepath.Join(s.dir, safe+".profile.json")
}

func validateForStore(p *Profile) error {
	if p == nil {
		return errors.New("identity/store: profile must not be nil")
	}
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("identity/store: profile ID must not be empty")
	}
	return nil
}

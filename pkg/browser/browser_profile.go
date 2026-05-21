//go:build windows

// Package browser — named browser profile store.
// A BrowserProfile is a named account that persists browsing state (tabs,
// history, bookmarks, WebView2 session data) across restarts.
// It is distinct from identity.Profile, which controls fingerprint spoofing.
package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"ghost-silicon/pkg/identity"
)

// BrowserProfile is the metadata stored at <profile-dir>/profile.json.
type BrowserProfile struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Created         time.Time `json:"created"`
	LastUsed        time.Time `json:"last_used"`
	SearchEngineURL string    `json:"search_engine_url"`
}

// BrowserProfileStore manages named browser profiles under baseDir.
// Each profile gets its own sub-directory used as the WebView2 DataPath,
// so cookies, localStorage, history, bookmarks, and tabs are fully isolated.
type BrowserProfileStore struct {
	mu      sync.Mutex
	baseDir string
}

// NewBrowserProfileStore opens (or creates) the profiles directory at baseDir.
func NewBrowserProfileStore(baseDir string) (*BrowserProfileStore, error) {
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, err
	}
	return &BrowserProfileStore{baseDir: baseDir}, nil
}

// List returns all profiles, most recently used first.
func (s *BrowserProfileStore) List() ([]*BrowserProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}
	var profiles []*BrowserProfile
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var p BrowserProfile
		if err := readJSON(filepath.Join(s.baseDir, e.Name(), "profile.json"), &p); err == nil {
			profiles = append(profiles, &p)
		}
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].LastUsed.After(profiles[j].LastUsed)
	})
	return profiles, nil
}

func (s *BrowserProfileStore) Create(name, searchEngineURL string) (*BrowserProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := &BrowserProfile{
		ID:              uuid.New().String(),
		Name:            name,
		Created:         time.Now().UTC(),
		LastUsed:        time.Now().UTC(),
		SearchEngineURL: searchEngineURL,
	}
	dir := filepath.Join(s.baseDir, p.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return p, writeJSON(filepath.Join(dir, "profile.json"), p)
}

func (s *BrowserProfileStore) Load(id string) (*BrowserProfile, error) {
	var p BrowserProfile
	if err := readJSON(filepath.Join(s.baseDir, id, "profile.json"), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Touch updates LastUsed (and optionally SearchEngineURL) for id.
func (s *BrowserProfileStore) Touch(id, searchEngineURL string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.baseDir, id, "profile.json")
	var p BrowserProfile
	if readJSON(path, &p) != nil {
		return
	}
	p.LastUsed = time.Now().UTC()
	if searchEngineURL != "" {
		p.SearchEngineURL = searchEngineURL
	}
	_ = writeJSON(path, &p)
}

// Delete removes a profile and all its data permanently.
func (s *BrowserProfileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.RemoveAll(filepath.Join(s.baseDir, id))
}

func (s *BrowserProfileStore) Rename(id, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.baseDir, id, "profile.json")
	var p BrowserProfile
	if err := readJSON(path, &p); err != nil {
		return err
	}
	p.Name = newName
	return writeJSON(path, &p)
}

// Dir returns the stable data directory for id (created on demand by Create).
func (s *BrowserProfileStore) Dir(id string) string {
	return filepath.Join(s.baseDir, id)
}

// EnsureDir creates the profile directory if it does not exist.
func (s *BrowserProfileStore) EnsureDir(id string) error {
	return os.MkdirAll(filepath.Join(s.baseDir, id), 0o700)
}

// SaveIdentity writes the identity profile to <profile-dir>/identity.json.
func (s *BrowserProfileStore) SaveIdentity(id string, prof *identity.Profile) error {
	return writeJSON(filepath.Join(s.baseDir, id, "identity.json"), prof)
}

// LoadIdentity reads the identity profile from <profile-dir>/identity.json.
func (s *BrowserProfileStore) LoadIdentity(id string) (*identity.Profile, error) {
	var prof identity.Profile
	if err := readJSON(filepath.Join(s.baseDir, id, "identity.json"), &prof); err != nil {
		return nil, err
	}
	return &prof, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

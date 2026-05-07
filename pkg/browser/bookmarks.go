// pkg/browser/bookmarks.go
// BookmarkStore provides simple bookmark persistence backed by a JSON file.
// The UI (bookmark bar, manager dialog) lives in separate files; this package
// only owns the data model and I/O.
package browser

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Bookmark is a single saved page.
type Bookmark struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Favicon   string    `json:"favicon,omitempty"` // base64-encoded ICO or data-URL
	CreatedAt time.Time `json:"created_at"`
	FolderID  string    `json:"folder_id,omitempty"` // reserved for folder support
}

// BookmarkStore manages a list of bookmarks backed by a JSON file.
// All exported methods are safe for concurrent use.
type BookmarkStore struct {
	mu   sync.RWMutex
	path string
	list []*Bookmark
}

// NewBookmarkStore opens (or creates) the bookmark file at path.
func NewBookmarkStore(path string) (*BookmarkStore, error) {
	s := &BookmarkStore{path: path}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("bookmarks: load %q: %w", path, err)
	}
	return s, nil
}

// All returns a snapshot of every bookmark (ordered by insertion).
func (s *BookmarkStore) All() []*Bookmark {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Bookmark, len(s.list))
	copy(out, s.list)
	return out
}

// Add appends a new bookmark and persists.
func (s *BookmarkStore) Add(url, title string) (*Bookmark, error) {
	b := &Bookmark{
		ID:        bookmarkID(),
		URL:       url,
		Title:     title,
		CreatedAt: time.Now().UTC(),
	}
	s.mu.Lock()
	s.list = append(s.list, b)
	s.mu.Unlock()
	if err := s.flush(); err != nil {
		return b, fmt.Errorf("bookmarks: save after add: %w", err)
	}
	return b, nil
}

// Remove deletes the bookmark with the given ID and persists.
// A missing ID is silently ignored.
func (s *BookmarkStore) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, b := range s.list {
		if b.ID == id {
			s.list = append(s.list[:i], s.list[i+1:]...)
			return s.flushLocked()
		}
	}
	return nil
}

// UpdateTitle replaces the title on an existing bookmark.
func (s *BookmarkStore) UpdateTitle(id, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.list {
		if b.ID == id {
			b.Title = title
			return s.flushLocked()
		}
	}
	return nil
}

// Has reports whether url already has a bookmark.
func (s *BookmarkStore) Has(url string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.list {
		if b.URL == url {
			return true
		}
	}
	return false
}

// ForURL returns the first bookmark for url, or nil if none.
func (s *BookmarkStore) ForURL(url string) *Bookmark {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.list {
		if b.URL == url {
			return b
		}
	}
	return nil
}

// ── private ───────────────────────────────────────────────────────────────────

func (s *BookmarkStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.list)
}

func (s *BookmarkStore) flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

func (s *BookmarkStore) flushLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.list, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write via temp file + rename.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// bookmarkID returns a short random hex ID for a new bookmark.
func bookmarkID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

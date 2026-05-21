package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HistoryEntry is a single visited page.
type HistoryEntry struct {
	URL   string
	Title string
}

// NavigationHistory maintains back and forward stacks for a single tab.
// All exported methods are safe for concurrent use.
type NavigationHistory struct {
	mu      sync.Mutex
	back    []HistoryEntry
	current HistoryEntry
	forward []HistoryEntry
}

func NewNavigationHistory() *NavigationHistory {
	return &NavigationHistory{}
}

// Push records navigation to url/title, clearing the forward stack.
// If the current URL is the same as url the call is a no-op (avoids duplicates
// on reload).
func (h *NavigationHistory) Push(url, title string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.current.URL == url {
		// Same URL — just update the title.
		h.current.Title = title
		return
	}
	if h.current.URL != "" {
		h.back = append(h.back, h.current)
	}
	h.current = HistoryEntry{URL: url, Title: title}
	h.forward = h.forward[:0]
}

// Back moves one step back. Returns (entry, true) on success and
// (zero, false) when there is no history to go back to.
func (h *NavigationHistory) Back() (HistoryEntry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.back) == 0 {
		return HistoryEntry{}, false
	}
	h.forward = append([]HistoryEntry{h.current}, h.forward...)
	h.current = h.back[len(h.back)-1]
	h.back = h.back[:len(h.back)-1]
	return h.current, true
}

// Forward moves one step forward. Returns (entry, true) on success.
func (h *NavigationHistory) Forward() (HistoryEntry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.forward) == 0 {
		return HistoryEntry{}, false
	}
	h.back = append(h.back, h.current)
	h.current = h.forward[0]
	h.forward = h.forward[1:]
	return h.current, true
}

func (h *NavigationHistory) CanGoBack() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.back) > 0
}

func (h *NavigationHistory) CanGoForward() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.forward) > 0
}

func (h *NavigationHistory) Current() HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.current
}

// UpdateTitle replaces the title of the current entry (called on
// document-title-changed events from WebView2).
func (h *NavigationHistory) UpdateTitle(title string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.current.Title = title
}

// BackList returns a snapshot of the back stack, oldest entry first.
func (h *NavigationHistory) BackList() []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]HistoryEntry, len(h.back))
	copy(out, h.back)
	return out
}

// ForwardList returns a snapshot of the forward stack, next entry first.
func (h *NavigationHistory) ForwardList() []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]HistoryEntry, len(h.forward))
	copy(out, h.forward)
	return out
}

// ── BrowsingHistoryStore ──────────────────────────────────────────────────────

// BrowsingVisit is a single visited page in the global browsing history.
type BrowsingVisit struct {
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	VisitedAt time.Time `json:"visited_at"`
}

// BrowsingHistoryStore persists the global list of visited pages.
// All exported methods are safe for concurrent use.
type BrowsingHistoryStore struct {
	mu      sync.Mutex
	path    string
	entries []BrowsingVisit
}

// NewBrowsingHistoryStore opens (or creates) the history file at path.
// Pass an empty path for an in-memory-only store.
func NewBrowsingHistoryStore(path string) (*BrowsingHistoryStore, error) {
	s := &BrowsingHistoryStore{path: path}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

// Record appends a visit entry, capping at 10 000 entries, then flushes.
func (s *BrowsingHistoryStore) Record(url, title string) {
	if url == "" || strings.HasPrefix(url, "data:") || strings.HasPrefix(url, "about:") {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, BrowsingVisit{URL: url, Title: title, VisitedAt: time.Now().UTC()})
	if len(s.entries) > 10000 {
		s.entries = s.entries[len(s.entries)-10000:]
	}
	_ = s.flushLocked()
}

// All returns a snapshot of all visits, newest first.
func (s *BrowsingHistoryStore) All() []BrowsingVisit {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]BrowsingVisit, len(s.entries))
	for i, e := range s.entries {
		out[len(s.entries)-1-i] = e
	}
	return out
}

// DeleteByURL removes all entries matching url and flushes.
func (s *BrowsingHistoryStore) DeleteByURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keep := s.entries[:0]
	for _, e := range s.entries {
		if e.URL != url {
			keep = append(keep, e)
		}
	}
	s.entries = keep
	_ = s.flushLocked()
}

// Clear removes all history and flushes.
func (s *BrowsingHistoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	_ = s.flushLocked()
}

func (s *BrowsingHistoryStore) load() error {
	if s.path == "" {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.entries)
}

func (s *BrowsingHistoryStore) flushLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(s.entries)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// pkg/browser/history.go
// NavigationHistory tracks back/forward navigation for one browser tab.
// It is a pure Go structure with no UI dependency; each Tab owns one instance.
package browser

import "sync"

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

// NewNavigationHistory returns an empty history.
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

// CanGoBack reports whether the back stack is non-empty.
func (h *NavigationHistory) CanGoBack() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.back) > 0
}

// CanGoForward reports whether the forward stack is non-empty.
func (h *NavigationHistory) CanGoForward() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.forward) > 0
}

// Current returns the entry for the currently loaded page.
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

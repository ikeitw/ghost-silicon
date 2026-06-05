//go:build windows

package browser

import (
	"sync"
	"time"
)

const netLogCap = 300

// NetEntry records one HTTP request that passed through the blocker.
type NetEntry struct {
	TS      time.Time `json:"ts"`
	Host    string    `json:"host"`
	URL     string    `json:"url"`
	Blocked bool      `json:"blocked"`
}

// NetworkLog is a thread-safe fixed-capacity ring buffer of recent requests.
type NetworkLog struct {
	mu      sync.Mutex
	entries []NetEntry
}

func newNetworkLog() *NetworkLog {
	return &NetworkLog{entries: make([]NetEntry, 0, netLogCap)}
}

func (l *NetworkLog) record(host, url string, blocked bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := NetEntry{TS: time.Now(), Host: host, URL: url, Blocked: blocked}
	if len(l.entries) >= netLogCap {
		copy(l.entries, l.entries[1:])
		l.entries = l.entries[:netLogCap-1]
		l.entries = append(l.entries, e)
	} else {
		l.entries = append(l.entries, e)
	}
}

// snapshot returns all entries newest-first.
func (l *NetworkLog) snapshot() []NetEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]NetEntry, len(l.entries))
	for i, e := range l.entries {
		out[len(l.entries)-1-i] = e
	}
	return out
}

func (l *NetworkLog) clear() {
	l.mu.Lock()
	l.entries = l.entries[:0]
	l.mu.Unlock()
}

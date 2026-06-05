//go:build windows

package browser

import (
	"sync"
	"time"
)

const auditLogCap = 500

// AuditEntry records a browser-level event.
type AuditEntry struct {
	TS     time.Time `json:"ts"`
	Type   string    `json:"type"`
	Detail string    `json:"detail"`
}

// AuditLog is a thread-safe fixed-capacity ring buffer of browser events.
type AuditLog struct {
	mu      sync.Mutex
	entries []AuditEntry
}

func newAuditLog() *AuditLog {
	return &AuditLog{entries: make([]AuditEntry, 0, auditLogCap)}
}

func (l *AuditLog) record(eventType, detail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := AuditEntry{TS: time.Now(), Type: eventType, Detail: detail}
	if len(l.entries) >= auditLogCap {
		copy(l.entries, l.entries[1:])
		l.entries = l.entries[:auditLogCap-1]
		l.entries = append(l.entries, e)
	} else {
		l.entries = append(l.entries, e)
	}
}

// snapshot returns all entries newest-first.
func (l *AuditLog) snapshot() []AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]AuditEntry, len(l.entries))
	for i, e := range l.entries {
		out[len(l.entries)-1-i] = e
	}
	return out
}

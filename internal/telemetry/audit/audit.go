// Package audit provides an append-only, structured audit trail for
// security-relevant events: profile loads, permission grants/denies,
// renderer lifecycle changes, and network policy decisions.
package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// EventType classifies an audit event.
type EventType string

const (
	EventProfileLoaded    EventType = "profile.loaded"
	EventProfileSaved     EventType = "profile.saved"
	EventProfileDeleted   EventType = "profile.deleted"
	EventProfileRotated   EventType = "profile.rotated"
	EventSessionStarted   EventType = "session.started"
	EventSessionStopped   EventType = "session.stopped"
	EventRendererStarted  EventType = "renderer.started"
	EventRendererStopped  EventType = "renderer.stopped"
	EventRendererCrashed  EventType = "renderer.crashed"
	EventPermissionGrant  EventType = "permission.granted"
	EventPermissionDeny   EventType = "permission.denied"
	EventNetworkAllowed   EventType = "network.allowed"
	EventNetworkBlocked   EventType = "network.blocked"
	EventBridgeRequest    EventType = "bridge.request"
	EventSandboxViolation EventType = "sandbox.violation"
	EventConfigLoaded     EventType = "config.loaded"
)

// Event is a single audit record. Every field is JSON-serialised so that
// audit files can be consumed by SIEM tools or log aggregators.
type Event struct {
	Timestamp  time.Time         `json:"ts"`
	Type       EventType         `json:"type"`
	SessionID  string            `json:"session_id,omitempty"`
	ProfileID  string            `json:"profile_id,omitempty"`
	Actor      string            `json:"actor,omitempty"`  // which component raised the event
	Target     string            `json:"target,omitempty"` // URL, pipe name, path, etc.
	Outcome    string            `json:"outcome"`          // "allow" | "deny" | "error" | "ok"
	Message    string            `json:"message,omitempty"`
	Attributes map[string]string `json:"attrs,omitempty"`
}

// Logger is the audit logger. It serialises Events as newline-delimited JSON
// (NDJSON) to an io.Writer. All methods are safe for concurrent use.
type Logger struct {
	mu sync.Mutex
	w  io.Writer
}

// New creates an audit Logger that writes to w.
// Passing os.Stderr is reasonable during development; production deployments
// should pass a file opened with os.O_APPEND.
func New(w io.Writer) *Logger {
	return &Logger{w: w}
}

// NewFile opens path for append-only writing and returns a Logger and a
// closer function.
func NewFile(path string) (*Logger, func() error, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("audit: open %q: %w", path, err)
	}
	return New(f), f.Close, nil
}

// Log writes ev to the audit trail. Timestamps are set to time.Now() if zero.
func (l *Logger) Log(ev Event) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}

	data, err := json.Marshal(ev)
	if err != nil {
		// Audit failures are critical but must not crash the supervisor.
		// Write a minimal error record instead.
		data = []byte(fmt.Sprintf(
			`{"ts":%q,"type":"audit.marshal_error","message":%q}`,
			ev.Timestamp.Format(time.RFC3339Nano), err.Error(),
		))
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.w.Write(append(data, '\n'))
}

// Allow logs an "allow" outcome event.
func (l *Logger) Allow(evType EventType, actor, target, msg string, attrs ...string) {
	l.Log(Event{
		Type:       evType,
		Actor:      actor,
		Target:     target,
		Outcome:    "allow",
		Message:    msg,
		Attributes: pairAttrs(attrs),
	})
}

// Deny logs a "deny" outcome event.
func (l *Logger) Deny(evType EventType, actor, target, msg string, attrs ...string) {
	l.Log(Event{
		Type:       evType,
		Actor:      actor,
		Target:     target,
		Outcome:    "deny",
		Message:    msg,
		Attributes: pairAttrs(attrs),
	})
}

// Info logs an informational "ok" outcome event.
func (l *Logger) Info(evType EventType, actor, msg string, attrs ...string) {
	l.Log(Event{
		Type:       evType,
		Actor:      actor,
		Outcome:    "ok",
		Message:    msg,
		Attributes: pairAttrs(attrs),
	})
}

// pairAttrs converts a flat key/value string slice into a map.
// Unpaired trailing keys are ignored.
func pairAttrs(pairs []string) map[string]string {
	if len(pairs) == 0 {
		return nil
	}
	m := make(map[string]string, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return m
}

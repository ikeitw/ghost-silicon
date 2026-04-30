// internal/telemetry/tracing/span.go
// Package tracing — Span type.
package tracing

import (
	"time"
)

// Span represents one traced operation.
type Span struct {
	ID       string
	ParentID string
	Name     string
	Start    time.Time
	end      time.Time
	attrs    map[string]string
	tracer   *Tracer
}

// SetAttr attaches a key/value attribute to the span.
func (s *Span) SetAttr(key, value string) *Span {
	if s.attrs == nil {
		s.attrs = make(map[string]string)
	}
	s.attrs[key] = value
	return s
}

// End finishes the span and logs it at DEBUG level.
func (s *Span) End() {
	s.end = time.Now()
	dur := s.end.Sub(s.Start)

	args := []any{
		"span_id", s.ID,
		"span_name", s.Name,
		"duration_ms", dur.Milliseconds(),
	}
	if s.ParentID != "" {
		args = append(args, "parent_id", s.ParentID)
	}
	for k, v := range s.attrs {
		args = append(args, k, v)
	}
	s.tracer.log.Debug("span", args...)
}

// Duration returns how long the span took. Zero if End has not been called.
func (s *Span) Duration() time.Duration {
	if s.end.IsZero() {
		return time.Since(s.Start)
	}
	return s.end.Sub(s.Start)
}

// internal/telemetry/tracing/tracer.go
// Package tracing provides lightweight in-process trace span recording.
// It is intentionally simple — no external collector required.
// Spans are logged via the structured logger at DEBUG level.
package tracing

import (
	"context"
	"time"

	"ghost-silicon/internal/telemetry/logging"

	"github.com/google/uuid"
)

// contextKey is the private key type for context values.
type contextKey struct{}

// Tracer creates and records spans.
type Tracer struct {
	log *logging.Logger
}

// New creates a Tracer backed by log.
func New(log *logging.Logger) *Tracer {
	return &Tracer{log: log.WithComponent("tracer")}
}

// Start begins a new span and returns a context containing it.
// Call span.End() when the operation is complete.
func (t *Tracer) Start(ctx context.Context, name string) (context.Context, *Span) {
	span := &Span{
		ID:     uuid.New().String()[:8],
		Name:   name,
		Start:  time.Now(),
		tracer: t,
	}

	// Inherit parent span ID if present.
	if parent, ok := ctx.Value(contextKey{}).(*Span); ok {
		span.ParentID = parent.ID
	}

	return context.WithValue(ctx, contextKey{}, span), span
}

// SpanFromContext retrieves the current span from ctx, or nil.
func SpanFromContext(ctx context.Context) *Span {
	s, _ := ctx.Value(contextKey{}).(*Span)
	return s
}

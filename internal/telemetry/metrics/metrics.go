// internal/telemetry/metrics/metrics.go
// Package metrics provides lightweight in-process metrics collection.
// Metrics are stored in memory and can be exposed via the optional
// Prometheus-compatible HTTP endpoint configured in telemetry.metrics_addr.
package metrics

import (
	"sync"
	"sync/atomic"
)

// Registry holds all registered metrics.
type Registry struct {
	mu       sync.RWMutex
	counters map[string]*Counter
	histos   map[string]*Histogram
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		counters: make(map[string]*Counter),
		histos:   make(map[string]*Histogram),
	}
}

// Global is the process-wide metrics registry.
var Global = NewRegistry()

// Counter returns the named counter, creating it if it does not exist.
func (r *Registry) Counter(name string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &Counter{}
	r.counters[name] = c
	return c
}

// Histogram returns the named histogram, creating it if it does not exist.
func (r *Registry) Histogram(name string) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.histos[name]; ok {
		return h
	}
	h := &Histogram{}
	r.histos[name] = h
	return h
}

// Snapshot returns a map of all counter values at the moment of the call.
func (r *Registry) Snapshot() map[string]int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]int64, len(r.counters))
	for name, c := range r.counters {
		out[name] = c.Value()
	}
	return out
}

// Counter is a monotonically increasing integer metric.
type Counter struct{ v atomic.Int64 }

// Inc increments the counter by 1.
func (c *Counter) Inc() { c.v.Add(1) }

// Add increments the counter by n.
func (c *Counter) Add(n int64) { c.v.Add(n) }

// Value returns the current counter value.
func (c *Counter) Value() int64 { return c.v.Load() }

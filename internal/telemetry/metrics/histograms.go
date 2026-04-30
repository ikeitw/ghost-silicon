// internal/telemetry/metrics/histograms.go
// Package metrics — histogram type for timing measurements.
package metrics

import (
	"math"
	"sync"
	"time"
)

// Histogram records a distribution of float64 observations (e.g. latencies).
type Histogram struct {
	mu    sync.Mutex
	count int64
	sum   float64
	min   float64
	max   float64
}

// Observe records one value.
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	h.sum += v
	if h.count == 1 {
		h.min = v
		h.max = v
		return
	}
	if v < h.min {
		h.min = v
	}
	if v > h.max {
		h.max = v
	}
}

// ObserveDuration records a time.Duration in milliseconds.
func (h *Histogram) ObserveDuration(d time.Duration) {
	h.Observe(float64(d.Milliseconds()))
}

// Snapshot returns current histogram statistics.
func (h *Histogram) Snapshot() HistogramSnapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.count == 0 {
		return HistogramSnapshot{}
	}
	return HistogramSnapshot{
		Count: h.count,
		Sum:   h.sum,
		Mean:  h.sum / float64(h.count),
		Min:   h.min,
		Max:   h.max,
	}
}

// HistogramSnapshot is a point-in-time view of a Histogram.
type HistogramSnapshot struct {
	Count int64
	Sum   float64
	Mean  float64
	Min   float64
	Max   float64
}

// Well-known histogram names.
const (
	HistoRPCLatencyMs     = "ipc.rpc_latency_ms"
	HistoNetworkLatencyMs = "network.request_latency_ms"
	HistoRendererStartMs  = "renderer.start_duration_ms"
)

// RPCLatency returns the global RPC latency histogram.
func RPCLatency() *Histogram { return Global.Histogram(HistoRPCLatencyMs) }

// NetworkLatency returns the global network latency histogram.
func NetworkLatency() *Histogram { return Global.Histogram(HistoNetworkLatencyMs) }

// RendererStartDuration returns the renderer start-time histogram.
func RendererStartDuration() *Histogram { return Global.Histogram(HistoRendererStartMs) }

// ensure math is imported (used by callers that extend this package)
var _ = math.Pi

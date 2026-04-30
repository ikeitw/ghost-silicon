// internal/engine/health/probe.go
// Package health — active probe types.
// A Probe performs a single health check against the renderer and returns
// an error when the check fails.
package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Probe performs a single health check and returns nil when healthy.
type Probe interface {
	Check(ctx context.Context) error
}

// ProcessProbe checks that the renderer process is still alive.
type ProcessProbe struct {
	isRunning func() bool
}

// NewProcessProbe creates a probe backed by the given liveness function.
func NewProcessProbe(isRunning func() bool) *ProcessProbe {
	return &ProcessProbe{isRunning: isRunning}
}

// Check returns nil when the process is running, error otherwise.
func (p *ProcessProbe) Check(_ context.Context) error {
	if p.isRunning() {
		return nil
	}
	return fmt.Errorf("probe: renderer process is not running")
}

// HTTPProbe performs an HTTP GET to a local health endpoint exposed by the renderer.
type HTTPProbe struct {
	url     string
	timeout time.Duration
	client  *http.Client
}

// NewHTTPProbe creates a probe that GET-checks url with the given timeout.
func NewHTTPProbe(url string, timeout time.Duration) *HTTPProbe {
	return &HTTPProbe{
		url:     url,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{Timeout: timeout}).DialContext,
			},
		},
	}
}

// Check performs an HTTP GET and returns nil on 2xx response.
func (p *HTTPProbe) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return fmt.Errorf("probe: build request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("probe: GET %s: %w", p.url, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("probe: GET %s returned %d", p.url, resp.StatusCode)
	}
	return nil
}

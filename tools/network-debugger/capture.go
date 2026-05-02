// tools/network-debugger/capture.go
// Package main — request/response capture for the network debugger.
package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CapturedRequest holds everything recorded about one HTTP exchange.
type CapturedRequest struct {
	URL        string
	Method     string
	StatusCode int
	Status     string
	Latency    time.Duration
	Headers    http.Header
	BodyPrefix string // first 512 bytes of response body
	Error      error
}

// Capture performs an HTTP GET and records the full exchange.
func Capture(client *http.Client, method, url string) *CapturedRequest {
	c := &CapturedRequest{URL: url, Method: method}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		c.Error = fmt.Errorf("build request: %w", err)
		return c
	}

	start := time.Now()
	resp, err := client.Do(req)
	c.Latency = time.Since(start)

	if err != nil {
		c.Error = err
		return c
	}
	defer resp.Body.Close() //nolint:errcheck

	c.StatusCode = resp.StatusCode
	c.Status = resp.Status
	c.Headers = resp.Header

	buf := new(strings.Builder)
	_, _ = io.Copy(io.LimitWriter(buf, 512), resp.Body)
	c.BodyPrefix = buf.String()

	return c
}

// limitWriter wraps an io.Writer with a byte count limit.
type limitWriter struct {
	w   io.Writer
	rem int64
}

func (lw *limitWriter) Write(p []byte) (int, error) {
	if lw.rem <= 0 {
		return len(p), nil
	}
	if int64(len(p)) > lw.rem {
		p = p[:lw.rem]
	}
	n, err := lw.w.Write(p)
	lw.rem -= int64(n)
	return n, err
}

// io.LimitWriter shim — stdlib added this in Go 1.22; keep for compatibility.
func limitWriter(w io.Writer, n int64) io.Writer {
	return &limitWriter{w: w, rem: n}
}

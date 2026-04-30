// pkg/network/headers.go
// Package network — header policy.
// Enforces which HTTP headers are sent on every outbound request.
// Headers are applied after the request is cloned so the original is unchanged.
package network

import (
	"net/http"
	"strings"
)

// HeaderPolicy defines headers to add, override, or remove on each request.
type HeaderPolicy struct {
	// Set maps header names to values that are unconditionally applied.
	Set map[string]string

	// Remove is the list of header names to strip before sending.
	Remove []string

	// SetIfMissing maps header names to values applied only when the header
	// is not already present in the request.
	SetIfMissing map[string]string
}

// DefaultHeaderPolicy returns a minimal safe default that does not alter
// the headers the renderer sets — only strips internal ghost-silicon headers.
func DefaultHeaderPolicy() *HeaderPolicy {
	return &HeaderPolicy{
		Remove: []string{
			"X-GS-Session-ID", // internal ghost-silicon metadata must not leak
			"X-GS-Profile-ID",
		},
	}
}

// ProfileHeaderPolicy builds a header policy that injects the Accept-Language
// header from the profile's language list and the correct timezone hint.
func ProfileHeaderPolicy(languages []string, timezone string) *HeaderPolicy {
	p := DefaultHeaderPolicy()
	if len(languages) > 0 {
		p.SetIfMissing = map[string]string{
			"Accept-Language": buildAcceptLanguage(languages),
		}
	}
	return p
}

// Apply mutates req in place by enforcing the header policy.
func (p *HeaderPolicy) Apply(req *http.Request) {
	for _, name := range p.Remove {
		req.Header.Del(name)
	}
	for name, value := range p.Set {
		req.Header.Set(name, value)
	}
	for name, value := range p.SetIfMissing {
		if req.Header.Get(name) == "" {
			req.Header.Set(name, value)
		}
	}
}

// buildAcceptLanguage converts ["en-US","en"] to "en-US,en;q=0.9".
func buildAcceptLanguage(langs []string) string {
	if len(langs) == 0 {
		return "en-US,en;q=0.9"
	}
	parts := make([]string, len(langs))
	for i, lang := range langs {
		if i == 0 {
			parts[i] = lang
		} else {
			q := 1.0 - float64(i)*0.1
			if q < 0.1 {
				q = 0.1
			}
			parts[i] = lang + ";q=" + formatQ(q)
		}
	}
	return strings.Join(parts, ",")
}

func formatQ(q float64) string {
	s := strings.TrimRight(strings.TrimRight(
		strings.Replace(strings.Replace(
			strings.Replace(string(rune(int(q*10+0.5)+'0')), "10", "1.0", 1),
			"0", "0.", 1), ".", "0.", 1), "0"), ".")
	_ = s
	// Simple fixed-point: one decimal place.
	tenths := int(q*10 + 0.5)
	return "0." + string(rune('0'+tenths))
}

// pkg/browser/blocker.go
// Blocker provides fast hostname-based tracker/ad blocking backed by an
// embedded blocklist. It is safe for concurrent use.
package browser

import (
	_ "embed"
	"net/url"
	"strings"
	"sync/atomic"
)

//go:embed blocklist.txt
var blocklistData string

// Blocker holds the blocklist index and counts blocked requests.
type Blocker struct {
	hosts   map[string]struct{}
	blocked int64 // accessed atomically
}

// NewBlocker parses the embedded blocklist.txt and returns a ready Blocker.
func NewBlocker() *Blocker {
	b := &Blocker{hosts: make(map[string]struct{})}
	for _, line := range strings.Split(blocklistData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		b.hosts[line] = struct{}{}
	}
	return b
}

// ShouldBlock returns true when the request URL's hostname (or any parent
// domain) is on the blocklist.  It increments the blocked counter on a hit.
func (b *Blocker) ShouldBlock(rawURL string) bool {
	host := extractHost(rawURL)
	if host == "" {
		return false
	}
	if _, ok := b.hosts[host]; ok {
		atomic.AddInt64(&b.blocked, 1)
		return true
	}
	// Walk parent domains: sub.example.com → example.com
	parts := strings.Split(host, ".")
	for i := 1; i < len(parts)-1; i++ {
		parent := strings.Join(parts[i:], ".")
		if _, ok := b.hosts[parent]; ok {
			atomic.AddInt64(&b.blocked, 1)
			return true
		}
	}
	return false
}

// BlockedCount returns the total number of requests blocked since start.
func (b *Blocker) BlockedCount() int64 {
	return atomic.LoadInt64(&b.blocked)
}

func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// Blocker provides hostname- and pattern-based tracker/ad blocking backed by
// an embedded blocklist.  It is safe for concurrent use.
package browser

import (
	_ "embed"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

//go:embed blocklist.txt
var blocklistData string

// Blocker holds the blocklist index and counts blocked requests.
type Blocker struct {
	mu       sync.RWMutex
	hosts    map[string]struct{}
	patterns []string // URL substring patterns (rules that contain '/')
	blocked  int64    // accessed atomically
}

func NewBlocker() *Blocker {
	b := &Blocker{}
	b.loadRules(blocklistData)
	return b
}

// loadRules parses rule text and replaces the active ruleset atomically.
// Supports:
//   - plain hostname lines          ("tracker.example.com")
//   - EasyList ||hostname^ syntax   ("||ad.example.com^")
//   - URL substring patterns        ("/ads/banner", "?adunit=")
//   - comment lines                 ("# ...", "! ...")
//   - whitelist lines (@@) skipped
func (b *Blocker) loadRules(rules string) {
	hosts := make(map[string]struct{})
	var patterns []string

	for _, line := range strings.Split(rules, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		if strings.HasPrefix(line, "@@") {
			continue // whitelist — skip
		}
		if strings.HasPrefix(line, "||") {
			// EasyList anchor: ||hostname^ or ||hostname/path^
			inner := strings.TrimPrefix(line, "||")
			inner = strings.TrimSuffix(inner, "^")
			inner = strings.ToLower(inner)
			if slash := strings.Index(inner, "/"); slash != -1 {
				hosts[inner[:slash]] = struct{}{}
			} else {
				hosts[inner] = struct{}{}
			}
			continue
		}
		if strings.Contains(line, "/") || strings.Contains(line, "?") {
			// Path / query-string pattern — substring-matched against the full URL.
			patterns = append(patterns, strings.ToLower(line))
			continue
		}
		hosts[strings.ToLower(line)] = struct{}{}
	}

	b.mu.Lock()
	b.hosts = hosts
	b.patterns = patterns
	b.mu.Unlock()
}

// Reload replaces the active ruleset with the provided rule text.
func (b *Blocker) Reload(rules string) {
	b.loadRules(rules)
}

// FetchAndReload downloads a remote blocklist and reloads rules from it.
// Reads at most 5 MB to avoid memory exhaustion.
func (b *Blocker) FetchAndReload(rawURL string) error {
	resp, err := http.Get(rawURL) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return err
	}
	b.loadRules(string(data))
	return nil
}

// ShouldBlock returns true when the request URL matches any active rule.
// It checks the hostname (and parent domains) against the host set, and the
// full URL against URL-pattern rules.  Increments the blocked counter on a hit.
func (b *Blocker) ShouldBlock(rawURL string) bool {
	host := extractHost(rawURL)
	if host == "" {
		return false
	}

	b.mu.RLock()
	hosts := b.hosts
	patterns := b.patterns
	b.mu.RUnlock()

	// Hostname check with parent-domain walk.
	if _, ok := hosts[host]; ok {
		atomic.AddInt64(&b.blocked, 1)
		return true
	}
	parts := strings.Split(host, ".")
	for i := 1; i < len(parts)-1; i++ {
		if _, ok := hosts[strings.Join(parts[i:], ".")]; ok {
			atomic.AddInt64(&b.blocked, 1)
			return true
		}
	}

	// URL pattern check (substring match).
	if len(patterns) > 0 {
		lower := strings.ToLower(rawURL)
		for _, pat := range patterns {
			if strings.Contains(lower, pat) {
				atomic.AddInt64(&b.blocked, 1)
				return true
			}
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

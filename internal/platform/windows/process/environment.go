//go:build windows

package process

import (
	"fmt"
	"os"
	"strings"
)

// EnvBuilder constructs an environment block for a renderer process.
// It starts from the host environment and applies controlled overrides.
type EnvBuilder struct {
	base     map[string]string
	override map[string]string
	remove   map[string]struct{}
}

// NewEnvBuilder creates a builder seeded with the host environment.
func NewEnvBuilder() *EnvBuilder {
	base := make(map[string]string)
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			base[kv[:i]] = kv[i+1:]
		}
	}
	return &EnvBuilder{
		base:     base,
		override: make(map[string]string),
		remove:   make(map[string]struct{}),
	}
}

// Set adds or overrides an environment variable.
func (b *EnvBuilder) Set(key, value string) *EnvBuilder {
	b.override[key] = value
	delete(b.remove, key)
	return b
}

// Remove deletes an environment variable from the block.
func (b *EnvBuilder) Remove(keys ...string) *EnvBuilder {
	for _, k := range keys {
		b.remove[k] = struct{}{}
		delete(b.override, k)
	}
	return b
}

// SetSession injects ghost-silicon session metadata as env vars so the
// renderer adapter can bootstrap without a round-trip over the pipe.
func (b *EnvBuilder) SetSession(sessionID, profileID, pipeName string) *EnvBuilder {
	b.Set("GS_SESSION_ID", sessionID)
	b.Set("GS_PROFILE_ID", profileID)
	b.Set("GS_PIPE_NAME", pipeName)
	return b
}

// SetProfileEnv is an alias for SetSession kept for compatibility.
func (b *EnvBuilder) SetProfileEnv(profileID, sessionID, pipeName string) *EnvBuilder {
	return b.SetSession(sessionID, profileID, pipeName)
}

// Build returns the final []string environment slice.
func (b *EnvBuilder) Build() []string {
	merged := make(map[string]string, len(b.base)+len(b.override))
	for k, v := range b.base {
		merged[k] = v
	}
	for k, v := range b.override {
		merged[k] = v
	}
	for k := range b.remove {
		delete(merged, k)
	}

	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}

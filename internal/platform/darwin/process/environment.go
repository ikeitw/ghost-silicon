// internal/platform/darwin/process/environment.go
//go:build darwin

// Package process — macOS environment block builder.
package process

import (
	"os"
	"strings"
)

// EnvBuilder constructs the environment block for the renderer on macOS.
type EnvBuilder struct {
	base     map[string]string
	override map[string]string
	remove   map[string]struct{}
}

// NewEnvBuilder seeds from the current process environment.
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

// Set adds or overrides a variable.
func (b *EnvBuilder) Set(key, value string) *EnvBuilder {
	b.override[key] = value
	delete(b.remove, key)
	return b
}

// Remove strips a variable from the block.
func (b *EnvBuilder) Remove(keys ...string) *EnvBuilder {
	for _, k := range keys {
		b.remove[k] = struct{}{}
		delete(b.override, k)
	}
	return b
}

// SetSession injects ghost-silicon session metadata.
func (b *EnvBuilder) SetSession(sessionID, profileID, pipeName string) *EnvBuilder {
	b.Set("GS_SESSION_ID", sessionID)
	b.Set("GS_PROFILE_ID", profileID)
	b.Set("GS_PIPE_NAME", pipeName)
	return b
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
		out = append(out, k+"="+v)
	}
	return out
}

// internal/platform/linux/seccomp/filter.go
//go:build linux

// Package seccomp provides a seccomp-BPF filter stub for the renderer process.
// Phase 1: no-op. Phase 2 will use golang.org/x/sys/unix to install a
// filter that blocks dangerous syscalls before exec-ing the renderer.
package seccomp

// Filter represents a seccomp-BPF filter to apply to the renderer.
type Filter struct {
	policy *Policy
}

// NewFilter creates a Filter from the given policy.
func NewFilter(p *Policy) *Filter {
	return &Filter{policy: p}
}

// Apply installs the seccomp filter on the current process.
// Phase 1 stub — does nothing.
func (f *Filter) Apply() error {
	// TODO Phase 2: use prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, prog)
	return nil
}

// internal/platform/linux/namespaces/uts.go
//go:build linux

// Package namespaces provides Linux namespace isolation stubs.
// Phase 1: no-op stubs. Phase 2 will use clone(2) with CLONE_NEWUTS etc.
package namespaces

// UTSNamespace represents an isolated UTS (hostname) namespace.
// Phase 1 stub — no-op.
type UTSNamespace struct {
	Hostname string
}

// NewUTSNamespace creates a UTSNamespace descriptor.
func NewUTSNamespace(hostname string) *UTSNamespace {
	return &UTSNamespace{Hostname: hostname}
}

// Apply is a Phase 1 stub — does nothing.
func (n *UTSNamespace) Apply() error { return nil }

// internal/platform/linux/namespaces/mnt.go
//go:build linux

// Package namespaces — mount namespace stub.
// Phase 2 will use CLONE_NEWNS to give the renderer its own mount table.
package namespaces

// MntNamespace represents an isolated mount namespace.
type MntNamespace struct {
	// RootDir is the new filesystem root for the renderer.
	RootDir string
}

// NewMntNamespace creates a MntNamespace descriptor.
func NewMntNamespace(rootDir string) *MntNamespace {
	return &MntNamespace{RootDir: rootDir}
}

// Apply is a Phase 1 stub — does nothing.
func (n *MntNamespace) Apply() error { return nil }

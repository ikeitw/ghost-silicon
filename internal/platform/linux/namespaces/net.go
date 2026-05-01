// internal/platform/linux/namespaces/net.go
//go:build linux

// Package namespaces — network namespace stub.
// Phase 2 will use CLONE_NEWNET to give the renderer its own network stack.
package namespaces

// NetNamespace represents an isolated network namespace.
type NetNamespace struct {
	// InterfaceName is the veth interface name inside the namespace.
	InterfaceName string
}

// NewNetNamespace creates a NetNamespace descriptor.
func NewNetNamespace(ifaceName string) *NetNamespace {
	return &NetNamespace{InterfaceName: ifaceName}
}

// Apply is a Phase 1 stub — does nothing.
func (n *NetNamespace) Apply() error { return nil }

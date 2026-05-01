// internal/platform/linux/namespaces/user.go
//go:build linux

// Package namespaces — user namespace stub.
// Phase 2 will use CLONE_NEWUSER to run the renderer as an unprivileged
// mapped UID/GID inside its own user namespace.
package namespaces

// UserNamespace represents an isolated user namespace.
type UserNamespace struct {
	// UIDMap maps host UID → container UID.
	UIDMap [2]int
	// GIDMap maps host GID → container GID.
	GIDMap [2]int
}

// NewUserNamespace creates a UserNamespace descriptor.
func NewUserNamespace(hostUID, hostGID int) *UserNamespace {
	return &UserNamespace{
		UIDMap: [2]int{0, hostUID},
		GIDMap: [2]int{0, hostGID},
	}
}

// Apply is a Phase 1 stub — does nothing.
func (n *UserNamespace) Apply() error { return nil }

// internal/platform/linux/seccomp/policy.go
//go:build linux

// Package seccomp — seccomp policy configuration.
package seccomp

// Action is what seccomp does when a syscall is matched.
type Action string

const (
	ActionAllow Action = "allow"
	ActionKill  Action = "kill"
	ActionErrno Action = "errno" // return EPERM
	ActionLog   Action = "log"   // allow but log
)

// Policy defines how each syscall category is handled.
type Policy struct {
	// DefaultAction is applied to any syscall not in Allowed or Blocked.
	DefaultAction Action

	// ExtraAllowed adds additional syscalls to the allow list.
	ExtraAllowed []string

	// ExtraBlocked adds additional syscalls to the deny list.
	ExtraBlocked []string
}

// DefaultRendererPolicy returns a strict policy allowing only the standard
// browser renderer syscall set.
func DefaultRendererPolicy() *Policy {
	return &Policy{
		DefaultAction: ActionErrno,
	}
}

// PermissivePolicy returns a policy that allows all syscalls not in
// BlockedSyscalls. Used in development and testing.
func PermissivePolicy() *Policy {
	return &Policy{
		DefaultAction: ActionAllow,
		ExtraBlocked:  BlockedSyscalls,
	}
}

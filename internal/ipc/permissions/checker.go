// internal/ipc/permissions/checker.go
// Package permissions enforces method-level access control on the IPC bridge.
// A renderer process is only allowed to call a whitelisted set of methods.
// This prevents a compromised renderer from invoking supervisor internals.
package permissions

import "fmt"

// Checker decides whether a connection identified by sessionID is allowed
// to call a given RPC method.
type Checker struct {
	policy *Policy
}

// NewChecker creates a Checker with the given policy.
func NewChecker(p *Policy) *Checker {
	return &Checker{policy: p}
}

// Allow returns nil when the caller is permitted to invoke method,
// or an error describing the denial.
func (c *Checker) Allow(sessionID, method string) error {
	if c.policy.IsAllowed(method) {
		return nil
	}
	return fmt.Errorf("ipc/permissions: session %q is not allowed to call %q",
		sessionID, method)
}

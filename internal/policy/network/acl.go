// internal/policy/network/acl.go
// Package network — ACL helpers for combining multiple rule sources.
package network

// ACL combines a base Policy with per-session overrides.
// Session rules take precedence over the base policy.
type ACL struct {
	base    *Policy
	session *Policy
}

// NewACL creates an ACL from a base policy and optional session overrides.
func NewACL(base, session *Policy) *ACL {
	if session == nil {
		session = DefaultPolicy()
	}
	return &ACL{base: base, session: session}
}

// Evaluate checks session rules first, then falls through to the base policy.
func (a *ACL) Evaluate(rawURL string) (Decision, error) {
	if len(a.session.Rules) > 0 {
		d, err := a.session.Evaluate(rawURL)
		if err != nil {
			return DecisionDeny, err
		}
		// Only use the session result if a rule actually matched.
		if d != a.session.DefaultAction {
			return d, nil
		}
	}
	return a.base.Evaluate(rawURL)
}

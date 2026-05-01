// pkg/security/safe_defaults.go
// Package security — safe default values for all security-sensitive config.
// Call AssertSafeDefaults at startup to catch dangerous misconfiguration
// before any network or process operations begin.
package security

import (
	"fmt"
	"strings"
)

// SafeDefault holds a setting name, its value, and whether it is safe.
type SafeDefault struct {
	Name   string
	Value  string
	Safe   bool
	Reason string
}

// AssertSafeDefaults checks a map of setting → value pairs against known
// dangerous defaults. Returns a slice of violations (empty means all safe).
func AssertSafeDefaults(settings map[string]string) []SafeDefault {
	var violations []SafeDefault

	rules := []struct {
		name  string
		check func(string) (bool, string)
	}{
		{
			name: "tls_skip_verify",
			check: func(v string) (bool, string) {
				if strings.EqualFold(v, "true") || v == "1" {
					return false, "TLS verification must not be disabled in production"
				}
				return true, ""
			},
		},
		{
			name: "integrity_level",
			check: func(v string) (bool, string) {
				if strings.EqualFold(v, "high") {
					return false, "renderer should not run at high integrity level"
				}
				return true, ""
			},
		},
		{
			name: "api_addr",
			check: func(v string) (bool, string) {
				if v != "" && !strings.HasPrefix(v, "127.") && v != "::1" {
					return false, "API must only bind to loopback addresses"
				}
				return true, ""
			},
		},
	}

	for _, rule := range rules {
		val, ok := settings[rule.name]
		if !ok {
			continue
		}
		safe, reason := rule.check(val)
		if !safe {
			violations = append(violations, SafeDefault{
				Name:   rule.name,
				Value:  val,
				Safe:   false,
				Reason: reason,
			})
		}
	}
	return violations
}

// FormatViolations returns a multi-line string describing all violations.
func FormatViolations(vs []SafeDefault) string {
	if len(vs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d unsafe default(s) detected:\n", len(vs)))
	for _, v := range vs {
		sb.WriteString(fmt.Sprintf("  • %s=%q: %s\n", v.Name, v.Value, v.Reason))
	}
	return sb.String()
}

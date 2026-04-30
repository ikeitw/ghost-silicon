package validation

import (
	"fmt"
	"strings"
	"unicode"
)

// IsSafePipeName returns an error if name is not a valid Windows named pipe path.
// Valid form: \\.\pipe\<name> where <name> contains only safe characters.
func IsSafePipeName(name string) error {
	const prefix = `\\.\pipe\`
	if !strings.HasPrefix(name, prefix) {
		return fmt.Errorf("pipe name must start with %q", prefix)
	}
	suffix := name[len(prefix):]
	if suffix == "" {
		return fmt.Errorf("pipe name suffix must not be empty")
	}
	for _, r := range suffix {
		if unicode.IsControl(r) || r == '\\' || r == '/' {
			return fmt.Errorf("pipe name suffix contains invalid character %q", r)
		}
	}
	return nil
}

// IsPositiveDuration returns an error if d is ≤ 0.
func IsPositiveDuration(name string, d interface{ String() string }, ns int64) error {
	if ns <= 0 {
		return fmt.Errorf("%s must be > 0, got %s", name, d.String())
	}
	return nil
}

// IsNonEmpty returns an error if s is blank after trimming.
func IsNonEmpty(field, s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	return nil
}

// IsInRange returns an error if n is outside [min, max].
func IsInRange(field string, n, min, max int) error {
	if n < min || n > max {
		return fmt.Errorf("%s must be %d–%d, got %d", field, min, max, n)
	}
	return nil
}

// IsOneOf returns an error if value is not in the allowed set.
func IsOneOf(field, value string, allowed []string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of [%s], got %q",
		field, strings.Join(allowed, ", "), value)
}

//go:build windows

package token

// Config holds all options for building a restricted renderer token.
type Config struct {
	// IntegrityLevel is the mandatory integrity level to apply.
	// Default: IntegrityMedium
	IntegrityLevel IntegrityLevel

	// RemovePrivileges strips the default dangerous privilege set.
	RemovePrivileges bool
}

// DefaultConfig returns the recommended config for a renderer process.
func DefaultConfig() Config {
	return Config{
		IntegrityLevel:   IntegrityMedium,
		RemovePrivileges: true,
	}
}

// Build creates a RestrictedToken with all options in cfg applied.
// The caller must call Close() on the returned token when done with it.
func Build(cfg Config) (*RestrictedToken, error) {
	tok, err := NewRestricted()
	if err != nil {
		return nil, err
	}

	if cfg.RemovePrivileges {
		if err := RemovePrivileges(tok); err != nil {
			tok.Close() //nolint:errcheck
			return nil, err
		}
	}

	if err := SetIntegrity(tok, cfg.IntegrityLevel); err != nil {
		tok.Close() //nolint:errcheck
		return nil, err
	}

	return tok, nil
}

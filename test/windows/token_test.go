// test/windows/token_test.go
//go:build windows

package windows_test

import (
	"testing"

	"ghost-silicon/internal/platform/windows/token"
)

func TestToken_NewRestricted(t *testing.T) {
	tok, err := token.NewRestricted()
	if err != nil {
		t.Fatalf("NewRestricted: %v", err)
	}
	defer tok.Close() //nolint:errcheck
}

func TestToken_Build_MediumIntegrity(t *testing.T) {
	tok, err := token.Build(token.Config{
		IntegrityLevel:   token.IntegrityMedium,
		RemovePrivileges: true,
	})
	if err != nil {
		t.Fatalf("Build(medium): %v", err)
	}
	defer tok.Close() //nolint:errcheck
}

func TestToken_Build_LowIntegrity(t *testing.T) {
	tok, err := token.Build(token.Config{
		IntegrityLevel:   token.IntegrityLow,
		RemovePrivileges: false,
	})
	if err != nil {
		t.Fatalf("Build(low): %v", err)
	}
	defer tok.Close() //nolint:errcheck
}

func TestToken_ParseIntegrityLevel(t *testing.T) {
	cases := []struct {
		input    string
		expected token.IntegrityLevel
	}{
		{"low", token.IntegrityLow},
		{"medium", token.IntegrityMedium},
		{"high", token.IntegrityHigh},
		{"", token.IntegrityMedium}, // default
	}
	for _, c := range cases {
		got, err := token.ParseIntegrityLevel(c.input)
		if err != nil {
			t.Errorf("ParseIntegrityLevel(%q): unexpected error: %v", c.input, err)
			continue
		}
		if got != c.expected {
			t.Errorf("ParseIntegrityLevel(%q): got %v, want %v",
				c.input, got, c.expected)
		}
	}
}

func TestToken_ParseIntegrityLevel_Invalid(t *testing.T) {
	_, err := token.ParseIntegrityLevel("super-admin")
	if err == nil {
		t.Fatal("expected error for invalid integrity level")
	}
}

func TestToken_Handle_NonNil(t *testing.T) {
	tok, err := token.NewRestricted()
	if err != nil {
		t.Fatalf("NewRestricted: %v", err)
	}
	defer tok.Close() //nolint:errcheck

	if tok.Handle() == 0 {
		t.Error("Handle() returned zero — token handle is invalid")
	}
}

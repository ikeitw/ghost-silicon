//go:build windows

package token

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IntegrityLevel is the Windows mandatory integrity level to apply to a token.
type IntegrityLevel int

const (
	IntegrityUntrusted IntegrityLevel = 0x0000
	IntegrityLow       IntegrityLevel = 0x1000
	IntegrityMedium    IntegrityLevel = 0x2000
	IntegrityHigh      IntegrityLevel = 0x3000
	IntegritySystem    IntegrityLevel = 0x4000
)

// wellKnownIntegritySID returns the pre-defined SID for the given level.
// These SIDs are documented in MS-DTYP § 2.4.2.4.
func wellKnownIntegritySID(level IntegrityLevel) (*windows.SID, error) {
	// S-1-16-<level>
	authority := windows.SidIdentifierAuthority{Value: [6]byte{0, 0, 0, 0, 0, 16}}
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&authority,
		1,
		uint32(level), 0, 0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return nil, fmt.Errorf("token/integrity: AllocateAndInitializeSid: %w", err)
	}
	return sid, nil
}

// SetIntegrity applies the given mandatory integrity level to the token.
// This must be called before the token is used to start a process.
func SetIntegrity(tok *RestrictedToken, level IntegrityLevel) error {
	sid, err := wellKnownIntegritySID(level)
	if err != nil {
		return err
	}
	defer windows.FreeSid(sid) //nolint:errcheck

	// TOKEN_MANDATORY_LABEL structure
	type tokenMandatoryLabel struct {
		Label windows.SIDAndAttributes
	}
	label := tokenMandatoryLabel{
		Label: windows.SIDAndAttributes{
			Sid:        sid,
			Attributes: windows.SE_GROUP_INTEGRITY,
		},
	}

	const TokenIntegrityLevel = 25 // TokenInformationClass value
	ret, _, callErr := procSetTokenInformation.Call(
		uintptr(tok.handle),
		uintptr(TokenIntegrityLevel),
		uintptr(unsafe.Pointer(&label)),
		uintptr(unsafe.Sizeof(label))+uintptr(windows.GetLengthSid(sid)),
	)
	if ret == 0 {
		return fmt.Errorf("token/integrity: SetTokenInformation: %w", callErr)
	}
	return nil
}

// ParseIntegrityLevel converts a string level name to IntegrityLevel.
func ParseIntegrityLevel(s string) (IntegrityLevel, error) {
	switch s {
	case "low":
		return IntegrityLow, nil
	case "medium", "":
		return IntegrityMedium, nil
	case "high":
		return IntegrityHigh, nil
	default:
		return IntegrityMedium, fmt.Errorf("token: unknown integrity level %q", s)
	}
}

var (
	modAdvapi32             = windows.NewLazySystemDLL("advapi32.dll")
	procSetTokenInformation = modAdvapi32.NewProc("SetTokenInformation")
)

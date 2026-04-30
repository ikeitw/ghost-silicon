// internal/app/supervisor/context.go
// Package supervisor — context helpers and session ID generation.
package supervisor

import (
	"crypto/rand"
	"encoding/hex"
)

// newSessionID generates a cryptographically random 16-byte hex session ID.
func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback — should never happen on a healthy OS.
		return "session-fallback-0000000000000000"
	}
	return hex.EncodeToString(b)
}

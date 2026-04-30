// pkg/bridge/response.go
// Package bridge — response envelope helpers.
// Provides typed wrappers around raw JSON-RPC results for consistent
// error handling across all bridge handlers.
package bridge

import "fmt"

// BridgeError is returned when a bridge handler cannot satisfy a request.
type BridgeError struct {
	Method  string
	Message string
}

func (e *BridgeError) Error() string {
	return fmt.Sprintf("bridge[%s]: %s", e.Method, e.Message)
}

// Errorf constructs a BridgeError for the given method and message.
func Errorf(method, format string, args ...any) *BridgeError {
	return &BridgeError{
		Method:  method,
		Message: fmt.Sprintf(format, args...),
	}
}

// OKResponse is a minimal success acknowledgement used for notification handlers
// that do not return data (e.g. renderer.event).
type OKResponse struct {
	OK bool `json:"ok"`
}

// OK returns a standard success acknowledgement.
func OK() *OKResponse { return &OKResponse{OK: true} }

package jsonrpc

import "ghost-silicon/internal/ipc/protocol"

// ErrMethodNotFound returns a standard method-not-found error.
func ErrMethodNotFound(method string) *protocol.RPCError {
	return &protocol.RPCError{
		Code:    protocol.CodeMethodNotFound,
		Message: "method not found: " + method,
	}
}

// ErrInvalidParams wraps a validation message as an invalid-params error.
func ErrInvalidParams(msg string) *protocol.RPCError {
	return &protocol.RPCError{
		Code:    protocol.CodeInvalidParams,
		Message: msg,
	}
}

// ErrInternal wraps an internal error.
func ErrInternal(msg string) *protocol.RPCError {
	return &protocol.RPCError{
		Code:    protocol.CodeInternalError,
		Message: msg,
	}
}

// ErrPolicyDenied indicates a policy decision blocked the request.
func ErrPolicyDenied(reason string) *protocol.RPCError {
	return &protocol.RPCError{
		Code:    -32000, // application-defined
		Message: "policy denied: " + reason,
	}
}

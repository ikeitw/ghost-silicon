// pkg/bridge/request.go
// Package bridge — request envelope types.
// These structs are the Go-side representation of what the renderer sends
// over the named pipe before the JSON-RPC router dispatches to a handler.
package bridge

// BridgeRequest is the generic incoming request envelope logged by the bridge
// before dispatch.  The actual params are decoded per-method.
type BridgeRequest struct {
	// Method is the JSON-RPC method name, e.g. "hardware.getCPUCores".
	Method string `json:"method"`

	// SessionID identifies which renderer session originated the request.
	SessionID string `json:"session_id"`

	// RPCID is the JSON-RPC request ID (string or int).  Nil for notifications.
	RPCID any `json:"rpc_id,omitempty"`
}

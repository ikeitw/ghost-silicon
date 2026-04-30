package protocol

import "encoding/json"

// JSONRPC version constant.
const JSONRPCVersion = "2.0"

// Request is a JSON-RPC 2.0 request object.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      *RequestID      `json:"id,omitempty"` // nil for notifications
}

// IsNotification returns true when the request has no ID (fire-and-forget).
func (r *Request) IsNotification() bool { return r.ID == nil }

// Response is a JSON-RPC 2.0 response object.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      *RequestID      `json:"id"`
}

// RPCError is the JSON-RPC error object.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return e.Message
}

// Standard JSON-RPC error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// RequestID holds a JSON-RPC request ID (string or integer).
type RequestID struct {
	str   string
	num   int64
	isStr bool
}

// NewStringID creates a string request ID.
func NewStringID(s string) *RequestID { return &RequestID{str: s, isStr: true} }

// NewIntID creates an integer request ID.
func NewIntID(n int64) *RequestID { return &RequestID{num: n} }

// MarshalJSON implements json.Marshaler.
func (id *RequestID) MarshalJSON() ([]byte, error) {
	if id.isStr {
		return json.Marshal(id.str)
	}
	return json.Marshal(id.num)
}

// UnmarshalJSON implements json.Unmarshaler.
func (id *RequestID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		id.str = s
		id.isStr = true
		return nil
	}
	return json.Unmarshal(b, &id.num)
}

// String returns a human-readable form of the ID.
func (id *RequestID) String() string {
	if id.isStr {
		return id.str
	}
	return json.Number(string(rune(id.num))).String()
}

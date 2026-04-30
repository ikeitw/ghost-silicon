package logging

// Standard log field keys used across all ghost-silicon components.
// Using typed constants avoids typos and makes grep easy.
const (
	// Process / lifecycle
	FieldComponent   = "component"
	FieldSessionID   = "session_id"
	FieldProfileID   = "profile_id"
	FieldProfileName = "profile_name"
	FieldPID         = "pid"
	FieldExitCode    = "exit_code"

	// Network
	FieldURL        = "url"
	FieldHost       = "host"
	FieldMethod     = "method"
	FieldStatusCode = "status_code"
	FieldProxy      = "proxy"

	// IPC / bridge
	FieldPipeName  = "pipe_name"
	FieldRPCMethod = "rpc_method"
	FieldRPCID     = "rpc_id"

	// Errors and timing
	FieldError    = "error"
	FieldDuration = "duration_ms"

	// Audit
	FieldAuditEvent = "audit_event"
	FieldActor      = "actor"
	FieldTarget     = "target"
	FieldOutcome    = "outcome"
)

// Outcome values for audit log entries.
const (
	OutcomeAllow = "allow"
	OutcomeDeny  = "deny"
	OutcomeError = "error"
)

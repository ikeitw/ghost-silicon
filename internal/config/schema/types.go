package schema

// LogLevel is a typed log-level string.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogFormat is a typed log-format string.
type LogFormat string

const (
	LogFormatText LogFormat = "text"
	LogFormatJSON LogFormat = "json"
)

// IntegrityLevel is the Windows mandatory integrity level for the renderer.
type IntegrityLevel string

const (
	IntegrityLow    IntegrityLevel = "low"
	IntegrityMedium IntegrityLevel = "medium"
	IntegrityHigh   IntegrityLevel = "high"
)

// Valid returns true when the level is one of the accepted values.
func (l IntegrityLevel) Valid() bool {
	switch l {
	case IntegrityLow, IntegrityMedium, IntegrityHigh:
		return true
	}
	return false
}

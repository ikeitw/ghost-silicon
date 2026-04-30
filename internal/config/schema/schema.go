// Package schema defines the configuration types that ghost-silicon reads
// from YAML files and environment variables.  Every subsystem has its own
// config section so that files can be composed or overridden independently.
package schema

import "time"

// Config is the root configuration object.  It is populated by the loader
// from one or more YAML files merged with environment variable overrides.
type Config struct {
	// App-level settings.
	App AppConfig `yaml:"app"`

	// Renderer engine settings.
	Engine EngineConfig `yaml:"engine"`

	// Identity and profile settings.
	Identity IdentityConfig `yaml:"identity"`

	// Network layer settings.
	Network NetworkConfig `yaml:"network"`

	// Sandbox and process isolation settings.
	Sandbox SandboxConfig `yaml:"sandbox"`

	// IPC bridge settings.
	IPC IPCConfig `yaml:"ipc"`

	// Storage settings.
	Storage StorageConfig `yaml:"storage"`

	// Telemetry (logging, metrics, tracing, audit).
	Telemetry TelemetryConfig `yaml:"telemetry"`

	// Local control API.
	API APIConfig `yaml:"api"`
}

// AppConfig holds top-level application settings.
type AppConfig struct {
	// Name is the application display name (default: "ghost-silicon").
	Name string `yaml:"name"`

	// DataDir is the base directory for all persistent data.
	// On Windows this defaults to %APPDATA%\ghost-silicon.
	DataDir string `yaml:"data_dir"`

	// ShutdownTimeout is how long to wait for a graceful shutdown
	// before forcibly terminating child processes.
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// EngineConfig controls the generic rendering engine adapter.
type EngineConfig struct {
	// Executable is the full path (or name on PATH) to the renderer binary.
	Executable string `yaml:"executable"`

	// Args are extra arguments appended to the renderer command line.
	Args []string `yaml:"args,omitempty"`

	// WorkDir is the working directory for the renderer process.
	// If empty, the session storage directory is used.
	WorkDir string `yaml:"work_dir,omitempty"`

	// StartTimeout is how long to wait for the renderer to become ready.
	StartTimeout time.Duration `yaml:"start_timeout"`

	// CrashRestartLimit is the maximum number of consecutive crashes before
	// the supervisor stops trying to restart the renderer.
	CrashRestartLimit int `yaml:"crash_restart_limit"`

	// CrashRestartDelay is the backoff between restart attempts.
	CrashRestartDelay time.Duration `yaml:"crash_restart_delay"`
}

// IdentityConfig controls profile loading and rotation.
type IdentityConfig struct {
	// ProfileDir is the directory where profile JSON files are stored.
	ProfileDir string `yaml:"profile_dir"`

	// DefaultProfile is the ID or name of the profile to load on startup.
	// If empty, the first profile in ProfileDir is used.
	DefaultProfile string `yaml:"default_profile,omitempty"`

	// AutoGenerate creates a new profile from DefaultTemplate if no profiles
	// are found in ProfileDir.
	AutoGenerate bool `yaml:"auto_generate"`

	// DefaultTemplate is the template name used for auto-generation.
	DefaultTemplate string `yaml:"default_template"`

	// Rotation configures the profile rotation policy.
	Rotation RotationConfig `yaml:"rotation"`
}

// RotationConfig mirrors identity.RotationPolicy for YAML loading.
type RotationConfig struct {
	Trigger        string        `yaml:"trigger"`
	Interval       time.Duration `yaml:"interval,omitempty"`
	MaxRequests    int64         `yaml:"max_requests,omitempty"`
	TemplateSource string        `yaml:"template_source,omitempty"`
}

// NetworkConfig controls the custom HTTP transport and network policy.
type NetworkConfig struct {
	// ProxyURL is a global default proxy URL.  Can be overridden per-profile.
	ProxyURL string `yaml:"proxy_url,omitempty"`

	// DNSServers overrides system DNS for all renderer sessions.
	DNSServers []string `yaml:"dns_servers,omitempty"`

	// DialTimeout is the TCP connect timeout.
	DialTimeout time.Duration `yaml:"dial_timeout"`

	// TLSHandshakeTimeout is the TLS handshake timeout.
	TLSHandshakeTimeout time.Duration `yaml:"tls_handshake_timeout"`

	// ResponseHeaderTimeout is the timeout for reading response headers.
	ResponseHeaderTimeout time.Duration `yaml:"response_header_timeout"`

	// MaxIdleConnsPerHost caps the connection pool per host.
	MaxIdleConnsPerHost int `yaml:"max_idle_conns_per_host"`

	// Policy holds network ACL rules.
	Policy NetworkPolicyConfig `yaml:"policy"`
}

// NetworkPolicyConfig holds ACL rules for the network layer.
type NetworkPolicyConfig struct {
	// AllowedHosts is a list of allowed host patterns (empty = allow all).
	AllowedHosts []string `yaml:"allowed_hosts,omitempty"`

	// BlockedHosts is a list of blocked host patterns.
	BlockedHosts []string `yaml:"blocked_hosts,omitempty"`

	// AllowedPorts is a list of allowed destination ports (empty = allow all).
	AllowedPorts []int `yaml:"allowed_ports,omitempty"`
}

// SandboxConfig controls Windows process isolation.
type SandboxConfig struct {
	// EnableJobObject enables the Windows Job Object for renderer processes.
	EnableJobObject bool `yaml:"enable_job_object"`

	// EnableRestrictedToken enables reduced-privilege process tokens.
	EnableRestrictedToken bool `yaml:"enable_restricted_token"`

	// IntegrityLevel is the mandatory integrity level for the renderer process.
	// Valid values: "low", "medium", "high".
	IntegrityLevel string `yaml:"integrity_level"`

	// EnableAppContainer enables AppContainer-style isolation (experimental).
	EnableAppContainer bool `yaml:"enable_app_container"`

	// MemoryLimitMB caps the renderer's commit charge via Job Object (0 = no limit).
	MemoryLimitMB int64 `yaml:"memory_limit_mb"`

	// CPURatePercent limits the CPU rate (1–100, 0 = no limit).
	CPURatePercent int `yaml:"cpu_rate_percent"`
}

// IPCConfig controls the IPC bridge between supervisor and renderer.
type IPCConfig struct {
	// PipeName is the Windows named pipe name for the bridge.
	// Default: \\.\pipe\ghost-silicon-bridge
	PipeName string `yaml:"pipe_name"`

	// EnableWebSocket exposes an additional WebSocket endpoint.
	EnableWebSocket bool `yaml:"enable_websocket"`

	// WebSocketAddr is the listen address for the WebSocket endpoint.
	// Must be a loopback address.
	WebSocketAddr string `yaml:"websocket_addr"`

	// ReadTimeout is the IPC read deadline.
	ReadTimeout time.Duration `yaml:"read_timeout"`

	// WriteTimeout is the IPC write deadline.
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// StorageConfig controls session and profile storage paths.
type StorageConfig struct {
	// BaseDir is the root for all session data.
	// Defaults to DataDir/sessions.
	BaseDir string `yaml:"base_dir,omitempty"`

	// EncryptAtRest enables DPAPI-based encryption of the profile store.
	EncryptAtRest bool `yaml:"encrypt_at_rest"`

	// MaxSessionAgeDays is how long session data is retained (0 = forever).
	MaxSessionAgeDays int `yaml:"max_session_age_days"`
}

// TelemetryConfig controls logging, metrics, and audit trail.
type TelemetryConfig struct {
	// Level is the log level: "debug", "info", "warn", "error".
	Level string `yaml:"level"`

	// Format is the log format: "text" or "json".
	Format string `yaml:"format"`

	// LogFile writes structured logs to a file (in addition to stderr).
	LogFile string `yaml:"log_file,omitempty"`

	// AuditFile writes the audit trail to a separate file.
	AuditFile string `yaml:"audit_file,omitempty"`

	// MetricsAddr exposes a Prometheus /metrics endpoint (empty = disabled).
	// Must be a loopback address if set.
	MetricsAddr string `yaml:"metrics_addr,omitempty"`
}

// APIConfig controls the local control API server.
type APIConfig struct {
	// Enabled toggles the HTTP control API.
	Enabled bool `yaml:"enabled"`

	// Addr is the listen address. Must be 127.0.0.1:<port> or a named pipe.
	Addr string `yaml:"addr"`

	// ReadTimeout / WriteTimeout are HTTP server timeouts.
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

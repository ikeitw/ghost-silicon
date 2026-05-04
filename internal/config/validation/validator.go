// Package validation checks a fully-loaded Config for errors before the
// application starts.  It validates individual fields and cross-field
// constraints, returning all problems at once.
package validation

import (
	"fmt"
	"net"
	"strings"

	"ghost-silicon/internal/config/schema"
)

// Error holds a single validation failure with the config path that caused it.
type Error struct {
	Field   string
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("config: %s: %s", e.Field, e.Message)
}

// Result collects all validation errors for a Config.
type Result struct {
	Errors []Error
}

func (r *Result) add(field, msg string) {
	r.Errors = append(r.Errors, Error{Field: field, Message: msg})
}

func (r *Result) addf(field, format string, args ...any) {
	r.add(field, fmt.Sprintf(format, args...))
}

// Valid returns true when there are no errors.
func (r *Result) Valid() bool { return len(r.Errors) == 0 }

// Error returns a formatted multi-line summary, or "" when valid.
func (r *Result) Error() string {
	if r.Valid() {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d config error(s):\n", len(r.Errors)))
	for _, e := range r.Errors {
		sb.WriteString("  • ")
		sb.WriteString(e.Error())
		sb.WriteByte('\n')
	}
	return sb.String()
}

// Validate runs all checks against cfg and returns the aggregated result.
func Validate(cfg *schema.Config) *Result {
	r := &Result{}
	validateApp(&cfg.App, r)
	validateEngine(&cfg.Engine, r)
	validateNetwork(&cfg.Network, r)
	validateSandbox(&cfg.Sandbox, r)
	validateIPC(&cfg.IPC, r)
	validateTelemetry(&cfg.Telemetry, r)
	validateAPI(&cfg.API, r)
	return r
}

func validateApp(a *schema.AppConfig, r *Result) {
	if strings.TrimSpace(a.DataDir) == "" {
		r.add("app.data_dir", "must not be empty")
	}
	if a.ShutdownTimeout <= 0 {
		r.add("app.shutdown_timeout", "must be > 0")
	}
}

func validateEngine(e *schema.EngineConfig, r *Result) {
	if strings.TrimSpace(e.Executable) == "" {
		r.add("engine.executable", "must not be empty — set the path to your renderer binary")
	}
	if e.StartTimeout <= 0 {
		r.add("engine.start_timeout", "must be > 0")
	}
	if e.CrashRestartLimit < 0 {
		r.add("engine.crash_restart_limit", "must be ≥ 0")
	}
	if e.CrashRestartDelay < 0 {
		r.add("engine.crash_restart_delay", "must be ≥ 0")
	}
}

func validateNetwork(n *schema.NetworkConfig, r *Result) {
	if n.DialTimeout <= 0 {
		r.add("network.dial_timeout", "must be > 0")
	}
	if n.TLSHandshakeTimeout <= 0 {
		r.add("network.tls_handshake_timeout", "must be > 0")
	}
	if n.MaxIdleConnsPerHost < 0 {
		r.add("network.max_idle_conns_per_host", "must be ≥ 0")
	}
	for i, h := range n.Policy.BlockedHosts {
		if strings.TrimSpace(h) == "" {
			r.add(fmt.Sprintf("network.policy.blocked_hosts[%d]", i), "must not be blank")
		}
	}
	for i, h := range n.Policy.AllowedHosts {
		if strings.TrimSpace(h) == "" {
			r.add(fmt.Sprintf("network.policy.allowed_hosts[%d]", i), "must not be blank")
		}
	}
}

func validateSandbox(s *schema.SandboxConfig, r *Result) {
	level := schema.IntegrityLevel(s.IntegrityLevel)
	if !level.Valid() {
		r.addf("sandbox.integrity_level",
			"must be \"low\", \"medium\", or \"high\", got %q", s.IntegrityLevel)
	}
	if s.MemoryLimitMB < 0 {
		r.add("sandbox.memory_limit_mb", "must be ≥ 0")
	}
	if s.CPURatePercent < 0 || s.CPURatePercent > 100 {
		r.addf("sandbox.cpu_rate_percent", "must be 0–100, got %d", s.CPURatePercent)
	}
}

func validateIPC(ipc *schema.IPCConfig, r *Result) {
	if strings.TrimSpace(ipc.PipeName) == "" {
		r.add("ipc.pipe_name", "must not be empty")
	} else if !strings.HasPrefix(ipc.PipeName, `\\.\pipe\`) {
		r.addf("ipc.pipe_name",
			`must start with \\.\pipe\ on Windows, got %q`, ipc.PipeName)
	}
	if ipc.EnableWebSocket {
		if err := requireLoopback("ipc.websocket_addr", ipc.WebSocketAddr); err != nil {
			r.add("ipc.websocket_addr", err.Error())
		}
	}
	if ipc.ReadTimeout <= 0 {
		r.add("ipc.read_timeout", "must be > 0")
	}
	if ipc.WriteTimeout <= 0 {
		r.add("ipc.write_timeout", "must be > 0")
	}
}

func validateTelemetry(t *schema.TelemetryConfig, r *Result) {
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[t.Level] {
		r.addf("telemetry.level",
			"must be debug/info/warn/error, got %q", t.Level)
	}
	validFormats := map[string]bool{"text": true, "json": true}
	if !validFormats[t.Format] {
		r.addf("telemetry.format", "must be text or json, got %q", t.Format)
	}
	if t.MetricsAddr != "" {
		if err := requireLoopback("telemetry.metrics_addr", t.MetricsAddr); err != nil {
			r.add("telemetry.metrics_addr", err.Error())
		}
	}
}

func validateAPI(a *schema.APIConfig, r *Result) {
	if !a.Enabled {
		return
	}
	if err := requireLoopback("api.addr", a.Addr); err != nil {
		r.add("api.addr", err.Error())
	}
	if a.ReadTimeout <= 0 {
		r.add("api.read_timeout", "must be > 0")
	}
	if a.WriteTimeout <= 0 {
		r.add("api.write_timeout", "must be > 0")
	}
}

// requireLoopback returns an error if addr does not resolve to a loopback IP.
// This enforces that no sensitive endpoints are accidentally exposed on public
// interfaces.
func requireLoopback(field, addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid address %q: %w", addr, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("address %q: host %q is not a valid IP — use 127.0.0.1", addr, host)
	}
	if !ip.IsLoopback() {
		return fmt.Errorf("address %q must bind to a loopback interface (127.x.x.x or ::1)", addr)
	}
	return nil
}

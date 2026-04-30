package loader

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"ghost-silicon/internal/config/schema"
)

// ApplyEnv reads GS_* environment variables and overlays them onto cfg.
// Only explicitly set variables override the existing config value.
//
// Supported variables (all prefixed GS_):
//
//	GS_DATA_DIR              → app.data_dir
//	GS_ENGINE_EXECUTABLE     → engine.executable
//	GS_PROFILE_DIR           → identity.profile_dir
//	GS_DEFAULT_PROFILE       → identity.default_profile
//	GS_LOG_LEVEL             → telemetry.level
//	GS_LOG_FORMAT            → telemetry.format
//	GS_LOG_FILE              → telemetry.log_file
//	GS_AUDIT_FILE            → telemetry.audit_file
//	GS_API_ENABLED           → api.enabled (true/false)
//	GS_API_ADDR              → api.addr
//	GS_PROXY_URL             → network.proxy_url
//	GS_PIPE_NAME             → ipc.pipe_name
//	GS_INTEGRITY_LEVEL       → sandbox.integrity_level
//	GS_JOB_OBJECT            → sandbox.enable_job_object (true/false)
//	GS_RESTRICTED_TOKEN      → sandbox.enable_restricted_token (true/false)
//	GS_MEMORY_LIMIT_MB       → sandbox.memory_limit_mb
//	GS_SHUTDOWN_TIMEOUT      → app.shutdown_timeout (e.g. "15s")
func ApplyEnv(cfg *schema.Config) error {
	if v := env("GS_DATA_DIR"); v != "" {
		cfg.App.DataDir = v
	}
	if v := env("GS_ENGINE_EXECUTABLE"); v != "" {
		cfg.Engine.Executable = v
	}
	if v := env("GS_PROFILE_DIR"); v != "" {
		cfg.Identity.ProfileDir = v
	}
	if v := env("GS_DEFAULT_PROFILE"); v != "" {
		cfg.Identity.DefaultProfile = v
	}
	if v := env("GS_LOG_LEVEL"); v != "" {
		cfg.Telemetry.Level = v
	}
	if v := env("GS_LOG_FORMAT"); v != "" {
		cfg.Telemetry.Format = v
	}
	if v := env("GS_LOG_FILE"); v != "" {
		cfg.Telemetry.LogFile = v
	}
	if v := env("GS_AUDIT_FILE"); v != "" {
		cfg.Telemetry.AuditFile = v
	}
	if v := env("GS_API_ADDR"); v != "" {
		cfg.API.Addr = v
	}
	if v := env("GS_PROXY_URL"); v != "" {
		cfg.Network.ProxyURL = v
	}
	if v := env("GS_PIPE_NAME"); v != "" {
		cfg.IPC.PipeName = v
	}
	if v := env("GS_INTEGRITY_LEVEL"); v != "" {
		cfg.Sandbox.IntegrityLevel = v
	}

	// Boolean overrides
	if err := applyBool("GS_API_ENABLED", &cfg.API.Enabled); err != nil {
		return err
	}
	if err := applyBool("GS_JOB_OBJECT", &cfg.Sandbox.EnableJobObject); err != nil {
		return err
	}
	if err := applyBool("GS_RESTRICTED_TOKEN", &cfg.Sandbox.EnableRestrictedToken); err != nil {
		return err
	}

	// Integer overrides
	if err := applyInt64("GS_MEMORY_LIMIT_MB", &cfg.Sandbox.MemoryLimitMB); err != nil {
		return err
	}

	// Duration overrides
	if err := applyDuration("GS_SHUTDOWN_TIMEOUT", &cfg.App.ShutdownTimeout); err != nil {
		return err
	}

	return nil
}

// env returns the trimmed value of the env var, or "" if unset or blank.
func env(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func applyBool(key string, dst *bool) error {
	v := env(key)
	if v == "" {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fmt.Errorf("env %s=%q: expected true/false: %w", key, v, err)
	}
	*dst = b
	return nil
}

func applyInt64(key string, dst *int64) error {
	v := env(key)
	if v == "" {
		return nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fmt.Errorf("env %s=%q: expected integer: %w", key, v, err)
	}
	*dst = n
	return nil
}

func applyDuration(key string, dst *time.Duration) error {
	v := env(key)
	if v == "" {
		return nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("env %s=%q: expected duration (e.g. 15s, 1m): %w", key, v, err)
	}
	*dst = d
	return nil
}

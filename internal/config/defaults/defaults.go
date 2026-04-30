// Package defaults provides the baseline Config that is used when fields are
// not overridden by YAML files or environment variables.
package defaults

import (
	"os"
	"path/filepath"
	"time"

	"ghost-silicon/internal/config/schema"
)

// Config returns the default configuration for a Windows 11 deployment.
// All values are conservative and safe — no permissive defaults.
func Config() *schema.Config {
	return &schema.Config{
		App: schema.AppConfig{
			Name:            "ghost-silicon",
			DataDir:         defaultDataDir(),
			ShutdownTimeout: 15 * time.Second,
		},

		Engine: schema.EngineConfig{
			Executable:        "",
			StartTimeout:      30 * time.Second,
			CrashRestartLimit: 3,
			CrashRestartDelay: 2 * time.Second,
		},

		Identity: schema.IdentityConfig{
			ProfileDir:      "", // resolved to DataDir/profiles at startup
			AutoGenerate:    true,
			DefaultTemplate: "windows11-desktop",
			Rotation: schema.RotationConfig{
				Trigger: "never",
			},
		},

		Network: schema.NetworkConfig{
			DialTimeout:           10 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConnsPerHost:   10,
			Policy:                schema.NetworkPolicyConfig{},
		},

		Sandbox: schema.SandboxConfig{
			EnableJobObject:       true,
			EnableRestrictedToken: true,
			IntegrityLevel:        string(schema.IntegrityMedium),
			EnableAppContainer:    false,
			MemoryLimitMB:         0,
			CPURatePercent:        0,
		},

		IPC: schema.IPCConfig{
			PipeName:        `\\.\pipe\ghost-silicon-bridge`,
			EnableWebSocket: false,
			WebSocketAddr:   "127.0.0.1:9222",
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
		},

		Storage: schema.StorageConfig{
			EncryptAtRest:     false,
			MaxSessionAgeDays: 0,
		},

		Telemetry: schema.TelemetryConfig{
			Level:  string(schema.LogLevelInfo),
			Format: string(schema.LogFormatText),
		},

		API: schema.APIConfig{
			Enabled:      false,
			Addr:         "127.0.0.1:9223",
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
	}
}

// defaultDataDir returns the platform-appropriate data directory.
// On Windows: %APPDATA%\ghost-silicon
// Fallback:   $HOME/.ghost-silicon
func defaultDataDir() string {
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "ghost-silicon")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".ghost-silicon")
	}
	return filepath.Join(".", "data")
}

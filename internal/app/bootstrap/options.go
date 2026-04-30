// internal/app/bootstrap/options.go
// Package bootstrap — startup options passed to Bootstrap.
package bootstrap

import (
	"ghost-silicon/internal/config/schema"
	"ghost-silicon/internal/telemetry/audit"
	"ghost-silicon/internal/telemetry/logging"
)

// Options carries everything Bootstrap.Run needs to wire the application.
type Options struct {
	// ConfigPath is the primary YAML config file path.
	// Empty means load defaults + env overrides only.
	ConfigPath string

	// ExtraConfigs are additional YAML overlay files applied after ConfigPath.
	ExtraConfigs []string

	// OverrideConfig optionally injects a pre-built Config, bypassing file loading.
	// Used in tests.
	OverrideConfig *schema.Config

	// Log is a pre-built logger. If nil, one is created from config.
	Log *logging.Logger

	// Auditor is a pre-built audit logger. If nil, one is created from config.
	Auditor *audit.Logger
}

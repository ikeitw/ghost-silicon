// internal/app/bootstrap/bootstrap.go
// Package bootstrap wires every subsystem together and hands a fully
// initialised App to the caller.  It is the only place that knows about
// all packages at once.
package bootstrap

import (
	"fmt"
	"path/filepath"

	"ghost-silicon/internal/config/loader"
	"ghost-silicon/internal/config/schema"
	"ghost-silicon/internal/config/validation"
	"ghost-silicon/internal/telemetry/audit"
	"ghost-silicon/internal/telemetry/logging"
)

// App is the fully wired application ready to run.
type App struct {
	Config  *schema.Config
	Log     *logging.Logger
	Auditor *audit.Logger
}

// Run loads configuration, validates it, wires telemetry, and returns an App.
func Run(opts Options) (*App, error) {
	// ── 1. Load configuration ────────────────────────────────────────────
	var cfg *schema.Config
	var err error

	if opts.OverrideConfig != nil {
		cfg = opts.OverrideConfig
	} else {
		paths := []string{opts.ConfigPath}
		paths = append(paths, opts.ExtraConfigs...)
		cfg, err = loader.LoadMultiple(paths...)
		if err != nil {
			return nil, fmt.Errorf("bootstrap: load config: %w", err)
		}
	}

	// ── 2. Validate configuration ────────────────────────────────────────
	result := validation.Validate(cfg)
	if !result.Valid() {
		return nil, fmt.Errorf("bootstrap: invalid config:\n%s", result.Error())
	}

	// ── 3. Wire telemetry ────────────────────────────────────────────────
	log := opts.Log
	if log == nil {
		log, err = logging.New(&logging.Options{
			Level:      cfg.Telemetry.Level,
			Format:     cfg.Telemetry.Format,
			OutputFile: cfg.Telemetry.LogFile,
		})
		if err != nil {
			return nil, fmt.Errorf("bootstrap: init logger: %w", err)
		}
	}

	auditor := opts.Auditor
	if auditor == nil {
		if cfg.Telemetry.AuditFile != "" {
			a, closer, err := audit.NewLoggerFromPath(cfg.Telemetry.AuditFile)
			if err != nil {
				return nil, fmt.Errorf("bootstrap: init audit logger: %w", err)
			}
			_ = closer // lifecycle managed by App shutdown
			auditor = a
		} else {
			auditor = audit.New(log)
		}
	}

	// ── 4. Resolve late-bound config paths ───────────────────────────────
	if cfg.Identity.ProfileDir == "" {
		cfg.Identity.ProfileDir = filepath.Join(cfg.App.DataDir, "profiles")
	}
	if cfg.Storage.BaseDir == "" {
		cfg.Storage.BaseDir = filepath.Join(cfg.App.DataDir, "sessions")
	}

	log.Info("ghost-silicon starting",
		"data_dir", cfg.App.DataDir,
		"engine", cfg.Engine.Executable,
		"pipe", cfg.IPC.PipeName,
	)

	return &App{
		Config:  cfg,
		Log:     log,
		Auditor: auditor,
	}, nil
}

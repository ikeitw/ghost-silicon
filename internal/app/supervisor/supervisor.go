// internal/app/supervisor/supervisor.go
// Package supervisor is the top-level orchestrator that owns the session,
// engine runtime, IPC bridge, and network layer for one browser instance.
package supervisor

import (
	"context"
	"fmt"

	"ghost-silicon/internal/app/lifecycle"
	"ghost-silicon/internal/config/schema"
	"ghost-silicon/internal/engine/runtime"
	"ghost-silicon/internal/ipc/jsonrpc"
	"ghost-silicon/internal/telemetry/audit"
	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/bridge"
	"ghost-silicon/pkg/identity"
)

// Supervisor owns and coordinates all subsystems for one browser session.
type Supervisor struct {
	cfg       *schema.Config
	log       *logging.Logger
	auditor   *audit.Logger
	lc        *lifecycle.Manager
	profile   *identity.Profile
	store     identity.Store
	bridge    *bridge.Bridge
	rpcSrv    *jsonrpc.Server
	rt        *runtime.Runtime
	sessionID string
}

// New creates a Supervisor from the bootstrapped application config.
func New(
	cfg *schema.Config,
	log *logging.Logger,
	auditor *audit.Logger,
	store identity.Store,
) *Supervisor {
	return &Supervisor{
		cfg:     cfg,
		log:     log.WithComponent("supervisor"),
		auditor: auditor,
		lc:      lifecycle.NewManager(log),
		store:   store,
	}
}

// Init loads the profile, creates the session ID, wires the bridge and RPC
// server, and registers all lifecycle hooks. Call before Run.
func (s *Supervisor) Init(ctx context.Context) error {
	// Load or generate the active profile.
	if err := s.loadProfile(ctx); err != nil {
		return fmt.Errorf("supervisor: load profile: %w", err)
	}

	// Create session ID.
	s.sessionID = newSessionID()

	// Build the IPC bridge backed by the loaded profile.
	s.bridge = bridge.New(s.profile, s.sessionID, s.log, s.auditor)
	s.rpcSrv = jsonrpc.NewServer(s.log)
	s.bridge.Register(s.rpcSrv)

	s.log.Info("supervisor initialised",
		logging.FieldProfileID, s.profile.ID,
		logging.FieldProfileName, s.profile.Name,
		logging.FieldSessionID, s.sessionID,
	)

	s.auditor.Info(audit.EventSessionStarted, "supervisor",
		"session_id", s.sessionID,
		"profile_id", s.profile.ID,
	)

	return nil
}

// Run starts all lifecycle hooks and blocks until ctx is cancelled.
func (s *Supervisor) Run(ctx context.Context) error {
	if err := s.lc.Start(ctx); err != nil {
		return fmt.Errorf("supervisor: startup: %w", err)
	}

	<-ctx.Done()

	shutCtx, cancel := context.WithTimeout(context.Background(), s.cfg.App.ShutdownTimeout)
	defer cancel()

	if err := s.lc.Stop(shutCtx); err != nil {
		s.log.Warn("shutdown error", logging.FieldError, err.Error())
	}

	s.auditor.Info(audit.EventSessionStopped, "supervisor",
		"session_id", s.sessionID,
	)
	return nil
}

// Profile returns the currently active identity profile.
func (s *Supervisor) Profile() *identity.Profile { return s.profile }

// SessionID returns the current session identifier.
func (s *Supervisor) SessionID() string { return s.sessionID }

// ── internal helpers ─────────────────────────────────────────────────────────

func (s *Supervisor) loadProfile(_ context.Context) error {
	// Try loading the configured default profile first.
	if s.cfg.Identity.DefaultProfile != "" {
		p, err := s.store.Load(s.cfg.Identity.DefaultProfile)
		if err == nil {
			s.profile = p
			s.auditor.Info(audit.EventProfileLoaded, "supervisor",
				"profile_id", p.ID, "profile_name", p.Name,
			)
			return nil
		}
	}

	// Fall back to the first profile in the store.
	metas, err := s.store.List()
	if err == nil && len(metas) > 0 {
		p, err := s.store.Load(metas[0].ID)
		if err == nil {
			s.profile = p
			return nil
		}
	}

	// Auto-generate if configured.
	if s.cfg.Identity.AutoGenerate {
		tmpl := identity.TemplateName(s.cfg.Identity.DefaultTemplate)
		p := identity.FromTemplate(tmpl)
		if p == nil {
			p = identity.Windows11DesktopTemplate()
		}
		if err := s.store.Save(p); err != nil {
			s.log.Warn("could not persist auto-generated profile",
				logging.FieldError, err.Error())
		}
		s.profile = p
		s.auditor.Info(audit.EventProfileLoaded, "supervisor",
			"profile_id", p.ID, "auto_generated", "true",
		)
		return nil
	}

	return fmt.Errorf("no profiles found in %q and auto_generate is disabled",
		s.cfg.Identity.ProfileDir)
}

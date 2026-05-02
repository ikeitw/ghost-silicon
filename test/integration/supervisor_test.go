// test/integration/supervisor_test.go
package integration_test

import (
	"context"
	"testing"
	"time"

	"ghost-silicon/internal/app/bootstrap"
	"ghost-silicon/internal/app/supervisor"
	"ghost-silicon/internal/config/defaults"
	"ghost-silicon/pkg/storage"
)

func TestSupervisor_InitWithAutoGenerate(t *testing.T) {
	dir := t.TempDir()
	cfg := defaults.Config()
	cfg.App.DataDir = dir
	cfg.Identity.ProfileDir = dir + "/profiles"
	cfg.Storage.BaseDir = dir + "/sessions"
	cfg.Engine.Executable = "mock"
	cfg.Identity.AutoGenerate = true

	app, err := bootstrap.Run(bootstrap.Options{OverrideConfig: cfg})
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	profileStore, err := storage.NewProfileStore(cfg.Identity.ProfileDir)
	if err != nil {
		t.Fatalf("profile store: %v", err)
	}

	sup := supervisor.New(cfg, app.Log, app.Auditor, profileStore)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := sup.Init(ctx); err != nil {
		t.Fatalf("supervisor Init: %v", err)
	}

	if sup.Profile() == nil {
		t.Fatal("expected a profile after Init")
	}
	if sup.SessionID() == "" {
		t.Fatal("expected a non-empty session ID after Init")
	}
}

func TestSupervisor_ProfilePersisted(t *testing.T) {
	dir := t.TempDir()
	cfg := defaults.Config()
	cfg.App.DataDir = dir
	cfg.Identity.ProfileDir = dir + "/profiles"
	cfg.Storage.BaseDir = dir + "/sessions"
	cfg.Engine.Executable = "mock"
	cfg.Identity.AutoGenerate = true

	app, _ := bootstrap.Run(bootstrap.Options{OverrideConfig: cfg})
	profileStore, _ := storage.NewProfileStore(cfg.Identity.ProfileDir)
	sup := supervisor.New(cfg, app.Log, app.Auditor, profileStore)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_ = sup.Init(ctx)
	profileID := sup.Profile().ID

	// Re-open the store and verify the profile was saved.
	store2, _ := storage.NewProfileStore(cfg.Identity.ProfileDir)
	if !store2.Exists(profileID) {
		t.Errorf("auto-generated profile %q was not persisted to disk", profileID)
	}
}

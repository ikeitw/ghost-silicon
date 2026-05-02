// test/integration/config_test.go
package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"ghost-silicon/internal/config/defaults"
	"ghost-silicon/internal/config/loader"
	"ghost-silicon/internal/config/validation"
)

func TestDefaultConfig_IsValid(t *testing.T) {
	cfg := defaults.Config()
	// Set required engine executable so validation passes.
	cfg.Engine.Executable = `C:\renderer\chrome.exe`

	result := validation.Validate(cfg)
	if !result.Valid() {
		t.Fatalf("default config failed validation:\n%s", result.Error())
	}
}

func TestLoader_EnvOverride(t *testing.T) {
	t.Setenv("GS_LOG_LEVEL", "debug")
	t.Setenv("GS_LOG_FORMAT", "json")

	cfg, err := loader.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Telemetry.Level != "debug" {
		t.Errorf("expected level=debug, got %q", cfg.Telemetry.Level)
	}
	if cfg.Telemetry.Format != "json" {
		t.Errorf("expected format=json, got %q", cfg.Telemetry.Format)
	}
}

func TestLoader_YAMLFile(t *testing.T) {
	dir := t.TempDir()
	yaml := `
telemetry:
  level: warn
  format: json
engine:
  executable: "C:\\renderer\\chrome.exe"
  start_timeout: 45s
`
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := loader.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Telemetry.Level != "warn" {
		t.Errorf("expected level=warn, got %q", cfg.Telemetry.Level)
	}
	if cfg.Engine.Executable != `C:\renderer\chrome.exe` {
		t.Errorf("expected engine executable, got %q", cfg.Engine.Executable)
	}
}

func TestLoader_MultipleOverlays(t *testing.T) {
	dir := t.TempDir()

	base := `
telemetry:
  level: info
engine:
  executable: "C:\\renderer\\chrome.exe"
`
	overlay := `
telemetry:
  level: debug
`
	basePath := filepath.Join(dir, "base.yaml")
	overlayPath := filepath.Join(dir, "overlay.yaml")
	os.WriteFile(basePath, []byte(base), 0o600)
	os.WriteFile(overlayPath, []byte(overlay), 0o600)

	cfg, err := loader.LoadMultiple(basePath, overlayPath)
	if err != nil {
		t.Fatalf("LoadMultiple: %v", err)
	}
	if cfg.Telemetry.Level != "debug" {
		t.Errorf("expected overlay level=debug, got %q", cfg.Telemetry.Level)
	}
}

func TestValidation_MissingExecutable(t *testing.T) {
	cfg := defaults.Config()
	cfg.Engine.Executable = ""

	result := validation.Validate(cfg)
	if result.Valid() {
		t.Fatal("expected validation to fail for missing engine executable")
	}
}

func TestValidation_InvalidLogLevel(t *testing.T) {
	cfg := defaults.Config()
	cfg.Engine.Executable = `C:\chrome.exe`
	cfg.Telemetry.Level = "verbose"

	result := validation.Validate(cfg)
	if result.Valid() {
		t.Fatal("expected validation to fail for invalid log level")
	}
}

func TestValidation_NonLoopbackAPIAddr(t *testing.T) {
	cfg := defaults.Config()
	cfg.Engine.Executable = `C:\chrome.exe`
	cfg.API.Enabled = true
	cfg.API.Addr = "0.0.0.0:9223"

	result := validation.Validate(cfg)
	if result.Valid() {
		t.Fatal("expected validation to fail for non-loopback API address")
	}
}

// internal/policy/sandbox/policy.go
// Package sandbox defines the supervisor-level sandbox policy that controls
// Windows process isolation settings for the renderer.
package sandbox

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Policy holds all sandbox isolation settings.
type Policy struct {
	// EnableJobObject wraps the renderer in a Windows Job Object.
	EnableJobObject bool `yaml:"enable_job_object"`

	// EnableRestrictedToken runs the renderer with a reduced-privilege token.
	EnableRestrictedToken bool `yaml:"enable_restricted_token"`

	// IntegrityLevel is the mandatory integrity level: "low", "medium", "high".
	IntegrityLevel string `yaml:"integrity_level"`

	// EnableAppContainer enables AppContainer-style isolation (experimental).
	EnableAppContainer bool `yaml:"enable_app_container"`

	// MemoryLimitMB caps renderer memory via Job Object (0 = unlimited).
	MemoryLimitMB int64 `yaml:"memory_limit_mb"`

	// CPURatePercent caps renderer CPU via Job Object (0 = unlimited, 1–100).
	CPURatePercent int `yaml:"cpu_rate_percent"`
}

// DefaultPolicy returns conservative but functional defaults.
func DefaultPolicy() *Policy {
	return &Policy{
		EnableJobObject:       true,
		EnableRestrictedToken: true,
		IntegrityLevel:        "medium",
		EnableAppContainer:    false,
		MemoryLimitMB:         0,
		CPURatePercent:        0,
	}
}

// Validate returns an error when the policy has invalid field values.
func (p *Policy) Validate() error {
	switch p.IntegrityLevel {
	case "low", "medium", "high":
	default:
		return fmt.Errorf("sandbox/policy: integrity_level must be low/medium/high, got %q",
			p.IntegrityLevel)
	}
	if p.MemoryLimitMB < 0 {
		return fmt.Errorf("sandbox/policy: memory_limit_mb must be >= 0")
	}
	if p.CPURatePercent < 0 || p.CPURatePercent > 100 {
		return fmt.Errorf("sandbox/policy: cpu_rate_percent must be 0–100, got %d",
			p.CPURatePercent)
	}
	return nil
}

// LoadFromFile reads a sandbox-policy YAML file.
func LoadFromFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("sandbox/policy: read %q: %w", path, err)
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("sandbox/policy: parse %q: %w", path, err)
	}
	return &p, nil
}

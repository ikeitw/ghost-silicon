// internal/policy/storage/policy.go
// Package storage defines the supervisor-level storage policy engine.
// It controls which storage APIs are available to the renderer and enforces
// per-session quota limits.
package storage

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Policy describes which storage APIs are permitted and what quotas apply.
type Policy struct {
	EnableCookies        bool `yaml:"enable_cookies"`
	EnableLocalStorage   bool `yaml:"enable_local_storage"`
	EnableSessionStorage bool `yaml:"enable_session_storage"`
	EnableIndexedDB      bool `yaml:"enable_indexed_db"`
	EnableCacheStorage   bool `yaml:"enable_cache_storage"`
	EnableServiceWorker  bool `yaml:"enable_service_worker"`
	MaxCookieJarMB       int  `yaml:"max_cookie_jar_mb"`
	MaxStorageMB         int  `yaml:"max_storage_mb"`
}

// DefaultPolicy returns a policy that enables all standard storage APIs
// with no quota caps.
func DefaultPolicy() *Policy {
	return &Policy{
		EnableCookies:        true,
		EnableLocalStorage:   true,
		EnableSessionStorage: true,
		EnableIndexedDB:      true,
		EnableCacheStorage:   true,
		EnableServiceWorker:  true,
	}
}

// HardenedPolicy returns a policy that disables persistent storage APIs.
func HardenedPolicy() *Policy {
	return &Policy{
		EnableCookies:        true,
		EnableLocalStorage:   false,
		EnableSessionStorage: true,
		EnableIndexedDB:      false,
		EnableCacheStorage:   false,
		EnableServiceWorker:  false,
		MaxCookieJarMB:       10,
		MaxStorageMB:         50,
	}
}

// LoadFromFile reads a storage policy from a YAML file.
func LoadFromFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("storage/policy: read %q: %w", path, err)
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("storage/policy: parse %q: %w", path, err)
	}
	return &p, nil
}

// Package loader reads ghost-silicon configuration from YAML files and
// environment variable overrides, merging them onto the default Config.
package loader

import (
	"fmt"
	"os"

	"ghost-silicon/internal/config/defaults"
	"ghost-silicon/internal/config/schema"

	"gopkg.in/yaml.v3"
)

// Load reads the primary config file at path, merges it onto the defaults,
// then applies environment variable overrides.
//
// If path is empty the default config is returned with env overrides applied.
func Load(path string) (*schema.Config, error) {
	cfg := defaults.Config()

	if path != "" {
		if err := mergeYAMLFile(cfg, path); err != nil {
			return nil, fmt.Errorf("loader: primary config %q: %w", path, err)
		}
	}

	// Apply environment variable overrides on top of file config.
	if err := ApplyEnv(cfg); err != nil {
		return nil, fmt.Errorf("loader: env overrides: %w", err)
	}

	return cfg, nil
}

// LoadMultiple loads a base config and applies one or more overlay files in
// order.  Later files override earlier ones.  Env overrides are applied last.
func LoadMultiple(paths ...string) (*schema.Config, error) {
	cfg := defaults.Config()

	for _, p := range paths {
		if p == "" {
			continue
		}
		if err := mergeYAMLFile(cfg, p); err != nil {
			return nil, fmt.Errorf("loader: config %q: %w", p, err)
		}
	}

	if err := ApplyEnv(cfg); err != nil {
		return nil, fmt.Errorf("loader: env overrides: %w", err)
	}

	return cfg, nil
}

// mergeYAMLFile unmarshals a YAML file directly into cfg using yaml.Decoder
// with KnownFields(true) so that typos in field names are caught early.
func mergeYAMLFile(cfg *schema.Config, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	if err := dec.Decode(cfg); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	return nil
}

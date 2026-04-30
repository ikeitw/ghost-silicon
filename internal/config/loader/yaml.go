package loader

import (
	"fmt"
	"os"

	"ghost-silicon/internal/config/schema"

	"gopkg.in/yaml.v3"
)

// ParseYAML decodes a YAML byte slice directly into a Config.
// Useful in tests where you don't want to hit the file system.
func ParseYAML(data []byte) (*schema.Config, error) {
	cfg := &schema.Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("loader/yaml: unmarshal: %w", err)
	}
	return cfg, nil
}

// WriteYAML serialises cfg to a YAML file at path.
// The file is created or truncated.  Parent directories must exist.
func WriteYAML(cfg *schema.Config, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("loader/yaml: create %q: %w", path, err)
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("loader/yaml: encode: %w", err)
	}
	return nil
}

// internal/policy/network/rules.go
// Package network — rule helpers and YAML-loadable rule set.
package network

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// RuleSet is the on-disk YAML format for network-policy.yaml.
type RuleSet struct {
	Rules         []Rule   `yaml:"rules"`
	DefaultAction Decision `yaml:"default_action"`
}

// LoadFromFile reads a network-policy YAML file and returns a Policy.
func LoadFromFile(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("network/rules: read %q: %w", path, err)
	}
	var rs RuleSet
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("network/rules: parse %q: %w", path, err)
	}
	if rs.DefaultAction == "" {
		rs.DefaultAction = DecisionAllow
	}
	return &Policy{Rules: rs.Rules, DefaultAction: rs.DefaultAction}, nil
}

// MustAllow appends an allow rule for the given host pattern.
func (p *Policy) MustAllow(host string) {
	p.Rules = append([]Rule{{Host: host, Action: DecisionAllow}}, p.Rules...)
}

// MustDeny prepends a deny rule for the given host pattern so it takes
// priority over any existing allow rules.
func (p *Policy) MustDeny(host string) {
	p.Rules = append([]Rule{{Host: host, Action: DecisionDeny}}, p.Rules...)
}

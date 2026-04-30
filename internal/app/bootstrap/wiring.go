// internal/app/bootstrap/wiring.go
// Package bootstrap — dependency wiring helpers.
// Provides thin constructors that combine multiple packages so bootstrap.go
// stays readable rather than a wall of initialisation code.
package bootstrap

import (
	"fmt"

	"ghost-silicon/internal/config/schema"
	"ghost-silicon/internal/telemetry/audit"
	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/network"
)

// BuildTransport constructs the policy-aware HTTP transport from cfg.
func BuildTransport(cfg *schema.Config, log *logging.Logger) (*network.Transport, error) {
	proxyFn, err := network.BuildProxyFunc(cfg.Network.ProxyURL, false)
	if err != nil {
		return nil, fmt.Errorf("wiring: proxy: %w", err)
	}

	policy := network.NewPolicy(
		cfg.Network.Policy.AllowedHosts,
		cfg.Network.Policy.BlockedHosts,
		cfg.Network.Policy.AllowedPorts,
	)

	resolver := network.NewResolver(cfg.Network.DNSServers)

	dialer := network.NewDialer(network.DialerOptions{
		DialTimeout:           cfg.Network.DialTimeout,
		TLSHandshakeTimeout:   cfg.Network.TLSHandshakeTimeout,
		ResponseHeaderTimeout: cfg.Network.ResponseHeaderTimeout,
		Resolver:              resolver,
	})

	pool := network.FromConfig(cfg.Network.MaxIdleConnsPerHost, cfg.Network.DialTimeout)

	t, err := network.NewTransport(network.TransportOptions{
		Policy:    policy,
		Headers:   network.DefaultHeaderPolicy(),
		Dialer:    dialer,
		TLSPolicy: network.DefaultTLSPolicy(),
		Pool:      pool,
		ProxyFunc: proxyFn,
	})
	if err != nil {
		return nil, fmt.Errorf("wiring: transport: %w", err)
	}

	log.Info("network transport ready",
		"proxy", cfg.Network.ProxyURL,
		"dns_servers", cfg.Network.DNSServers,
	)
	return t, nil
}

// LogAuditEvent emits an application-start audit event.
func LogAuditEvent(a *audit.Logger, cfg *schema.Config) {
	a.Info(audit.EventConfigLoaded, "bootstrap",
		"data_dir", cfg.App.DataDir,
		"engine", cfg.Engine.Executable,
	)
}

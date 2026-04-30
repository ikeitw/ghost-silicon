// pkg/network/windows_proxy.go
//go:build windows

// Package network — Windows system proxy reader.
// Reads the current proxy configuration from the Windows registry so that
// the supervisor can default to the same proxy the user has configured
// in Windows Settings → Network → Proxy.
package network

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const (
	internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
)

// WindowsProxySettings holds the proxy configuration read from the registry.
type WindowsProxySettings struct {
	// ProxyEnabled is true when a proxy is configured in Windows Settings.
	ProxyEnabled bool

	// ProxyServer is the configured proxy address, e.g. "127.0.0.1:8080".
	ProxyServer string

	// ProxyOverride is the bypass list, e.g. "<local>;*.example.com".
	ProxyOverride string
}

// ReadWindowsProxySettings reads the current user's proxy settings from the
// Windows Internet Settings registry key.
func ReadWindowsProxySettings() (*WindowsProxySettings, error) {
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		internetSettingsKey,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return nil, fmt.Errorf("windows_proxy: open registry key: %w", err)
	}
	defer k.Close() //nolint:errcheck

	enabled, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil {
		// Key may not exist if no proxy is configured.
		return &WindowsProxySettings{ProxyEnabled: false}, nil
	}

	server, _, _ := k.GetStringValue("ProxyServer")
	override, _, _ := k.GetStringValue("ProxyOverride")

	return &WindowsProxySettings{
		ProxyEnabled:  enabled != 0,
		ProxyServer:   server,
		ProxyOverride: override,
	}, nil
}

// ProxyURL returns a proxy URL string suitable for FixedProxy(), or ""
// if the system proxy is disabled or not configured.
func (s *WindowsProxySettings) ProxyURL() string {
	if !s.ProxyEnabled || s.ProxyServer == "" {
		return ""
	}
	// Windows stores the proxy without a scheme; default to http.
	if len(s.ProxyServer) > 4 && s.ProxyServer[:4] == "http" {
		return s.ProxyServer
	}
	return "http://" + s.ProxyServer
}

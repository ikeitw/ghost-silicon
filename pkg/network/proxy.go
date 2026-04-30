// pkg/network/proxy.go
// Package network — proxy support.
// Resolves the upstream proxy for a given request URL.
// Supports HTTP, HTTPS, and SOCKS5 proxies from the profile or config.
package network

import (
	"fmt"
	"net/http"
	"net/url"
)

// ProxyFunc is the function signature expected by http.Transport.Proxy.
type ProxyFunc func(*http.Request) (*url.URL, error)

// NoProxy returns nil for every request (direct connection).
func NoProxy() ProxyFunc {
	return func(_ *http.Request) (*url.URL, error) { return nil, nil }
}

// FixedProxy returns a ProxyFunc that routes all requests through proxyURL.
// proxyURL must be a valid http, https, or socks5 URL string.
func FixedProxy(proxyURL string) (ProxyFunc, error) {
	if proxyURL == "" {
		return NoProxy(), nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("network/proxy: parse %q: %w", proxyURL, err)
	}
	switch u.Scheme {
	case "http", "https", "socks5":
	default:
		return nil, fmt.Errorf("network/proxy: unsupported scheme %q (use http, https, socks5)", u.Scheme)
	}
	return func(_ *http.Request) (*url.URL, error) {
		return u, nil
	}, nil
}

// FromEnvironment returns a ProxyFunc backed by the standard HTTP_PROXY /
// HTTPS_PROXY / NO_PROXY environment variables.
func FromEnvironment() ProxyFunc {
	return http.ProxyFromEnvironment
}

// BuildProxyFunc selects the right ProxyFunc from the network profile.
// Priority: profile.ProxyURL → environment → no proxy.
func BuildProxyFunc(profileProxyURL string, useEnv bool) (ProxyFunc, error) {
	if profileProxyURL != "" {
		return FixedProxy(profileProxyURL)
	}
	if useEnv {
		return FromEnvironment(), nil
	}
	return NoProxy(), nil
}

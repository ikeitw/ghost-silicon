# Network Layer

Ghost-Silicon replaces the renderer's default network stack with a
policy-aware custom transport built on Go's `net/http` package.

## Components

### Transport (`pkg/network/transport.go`)

A `http.RoundTripper` that:
1. Checks the request against the network ACL policy
2. Clones the request and applies the header policy
3. Forwards to an inner `*http.Transport` with custom dialer and TLS config

### Dialer (`pkg/network/dialer.go`)

Wraps `net.Dialer` with:
- Configurable TCP connect timeout
- Optional custom DNS resolver
- Keep-alive interval

### Resolver (`pkg/network/resolver.go`)

Wraps `net.Resolver` to allow overriding the system DNS with configured
nameservers. Useful for routing all DNS through a trusted resolver.

### Proxy (`pkg/network/proxy.go`)

Supports three proxy schemes:
- `http://` — HTTP CONNECT proxy
- `https://` — HTTPS proxy
- `socks5://` — SOCKS5 proxy

Priority: profile `network.proxy_url` → environment variables → no proxy.

### Header Policy (`pkg/network/headers.go`)

Applied to every outbound request:
- **Strip** internal ghost-silicon headers (`X-GS-Session-ID`, `X-GS-Profile-ID`)
- **Inject** `Accept-Language` from the profile's language list
- **Set-if-missing** any configured default headers

### TLS Policy (`pkg/network/tls_policy.go`)

Enforces minimum TLS 1.2. `InsecureSkipVerify` is available for isolated
development environments but must never be enabled in production.

### Network ACL (`pkg/network/policy.go`)

Evaluates outbound requests against allowlist and blocklist rules:

```yaml
network:
  policy:
    blocked_hosts:
      - "*.google-analytics.com"
      - "*.doubleclick.net"
    allowed_hosts: []   # empty = allow all non-blocked
    allowed_ports: []   # empty = allow all ports
```

Wildcard prefix `*.` matches all subdomains.

### Connection Pool (`pkg/network/connection_pool.go`)

Configures `MaxIdleConns`, `MaxIdleConnsPerHost`, `MaxConnsPerHost`, and
`IdleConnTimeout` on the inner transport.

## Windows Integration

### System Proxy (`pkg/network/windows_proxy.go`)

On Windows, ghost-silicon reads the proxy configuration from:

```
HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings
  ProxyEnable    (DWORD)
  ProxyServer    (string)  e.g. "127.0.0.1:8080"
  ProxyOverride  (string)  e.g. "<local>"
```

Used as a fallback when no `network.proxy_url` is configured.

### System DNS (`pkg/network/windows_dns.go`)

Reads DNS servers from:

```
HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\*
  NameServer      (string)  static DNS
  DhcpNameServer  (string)  DHCP-assigned DNS
```

Used as a fallback when no `network.dns_servers` list is configured.

## Building the transport

```go
proxyFn, _ := network.BuildProxyFunc("socks5://127.0.0.1:1080", false)
policy      := network.NewPolicy(nil, []string{"*.ads.com"}, nil)
resolver    := network.NewResolver([]string{"1.1.1.1:53"})
dialer      := network.NewDialer(network.DialerOptions{Resolver: resolver})

tr, _ := network.NewTransport(network.TransportOptions{
    Policy:    policy,
    Dialer:    dialer,
    ProxyFunc: proxyFn,
})
client := tr.NewHTTPClient()
```
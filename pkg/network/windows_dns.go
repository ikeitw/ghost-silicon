// pkg/network/windows_dns.go
//go:build windows

// Package network — Windows system DNS reader.
// Reads the DNS servers configured in the active network adapter so that
// the supervisor can report them to the resolver when no profile DNS override
// is set.
package network

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	tcpipInterfacesKey = `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces`
)

// ReadWindowsDNSServers returns the DNS server addresses from the Windows
// TCP/IP registry for all active adapters.
// Returns an empty slice if nothing can be read.
func ReadWindowsDNSServers() ([]string, error) {
	k, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		tcpipInterfacesKey,
		registry.ENUMERATE_SUB_KEYS,
	)
	if err != nil {
		return nil, fmt.Errorf("windows_dns: open registry key: %w", err)
	}
	defer k.Close() //nolint:errcheck

	subkeys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil, fmt.Errorf("windows_dns: read subkeys: %w", err)
	}

	seen := make(map[string]struct{})
	var servers []string

	for _, name := range subkeys {
		sub, err := registry.OpenKey(k, name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}

		// NameServer is a static DNS list; DhcpNameServer is DHCP-assigned.
		for _, val := range []string{"NameServer", "DhcpNameServer"} {
			ns, _, err := sub.GetStringValue(val)
			if err != nil || strings.TrimSpace(ns) == "" {
				continue
			}
			for _, addr := range strings.FieldsFunc(ns, func(r rune) bool {
				return r == ',' || r == ' '
			}) {
				addr = strings.TrimSpace(addr)
				if addr == "" {
					continue
				}
				// Add port 53 if missing.
				if !strings.Contains(addr, ":") {
					addr = addr + ":53"
				}
				if _, dup := seen[addr]; !dup {
					seen[addr] = struct{}{}
					servers = append(servers, addr)
				}
			}
		}
		sub.Close() //nolint:errcheck
	}

	return servers, nil
}

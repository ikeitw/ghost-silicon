// tools/network-debugger/main.go
// Network Debugger — tests the ghost-silicon network transport layer.
// Usage: network-debugger <url> [--proxy socks5://...] [--block host]
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"ghost-silicon/pkg/network"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "network-debugger: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		proxyURL  = flag.String("proxy", "", "upstream proxy URL")
		blockHost = flag.String("block", "", "block this host via policy")
		timeout   = flag.Duration("timeout", 10*time.Second, "request timeout")
	)
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		return fmt.Errorf("usage: network-debugger <url>")
	}
	targetURL := args[0]

	// Build transport
	proxyFn, err := network.BuildProxyFunc(*proxyURL, false)
	if err != nil {
		return fmt.Errorf("proxy: %w", err)
	}

	var blockedHosts []string
	if *blockHost != "" {
		blockedHosts = []string{*blockHost}
	}
	policy := network.NewPolicy(nil, blockedHosts, nil)

	dialer := network.NewDialer(network.DialerOptions{
		DialTimeout:           *timeout,
		TLSHandshakeTimeout:   *timeout,
		ResponseHeaderTimeout: *timeout,
	})

	tr, err := network.NewTransport(network.TransportOptions{
		Policy:    policy,
		Dialer:    dialer,
		ProxyFunc: proxyFn,
	})
	if err != nil {
		return fmt.Errorf("build transport: %w", err)
	}

	client := tr.NewHTTPClient()
	client.Timeout = *timeout

	fmt.Printf("GET %s\n", targetURL)
	start := time.Now()

	resp, err := client.Get(targetURL)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("ERROR: %v (%.0fms)\n", err, float64(elapsed.Milliseconds()))
		return nil // don't exit 1 — just show what happened
	}
	defer resp.Body.Close()

	fmt.Printf("Status:  %s\n", resp.Status)
	fmt.Printf("Latency: %dms\n", elapsed.Milliseconds())
	fmt.Printf("Headers:\n")
	for k, vs := range resp.Header {
		for _, v := range vs {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}
	return nil
}

// examples/custom-network-policy/main.go
// Demonstrates building a custom network transport with ACL rules.
package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"ghost-silicon/pkg/network"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "custom-network-policy: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Start two local test servers — one allowed, one blocked.
	allowed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "hello from allowed server")
	}))
	defer allowed.Close()

	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "hello from blocked server")
	}))
	defer blocked.Close()

	// 2. Build a policy that blocks the blocked server's host.
	blockedHost := "127.0.0.1"
	policy := network.NewPolicy(nil, []string{blockedHost}, nil)

	// 3. Build the transport with the custom policy.
	tr, err := network.NewTransport(network.TransportOptions{Policy: policy})
	if err != nil {
		return fmt.Errorf("build transport: %w", err)
	}
	client := tr.NewHTTPClient()

	// 4. Try the blocked server — should fail.
	fmt.Printf("GET %s (blocked)... ", blocked.URL)
	_, err = client.Get(blocked.URL)
	if err != nil {
		fmt.Println("BLOCKED ✓")
	} else {
		fmt.Println("ALLOWED (unexpected!)")
	}

	// 5. Build an unrestricted transport for the allowed server.
	trOpen, _ := network.NewTransport(network.TransportOptions{})
	clientOpen := trOpen.NewHTTPClient()

	fmt.Printf("GET %s (allowed)... ", allowed.URL)
	resp, err := clientOpen.Get(allowed.URL)
	if err != nil {
		return fmt.Errorf("allowed request failed: %w", err)
	}
	defer resp.Body.Close()
	fmt.Printf("STATUS %d ✓\n", resp.StatusCode)

	return nil
}

// test/network/dns_test.go
package network_test

import (
	"context"
	"testing"

	"ghost-silicon/pkg/network"
)

func TestResolver_DefaultSystemResolver(t *testing.T) {
	r := network.NewResolver(nil)
	if r == nil {
		t.Fatal("NewResolver(nil) returned nil")
	}
	// Resolve a well-known stable hostname using the system resolver.
	addrs, err := r.LookupHost(context.Background(), "localhost")
	if err != nil {
		t.Fatalf("LookupHost localhost: %v", err)
	}
	if len(addrs) == 0 {
		t.Error("expected at least one address for localhost")
	}
}

func TestResolver_EmptyServersUsesSystem(t *testing.T) {
	r := network.NewResolver([]string{})
	addrs, err := r.LookupHost(context.Background(), "localhost")
	if err != nil {
		t.Fatalf("LookupHost: %v", err)
	}
	if len(addrs) == 0 {
		t.Error("expected addresses for localhost with empty server list")
	}
}

func TestResolver_ServersListPreserved(t *testing.T) {
	servers := []string{"1.1.1.1:53", "8.8.8.8:53"}
	r := network.NewResolver(servers)
	got := r.Servers()
	if len(got) != len(servers) {
		t.Errorf("expected %d servers, got %d", len(servers), len(got))
	}
}

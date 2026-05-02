// test/network/dialer_test.go
package network_test

import (
	"context"
	"net"
	"testing"
	"time"

	"ghost-silicon/pkg/network"
)

func TestDialer_ConnectsToLocalServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	dialer := network.NewDialer(network.DialerOptions{
		DialTimeout: 5 * time.Second,
	})

	ctx := context.Background()
	conn, err := dialer.DialContext(ctx, "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	defer conn.Close()
}

func TestDialer_RejectsUnsupportedNetwork(t *testing.T) {
	dialer := network.NewDialer(network.DialerOptions{})
	_, err := dialer.DialContext(context.Background(), "udp", "127.0.0.1:53")
	if err == nil {
		t.Fatal("expected error for unsupported network 'udp'")
	}
}

func TestDialer_DefaultTimeouts(t *testing.T) {
	// Verify that a zero-value DialerOptions produces a working dialer.
	dialer := network.NewDialer(network.DialerOptions{})
	if dialer == nil {
		t.Fatal("NewDialer returned nil")
	}
}

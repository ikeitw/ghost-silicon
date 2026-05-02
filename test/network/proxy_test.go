// test/network/proxy_test.go
package network_test

import (
	"net/http"
	"testing"

	"ghost-silicon/pkg/network"
)

func TestFixedProxy_ValidHTTP(t *testing.T) {
	fn, err := network.FixedProxy("http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("FixedProxy: %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	u, err := fn(req)
	if err != nil {
		t.Fatalf("proxy func: %v", err)
	}
	if u == nil {
		t.Fatal("expected non-nil proxy URL")
	}
	if u.Host != "127.0.0.1:8080" {
		t.Errorf("expected proxy host 127.0.0.1:8080, got %s", u.Host)
	}
}

func TestFixedProxy_ValidSOCKS5(t *testing.T) {
	fn, err := network.FixedProxy("socks5://127.0.0.1:1080")
	if err != nil {
		t.Fatalf("FixedProxy socks5: %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	u, _ := fn(req)
	if u.Scheme != "socks5" {
		t.Errorf("expected scheme socks5, got %s", u.Scheme)
	}
}

func TestFixedProxy_InvalidScheme(t *testing.T) {
	_, err := network.FixedProxy("ftp://proxy.example.com")
	if err == nil {
		t.Fatal("expected error for unsupported proxy scheme ftp://")
	}
}

func TestFixedProxy_Empty(t *testing.T) {
	fn, err := network.FixedProxy("")
	if err != nil {
		t.Fatalf("FixedProxy empty: %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	u, _ := fn(req)
	if u != nil {
		t.Errorf("expected nil proxy URL for empty string, got %v", u)
	}
}

func TestBuildProxyFunc_ProfileOverEnv(t *testing.T) {
	fn, err := network.BuildProxyFunc("http://127.0.0.1:9999", true)
	if err != nil {
		t.Fatalf("BuildProxyFunc: %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	u, _ := fn(req)
	if u == nil || u.Host != "127.0.0.1:9999" {
		t.Errorf("expected profile proxy to take precedence, got %v", u)
	}
}

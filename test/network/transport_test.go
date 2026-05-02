// test/network/transport_test.go
package network_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ghost-silicon/pkg/network"
)

func newTestTransport(t *testing.T) *network.Transport {
	t.Helper()
	tr, err := network.NewTransport(network.TransportOptions{})
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	return tr
}

func TestTransport_AllowsNormalRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tr := newTestTransport(t)
	client := tr.NewHTTPClient()

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET %s: %v", srv.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestTransport_BlockedHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host := "127.0.0.1"
	policy := network.NewPolicy(nil, []string{host}, nil)

	tr, err := network.NewTransport(network.TransportOptions{Policy: policy})
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	client := tr.NewHTTPClient()

	_, err = client.Get(srv.URL)
	if err == nil {
		t.Fatal("expected error for blocked host, got nil")
	}
}

func TestTransport_HeadersInjected(t *testing.T) {
	var gotLang string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLang = r.Header.Get("Accept-Language")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	headers := network.ProfileHeaderPolicy([]string{"fr-FR", "fr", "en"}, "Europe/Paris")
	tr, err := network.NewTransport(network.TransportOptions{Headers: headers})
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	client := tr.NewHTTPClient()

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if gotLang == "" {
		t.Error("expected Accept-Language header to be set")
	}
}

func TestTransport_StripInternalHeaders(t *testing.T) {
	var gotSessionID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSessionID = r.Header.Get("X-GS-Session-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tr := newTestTransport(t)
	client := tr.NewHTTPClient()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("X-GS-Session-ID", "secret-session-id")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if gotSessionID != "" {
		t.Errorf("internal header X-GS-Session-ID leaked to server: %q", gotSessionID)
	}
}

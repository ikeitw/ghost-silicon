// pkg/api/network.go
// Package api — network status endpoint handler.
package api

import (
	"net/http"
	"runtime"
)

// NetworkHandler handles all /network routes.
type NetworkHandler struct {
	ProxyURL   string
	DNSServers []string
}

// NetworkStatusResponse is the JSON body for GET /network/status.
type NetworkStatusResponse struct {
	ProxyURL   string   `json:"proxy_url,omitempty"`
	DNSServers []string `json:"dns_servers,omitempty"`
	Platform   string   `json:"platform"`
}

// Status handles GET /network/status.
func (h NetworkHandler) Status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, &NetworkStatusResponse{
		ProxyURL:   h.ProxyURL,
		DNSServers: h.DNSServers,
		Platform:   runtime.GOOS,
	})
}

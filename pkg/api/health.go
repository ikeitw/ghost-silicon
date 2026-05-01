// pkg/api/health.go
// Package api — health endpoint handler.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"ghost-silicon/pkg/version"
)

// HealthHandler handles GET /health.
type HealthHandler struct{}

// HealthResponse is the JSON body returned by GET /health.
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// GetHealth writes a 200 OK with basic supervisor status.
func (h HealthHandler) GetHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, &HealthResponse{
		Status:    "ok",
		Version:   version.Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// writeJSON serialises v to JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error body.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// pkg/api/routes.go
// Package api — route registration.
// Mounts all API endpoint handlers onto the ServeMux.
package api

import (
	"net/http"
)

// Deps holds the dependencies injected into API handlers.
type Deps struct {
	Profiles ProfilesHandler
	Sessions SessionsHandler
	Network  NetworkHandler
	Health   HealthHandler
}

// registerRoutes mounts all routes onto mux.
func registerRoutes(mux *http.ServeMux, deps *Deps) {
	// Health
	mux.HandleFunc("GET /health", deps.Health.GetHealth)

	// Profiles
	mux.HandleFunc("GET /profiles", deps.Profiles.List)
	mux.HandleFunc("POST /profiles", deps.Profiles.Create)
	mux.HandleFunc("GET /profiles/{id}", deps.Profiles.Get)
	mux.HandleFunc("DELETE /profiles/{id}", deps.Profiles.Delete)

	// Sessions
	mux.HandleFunc("GET /sessions", deps.Sessions.List)
	mux.HandleFunc("POST /sessions", deps.Sessions.Create)
	mux.HandleFunc("GET /sessions/{id}", deps.Sessions.Get)
	mux.HandleFunc("POST /sessions/{id}/stop", deps.Sessions.Stop)

	// Network
	mux.HandleFunc("GET /network/status", deps.Network.Status)
}

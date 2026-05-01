// pkg/api/sessions.go
// Package api — sessions endpoint handlers.
package api

import (
	"net/http"

	"ghost-silicon/pkg/storage"
)

// SessionsHandler handles all /sessions routes.
type SessionsHandler struct {
	Store *storage.SessionStore
}

// List handles GET /sessions — returns all stored session metadata.
func (h SessionsHandler) List(w http.ResponseWriter, _ *http.Request) {
	metas, err := h.Store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, metas)
}

// Get handles GET /sessions/{id} — returns a single session record.
func (h SessionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := h.Store.Load(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// Create handles POST /sessions — placeholder; session creation is driven
// by the supervisor, not the API. Returns 501 for now.
func (h SessionsHandler) Create(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented,
		"session creation via API not yet implemented; use the supervisor directly")
}

// Stop handles POST /sessions/{id}/stop — marks a session as stopped.
func (h SessionsHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := h.Store.Load(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	m.State = "stopped"
	if err := h.Store.Save(m); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

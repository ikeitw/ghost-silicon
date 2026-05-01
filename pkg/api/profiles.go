// pkg/api/profiles.go
// Package api — profiles endpoint handlers.
package api

import (
	"encoding/json"
	"net/http"

	"ghost-silicon/pkg/identity"
)

// ProfilesHandler handles all /profiles routes.
type ProfilesHandler struct {
	Store identity.Store
}

// List handles GET /profiles — returns all stored profile metadata.
func (h ProfilesHandler) List(w http.ResponseWriter, r *http.Request) {
	metas, err := h.Store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, metas)
}

// Get handles GET /profiles/{id} — returns a single full profile.
func (h ProfilesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := h.Store.Load(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// Create handles POST /profiles — decodes and saves a new profile.
func (h ProfilesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p identity.Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	result := identity.Validate(&p)
	if !result.Valid() {
		writeError(w, http.StatusUnprocessableEntity, result.Error())
		return
	}

	if err := h.Store.Save(&p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, &p)
}

// Delete handles DELETE /profiles/{id} — removes a stored profile.
func (h ProfilesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Store.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

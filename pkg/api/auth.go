// pkg/api/auth.go
// Package api — local API authentication.
// The control API binds only to 127.0.0.1 so network-level access is already
// restricted. This file adds an optional bearer token check for environments
// where multiple local users share a machine.
package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// TokenAuthMiddleware wraps h with a bearer token check.
// If token is empty, all requests are allowed (no auth).
func TokenAuthMiddleware(h http.Handler, token string) http.Handler {
	if token == "" {
		return h // auth disabled
	}
	expected := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimSpace(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare([]byte(got), expected) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid or missing Authorization token")
			return
		}
		h.ServeHTTP(w, r)
	})
}

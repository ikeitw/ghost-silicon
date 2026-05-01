// pkg/api/middleware.go
// Package api — HTTP middleware for the local control API.
package api

import (
	"net/http"
	"time"

	"ghost-silicon/internal/telemetry/logging"
)

// LoggingMiddleware wraps h and logs every request at DEBUG level.
func LoggingMiddleware(h http.Handler, log *logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(rw, r)
		log.Debug("api request",
			logging.FieldMethod, r.Method,
			logging.FieldURL, r.URL.Path,
			"status", rw.status,
			logging.FieldDuration, time.Since(start).Milliseconds(),
		)
	})
}

// RecoveryMiddleware catches panics in handlers and returns 500.
func RecoveryMiddleware(h http.Handler, log *logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("api handler panic",
					logging.FieldMethod, r.Method,
					logging.FieldURL, r.URL.Path,
					"panic", rec,
				)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		h.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

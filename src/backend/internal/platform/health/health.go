// Package health provides the operational liveness and readiness endpoints.
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

const readinessTimeout = 2 * time.Second

// Pinger reports whether a required dependency is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

type response struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

// Liveness answers 200 while the process can serve HTTP.
func Liveness() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, response{Status: "ok"})
	}
}

// Readiness answers 200 when PostgreSQL is reachable and 503 otherwise.
// Failure details are logged, never returned to the caller.
func Readiness(db Pinger, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			logger.WarnContext(r.Context(), "readiness check failed", "dependency", "postgres", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, response{
				Status: "not_ready",
				Checks: map[string]string{"postgres": "unavailable"},
			})
			return
		}
		writeJSON(w, http.StatusOK, response{
			Status: "ready",
			Checks: map[string]string{"postgres": "ok"},
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, body response) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

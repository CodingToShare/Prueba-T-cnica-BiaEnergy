package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"bia-energy.local/backend/internal/auth"
)

// RequestIDHeader carries the request ID in every response.
const RequestIDHeader = "X-Request-ID"

// requestTimeout bounds the work of one API request (database queries
// included); the HTTP server's own timeouts still apply.
const requestTimeout = 15 * time.Second

type contextKey int

const (
	requestIDKey contextKey = iota
	logFieldsKey
	sessionKey
)

// RequestID returns the request's ID ("" outside a request).
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// newRequestID returns 128 random bits in hex. Unlike chi's default it
// contains no host name. Incoming X-Request-ID values are not trusted.
func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read never fails on supported platforms
	return hex.EncodeToString(b)
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// logFields collects attributes a handler adds to its request log line
// (for example analysis_id or meter_id).
type logFields struct {
	mu    sync.Mutex
	attrs []any
}

// addLogFields attaches key/value pairs to the current request's log line.
func addLogFields(ctx context.Context, kv ...any) {
	if f, ok := ctx.Value(logFieldsKey).(*logFields); ok {
		f.mu.Lock()
		f.attrs = append(f.attrs, kv...)
		f.mu.Unlock()
	}
}

// requestLogger writes one structured line per request: never bodies,
// cookies, query strings or credentials.
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			fields := &logFields{}
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r.WithContext(context.WithValue(r.Context(), logFieldsKey, fields)))

			route := ""
			if rc := chi.RouteContext(r.Context()); rc != nil {
				route = rc.RoutePattern()
			}
			attrs := []any{
				"request_id", RequestID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"route", route,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
			}
			fields.mu.Lock()
			attrs = append(attrs, fields.attrs...)
			fields.mu.Unlock()
			logger.InfoContext(r.Context(), "http request", attrs...)
		})
	}
}

// recoverer turns a panic into the standard 500 body and logs it with its
// stack; the client sees no internals.
func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logger.ErrorContext(r.Context(), "panic while serving request",
					"request_id", RequestID(r.Context()), "panic_type", fmt.Sprintf("%T", rec), "stack", string(debug.Stack()))
				writeError(w, r, errInternal)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func withTimeout(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireSession answers 401 unless the request carries a valid session
// cookie; it never redirects (the frontend decides navigation).
func requireSession(m *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, err := m.FromRequest(r)
			if err != nil {
				writeError(w, r, errUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey, s)))
		})
	}
}

func sessionFrom(ctx context.Context) (auth.Session, bool) {
	s, ok := ctx.Value(sessionKey).(auth.Session)
	return s, ok
}

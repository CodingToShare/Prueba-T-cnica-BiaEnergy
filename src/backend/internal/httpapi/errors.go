package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/anomaly"
	"bia-energy.local/backend/internal/meter"
)

// apiError is an error with its HTTP status, stable code and safe message.
type apiError struct {
	status  int
	code    string
	message string
}

func (e *apiError) Error() string { return e.code + ": " + e.message }

func badRequest(code, message string) error {
	return &apiError{status: http.StatusBadRequest, code: code, message: message}
}

var (
	errUnauthorized       = &apiError{http.StatusUnauthorized, "unauthorized", "Authentication is required."}
	errInvalidCredentials = &apiError{http.StatusUnauthorized, "invalid_credentials", "Invalid username or password."}
	errInvalidLoginBody   = &apiError{http.StatusBadRequest, "invalid_request", "The body must be a JSON object with a username and a password."}
	errNotFound           = &apiError{http.StatusNotFound, "not_found", "The requested resource does not exist."}
	errMethodNotAllowed   = &apiError{http.StatusMethodNotAllowed, "method_not_allowed", "The method is not allowed for this resource."}
	errMeterNotFound      = &apiError{http.StatusNotFound, "meter_not_found", "Meter was not found."}
	errAnomalyNotFound    = &apiError{http.StatusNotFound, "anomaly_not_found", "Anomaly was not found."}
	errAnalysisNotFound   = &apiError{http.StatusNotFound, "analysis_not_found", "Analysis was not found."}
	errTimeout            = &apiError{http.StatusServiceUnavailable, "request_timeout", "The request took too long. Please retry."}
	errInternal           = &apiError{http.StatusInternalServerError, "internal_error", "An unexpected error occurred."}
)

// errorBody is the single JSON error shape of the API.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// writeError answers with the standard error body; internals never reach it.
func writeError(w http.ResponseWriter, r *http.Request, e *apiError) {
	writeJSON(w, e.status, errorBody{Error: errorDetail{Code: e.code, Message: e.message, RequestID: RequestID(r.Context())}})
}

// respond maps a handler error to a response. Known domain errors become
// 4xx; anything else is logged and answered with a generic 500.
func respond(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	var ae *apiError
	switch {
	case errors.As(err, &ae):
		writeError(w, r, ae)
	case errors.Is(err, meter.ErrNotFound):
		writeError(w, r, errMeterNotFound)
	case errors.Is(err, anomaly.ErrNotFound):
		writeError(w, r, errAnomalyNotFound)
	case errors.Is(err, analysisrun.ErrNotFound):
		writeError(w, r, errAnalysisNotFound)
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
		logger.WarnContext(r.Context(), "request did not complete in time", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, r, errTimeout)
	default:
		logger.ErrorContext(r.Context(), "request failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, r, errInternal)
	}
}

// writeJSON encodes before writing, so an encoding failure still yields a
// well-formed 500 instead of a truncated body.
func writeJSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		status = http.StatusInternalServerError
		data, _ = json.Marshal(errorBody{Error: errorDetail{
			Code: errInternal.code, Message: errInternal.message, RequestID: w.Header().Get(RequestIDHeader),
		}})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}

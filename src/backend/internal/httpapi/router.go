// Package httpapi is the versioned HTTP API (/api/v1) plus the operational
// endpoints. Handlers only decode, validate, call a service and encode;
// analytics and SQL live elsewhere.
package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/anomaly"
	"bia-energy.local/backend/internal/auth"
	"bia-energy.local/backend/internal/dashboard"
	"bia-energy.local/backend/internal/meter"
	"bia-energy.local/backend/internal/platform/health"
	"bia-energy.local/backend/internal/platform/metrics"
)

// maxLoginBody bounds the login request body.
const maxLoginBody = 4 << 10

// Deps are the services the API serves.
type Deps struct {
	Logger    *slog.Logger
	DB        health.Pinger
	Auth      *auth.Manager
	Meters    *meter.Service
	Anomalies *anomaly.Service
	Dashboard *dashboard.Service
	Runs      *analysisrun.Service
	// Metrics is optional; with it the router records request metrics and
	// serves GET /metrics.
	Metrics *metrics.Metrics
}

type api struct{ Deps }

// NewRouter builds the routes. Health, readiness and metrics stay public and
// outside /api/v1 (operational endpoints; restrict them at the network edge
// in a real deployment); every /api/v1 route except login and logout needs a
// session.
func NewRouter(d Deps) http.Handler {
	a := &api{Deps: d}
	r := chi.NewRouter()
	r.Use(withRequestID, requestLogger(d.Logger, d.Metrics), recoverer(d.Logger))
	r.NotFound(func(w http.ResponseWriter, r *http.Request) { writeError(w, r, errNotFound) })
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) { writeError(w, r, errMethodNotAllowed) })

	r.Get("/healthz", health.Liveness())
	r.Get("/readyz", health.Readiness(d.DB, d.Logger))
	if d.Metrics != nil {
		r.Method(http.MethodGet, "/metrics", d.Metrics.Handler())
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(withTimeout)
		r.Post("/auth/login", a.handle(a.login))
		r.Post("/auth/logout", a.handle(a.logout))
		r.Group(func(r chi.Router) {
			r.Use(requireSession(d.Auth))
			r.Get("/auth/session", a.handle(a.session))
			r.Get("/meters", a.handle(a.listMeters))
			r.Get("/meters/{meterId}", a.handle(a.getMeter))
			r.Get("/meters/{meterId}/readings", a.handle(a.listReadings))
			r.Get("/anomalies", a.handle(a.listAnomalies))
			r.Get("/anomalies/{id}", a.handle(a.getAnomaly))
			r.Post("/ai/analyze", a.handle(a.analyze))
			r.Get("/ai/analysis/{id}", a.handle(a.getAnalysis))
			r.Get("/dashboard/summary", a.handle(a.dashboardSummary))
		})
	})
	return r
}

func (a *api) handle(fn func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			respond(a.Logger, w, r, err)
		}
	}
}

func (a *api) login(w http.ResponseWriter, r *http.Request) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errInvalidLoginBody
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || body.Username == "" || body.Password == "" {
		return errInvalidLoginBody
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return errInvalidLoginBody
	}
	if !a.Auth.CheckCredentials(body.Username, body.Password) {
		a.Logger.WarnContext(r.Context(), "login failed", "request_id", RequestID(r.Context()))
		return errInvalidCredentials
	}
	session, token, err := a.Auth.Issue()
	if err != nil {
		return err
	}
	http.SetCookie(w, a.Auth.SessionCookie(token, session))
	a.Logger.InfoContext(r.Context(), "login succeeded", "request_id", RequestID(r.Context()))
	writeJSON(w, http.StatusOK, sessionBody(session))
	return nil
}

// logout always clears the cookie and answers 204, with or without a valid
// session. There is no server-side session store to revoke (OD-12).
func (a *api) logout(w http.ResponseWriter, _ *http.Request) error {
	http.SetCookie(w, a.Auth.ClearCookie())
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (a *api) session(w http.ResponseWriter, r *http.Request) error {
	s, ok := sessionFrom(r.Context())
	if !ok {
		return errUnauthorized
	}
	writeJSON(w, http.StatusOK, sessionBody(s))
	return nil
}

var (
	meterSorts     = []string{string(meter.SortMeterID), string(meter.SortConsumption), string(meter.SortVariation), string(meter.SortSeverity)}
	meterStatuses  = []string{"OK", "ALERT", "CRITICAL"}
	anomalyTypes   = []string{"REAL_ANOMALY", "DATA_QUALITY", "EXPLAINABLE_ANOMALY", "FALSE_POSITIVE"}
	severities     = []string{"HIGH", "MEDIUM", "LOW"}
	sortDirections = []string{"asc", "desc"}
)

func (a *api) listMeters(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := pageParams(r)
	if err != nil {
		return err
	}
	p := meter.ListParams{Sort: meter.SortMeterID, Limit: limit, Offset: offset}
	if raw, ok, err := queryValue(r, "search"); err != nil {
		return err
	} else if ok && raw != "" {
		if utf8.RuneCountInString(raw) > maxSearchLength {
			return invalidQuery("search", "must have at most 64 characters")
		}
		p.Search = &raw
	}
	if p.ComputedStatus, err = enumParam(r, "status", meterStatuses...); err != nil {
		return err
	}
	sort, err := enumParam(r, "sort", meterSorts...)
	if err != nil {
		return err
	}
	if sort != nil {
		p.Sort = meter.SortKey(*sort)
	}
	order, err := enumParam(r, "order", sortDirections...)
	if err != nil {
		return err
	}
	// Default direction: A→Z for meter_id; largest first for the others.
	p.Descending = p.Sort != meter.SortMeterID
	if order != nil {
		p.Descending = *order == "desc"
	}
	page, err := a.Meters.List(r.Context(), p)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, meterList(page, limit, offset))
	return nil
}

func (a *api) getMeter(w http.ResponseWriter, r *http.Request) error {
	id, err := meterIDPath(r)
	if err != nil {
		return err
	}
	addLogFields(r.Context(), "meter_id", id)
	d, err := a.Meters.Get(r.Context(), id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, meterDetail(d))
	return nil
}

func (a *api) listReadings(w http.ResponseWriter, r *http.Request) error {
	id, err := meterIDPath(r)
	if err != nil {
		return err
	}
	addLogFields(r.Context(), "meter_id", id)
	p := meter.ReadingsParams{}
	if p.From, err = sourceTimeParam(r, "from"); err != nil {
		return err
	}
	if p.To, err = sourceTimeParam(r, "to"); err != nil {
		return err
	}
	if p.From != nil && p.To != nil && p.From.After(*p.To) {
		return badRequest("invalid_range", `"from" must not be later than "to".`)
	}
	if p.Limit, err = intParam(r, "limit", defaultReadingLimit, 1, maxReadingLimit); err != nil {
		return err
	}
	rs, more, err := a.Meters.Readings(r.Context(), id, p)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, readings(id, p, rs, more))
	return nil
}

func (a *api) listAnomalies(w http.ResponseWriter, r *http.Request) error {
	limit, offset, err := pageParams(r)
	if err != nil {
		return err
	}
	p := anomaly.ListParams{Limit: limit, Offset: offset}
	if raw, ok, err := queryValue(r, "meter_id"); err != nil {
		return err
	} else if ok {
		id, err := meterIDValue(raw)
		if err != nil {
			return err
		}
		p.MeterID = &id
	}
	if p.Type, err = enumParam(r, "type", anomalyTypes...); err != nil {
		return err
	}
	if p.Severity, err = enumParam(r, "severity", severities...); err != nil {
		return err
	}
	page, err := a.Anomalies.List(r.Context(), p)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, anomalyList(page, limit, offset))
	return nil
}

func (a *api) getAnomaly(w http.ResponseWriter, r *http.Request) error {
	id, err := idPath(r, "id", "invalid_anomaly_id", "anomaly")
	if err != nil {
		return err
	}
	addLogFields(r.Context(), "anomaly_id", id)
	d, err := a.Anomalies.Get(r.Context(), id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, anomalyDetail(d))
	return nil
}

// analyze queues a run (or returns the active one) and answers 202 without
// waiting for the analysis.
func (a *api) analyze(w http.ResponseWriter, r *http.Request) error {
	run, created, err := a.Runs.Request(r.Context())
	if err != nil {
		return err
	}
	addLogFields(r.Context(), "analysis_id", run.ID, "created", created)
	w.Header().Set("Location", "/api/v1/ai/analysis/"+strconv.FormatInt(run.ID, 10))
	writeJSON(w, http.StatusAccepted, analyzeDTO{runDTO: runBody(run), Created: created})
	return nil
}

func (a *api) getAnalysis(w http.ResponseWriter, r *http.Request) error {
	id, err := idPath(r, "id", "invalid_analysis_id", "analysis")
	if err != nil {
		return err
	}
	addLogFields(r.Context(), "analysis_id", id)
	run, err := a.Runs.Get(r.Context(), id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, runBody(run))
	return nil
}

func (a *api) dashboardSummary(w http.ResponseWriter, r *http.Request) error {
	s, err := a.Dashboard.Summary(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, dashboardBody(s))
	return nil
}

// Package metrics defines the service's Prometheus metrics (architecture §9).
// Every collector lives in a Registry owned by the application, never the
// global default registry, so tests build independent instances. Labels are
// bounded enumerations: route templates, method, status class, run status,
// explanation provider, outcome and fallback code. No identifier (meter,
// anomaly, analysis, request) is ever a label.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

const namespace = "bia"

// Explanation outcomes.
const (
	OutcomeGenerated   = "generated"   // the configured provider produced the text
	OutcomeFallback    = "fallback"    // the deterministic text replaced a failed provider
	OutcomeUnavailable = "unavailable" // no text could be produced
)

// Metrics holds the collectors. A nil *Metrics is valid and records nothing,
// so components can be built without metrics in tests.
type Metrics struct {
	registry *prometheus.Registry

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec

	runsTotal   *prometheus.CounterVec
	runDuration *prometheus.HistogramVec
	runFindings prometheus.Histogram
	activeRuns  prometheus.Gauge

	explanations        *prometheus.CounterVec
	explanationDuration *prometheus.HistogramVec
	fallbacks           *prometheus.CounterVec
}

// New registers every collector in a new registry, plus the standard Go
// runtime and process collectors.
func New() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "http_requests_total",
			Help: "HTTP requests by method, route template and status class.",
		}, []string{"method", "route", "status_class"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "http_request_duration_seconds",
			Help:    "HTTP request duration by method and route template.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route"}),
		runsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "analysis_runs_total",
			Help: "Analysis runs that reached a terminal status (counted after the status is stored).",
		}, []string{"status"}),
		runDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "analysis_run_duration_seconds",
			Help:    "Duration of analysis runs from claim to terminal status.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
		}, []string{"status"}),
		runFindings: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace, Name: "analysis_findings",
			Help:    "Findings per completed analysis run.",
			Buckets: []float64{0, 1, 2, 4, 8, 16, 32, 64},
		}),
		activeRuns: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace, Name: "analysis_runs_active",
			Help: "Analysis runs currently executing in this process.",
		}),
		explanations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "explanation_generation_total",
			Help: "Finding explanations by configured provider and outcome (generated, fallback, unavailable).",
		}, []string{"provider", "outcome"}),
		explanationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace, Name: "explanation_generation_duration_seconds",
			Help:    "Time spent producing one finding explanation, including any fallback.",
			Buckets: []float64{0.0001, 0.001, 0.01, 0.1, 0.5, 1, 2.5, 5, 10, 20, 40, 80},
		}, []string{"provider"}),
		fallbacks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Name: "explanation_fallback_total",
			Help: "Explanations that fell back to the deterministic text, by sanitized fallback code.",
		}, []string{"fallback_code"}),
	}
	m.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.httpRequests, m.httpDuration,
		m.runsTotal, m.runDuration, m.runFindings, m.activeRuns,
		m.explanations, m.explanationDuration, m.fallbacks,
	)
	return m
}

// Handler serves the registry in the Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry})
}

// Gatherer exposes the registry for tests and in-process inspection.
func (m *Metrics) Gatherer() prometheus.Gatherer { return m.registry }

// Value returns the value of a counter or gauge sample, or the sample count
// of a histogram, whose labels include every pair in labels.
func Value(g prometheus.Gatherer, name string, labels map[string]string) (float64, bool) {
	families, err := g.Gather()
	if err != nil {
		return 0, false
	}
	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		for _, sample := range mf.GetMetric() {
			if !hasLabels(sample.GetLabel(), labels) {
				continue
			}
			switch {
			case sample.GetCounter() != nil:
				return sample.GetCounter().GetValue(), true
			case sample.GetGauge() != nil:
				return sample.GetGauge().GetValue(), true
			case sample.GetHistogram() != nil:
				return float64(sample.GetHistogram().GetSampleCount()), true
			}
		}
	}
	return 0, false
}

func hasLabels(pairs []*dto.LabelPair, want map[string]string) bool {
	for k, v := range want {
		found := false
		for _, p := range pairs {
			if p.GetName() == k && p.GetValue() == v {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// ObserveHTTP records one request. route must be a route template (or
// "unmatched"), never a raw path.
func (m *Metrics) ObserveHTTP(method, route string, status int, d time.Duration) {
	if m == nil {
		return
	}
	m.httpRequests.WithLabelValues(method, route, statusClass(status)).Inc()
	m.httpDuration.WithLabelValues(method, route).Observe(d.Seconds())
}

// RunStarted marks a run as executing.
func (m *Metrics) RunStarted() {
	if m == nil {
		return
	}
	m.activeRuns.Inc()
}

// RunStopped clears the in-process active gauge without publishing a terminal
// status. It is used when the worker stopped but storing its terminal status
// failed, so the metrics never claim a database transition that did not occur.
func (m *Metrics) RunStopped() {
	if m == nil {
		return
	}
	m.activeRuns.Dec()
}

// RunFinished records a run whose terminal status ("completed" or "failed")
// has been stored. findings is ignored for failed runs.
func (m *Metrics) RunFinished(status string, d time.Duration, findings int) {
	if m == nil {
		return
	}
	m.RunStopped()
	m.runsTotal.WithLabelValues(status).Inc()
	m.runDuration.WithLabelValues(status).Observe(d.Seconds())
	if status == "completed" {
		m.runFindings.Observe(float64(findings))
	}
}

// Explanation records one finding explanation. fallbackCode is set only when
// outcome is OutcomeFallback or OutcomeUnavailable.
func (m *Metrics) Explanation(provider, outcome, fallbackCode string, d time.Duration) {
	if m == nil {
		return
	}
	m.explanations.WithLabelValues(provider, outcome).Inc()
	m.explanationDuration.WithLabelValues(provider).Observe(d.Seconds())
	if outcome == OutcomeFallback {
		m.fallbacks.WithLabelValues(fallbackCode).Inc()
	}
}

func statusClass(status int) string {
	if status < 100 || status > 599 {
		return "other"
	}
	return strconv.Itoa(status/100) + "xx"
}

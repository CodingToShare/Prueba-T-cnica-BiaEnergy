package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"bia-energy.local/backend/internal/platform/jsontime"
)

// Paging bounds.
const (
	defaultPageLimit    = 50
	maxPageLimit        = 100
	maxOffset           = 1_000_000
	defaultReadingLimit = 1000
	maxReadingLimit     = 1000
	maxSearchLength     = 64
)

// meterIDPattern accepts identifiers like the source's (letters, digits,
// '.', '_' and '-'); anything else is rejected before reaching SQL.
var meterIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func invalidQuery(name, rule string) error {
	return badRequest("invalid_query", fmt.Sprintf("Query parameter %q %s.", name, rule))
}

// queryValue returns a single query value. A malformed query string (which
// r.URL.Query would silently drop, e.g. a ";" separator) or a repeated
// parameter is an error, never silently resolved.
func queryValue(r *http.Request, name string) (string, bool, error) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return "", false, badRequest("invalid_query", "The query string is malformed.")
	}
	values, ok := query[name]
	if !ok {
		return "", false, nil
	}
	if len(values) != 1 {
		return "", false, invalidQuery(name, "must be given once")
	}
	return values[0], true, nil
}

func intParam(r *http.Request, name string, def, lo, hi int) (int, error) {
	raw, ok, err := queryValue(r, name)
	if err != nil || !ok {
		return def, err
	}
	v, perr := strconv.Atoi(raw)
	if perr != nil || v < lo || v > hi {
		return 0, invalidQuery(name, fmt.Sprintf("must be an integer from %d to %d", lo, hi))
	}
	return v, nil
}

// enumParam returns the value if it is one of allowed, nil when absent.
func enumParam(r *http.Request, name string, allowed ...string) (*string, error) {
	raw, ok, err := queryValue(r, name)
	if err != nil || !ok {
		return nil, err
	}
	if !slices.Contains(allowed, raw) {
		return nil, invalidQuery(name, "must be one of "+strings.Join(allowed, ", "))
	}
	return &raw, nil
}

// sourceTimeParam parses an optional source wall-clock time.
func sourceTimeParam(r *http.Request, name string) (*time.Time, error) {
	raw, ok, err := queryValue(r, name)
	if err != nil || !ok {
		return nil, err
	}
	t, perr := jsontime.ParseSource(raw)
	if perr != nil {
		return nil, badRequest("invalid_timestamp",
			fmt.Sprintf("Query parameter %q must be a source time like 2026-09-12T14:00:00, without an offset.", name))
	}
	return &t, nil
}

func pageParams(r *http.Request) (limit, offset int, err error) {
	if limit, err = intParam(r, "limit", defaultPageLimit, 1, maxPageLimit); err != nil {
		return 0, 0, err
	}
	offset, err = intParam(r, "offset", 0, 0, maxOffset)
	return limit, offset, err
}

func meterIDValue(raw string) (string, error) {
	if !meterIDPattern.MatchString(raw) {
		return "", badRequest("invalid_meter_id", "The meter ID must be 1–64 letters, digits, '.', '_' or '-'.")
	}
	return raw, nil
}

func meterIDPath(r *http.Request) (string, error) {
	return meterIDValue(chi.URLParam(r, "meterId"))
}

// idPath parses a positive integer path identifier.
func idPath(r *http.Request, name, code, label string) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id < 1 {
		return 0, badRequest(code, fmt.Sprintf("The %s ID must be a positive integer.", label))
	}
	return id, nil
}

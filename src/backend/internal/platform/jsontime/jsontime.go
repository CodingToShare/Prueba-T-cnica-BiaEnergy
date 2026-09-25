// Package jsontime is the JSON boundary for the two kinds of time the API
// exposes. Source times (readings, events, finding episodes) are the source
// wall clock with no zone (ADR-008) and are written without an offset;
// system instants (analysis runs, sessions) are written as RFC 3339 UTC.
// Marshaling a plain time.Time would silently claim UTC for source times.
package jsontime

import (
	"fmt"
	"strconv"
	"time"
)

// SourceLayout is the API format of a source wall-clock time.
const SourceLayout = "2006-01-02T15:04:05"

// Source is a source wall-clock time, serialized as "2006-01-02T15:04:05".
type Source time.Time

// MarshalJSON writes the wall clock without any offset.
func (s Source) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(time.Time(s).Format(SourceLayout))), nil
}

// UnmarshalJSON reads the SourceLayout format.
func (s *Source) UnmarshalJSON(data []byte) error {
	text, err := strconv.Unquote(string(data))
	if err != nil {
		return fmt.Errorf("source time must be a JSON string: %w", err)
	}
	t, err := ParseSource(text)
	if err != nil {
		return err
	}
	*s = Source(t)
	return nil
}

// ParseSource parses a source wall-clock time. An offset or zone suffix is
// rejected because source times have none. The result uses UTC only as a
// carrier, like the ingestion parser.
func ParseSource(text string) (time.Time, error) {
	t, err := time.ParseInLocation(SourceLayout, text, time.UTC)
	if err != nil || t.Format(SourceLayout) != text {
		return time.Time{}, fmt.Errorf("%q is not a source time in the form %s without an offset", text, SourceLayout)
	}
	return t, nil
}

// SourcePtr converts an optional time.
func SourcePtr(t *time.Time) *Source {
	if t == nil {
		return nil
	}
	s := Source(*t)
	return &s
}

// System is a real instant, serialized as RFC 3339 in UTC.
type System time.Time

// MarshalJSON writes RFC 3339 (with fractional seconds when present) in UTC.
func (s System) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(time.Time(s).UTC().Format(time.RFC3339Nano))), nil
}

// UnmarshalJSON reads RFC 3339.
func (s *System) UnmarshalJSON(data []byte) error {
	text, err := strconv.Unquote(string(data))
	if err != nil {
		return fmt.Errorf("system time must be a JSON string: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return fmt.Errorf("%q is not an RFC 3339 time", text)
	}
	*s = System(t)
	return nil
}

// SystemPtr converts an optional instant.
func SystemPtr(t *time.Time) *System {
	if t == nil {
		return nil
	}
	s := System(*t)
	return &s
}

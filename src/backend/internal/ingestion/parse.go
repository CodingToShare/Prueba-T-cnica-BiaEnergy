package ingestion

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Expected source headers. Columns may appear in any order; a missing,
// duplicated or unknown column rejects the file.
var (
	readingsColumns = []string{"meter_id", "timestamp", "consumption_kwh", "voltage_v", "current_a", "power_factor", "status"}
	eventsColumns   = []string{"meter_id", "event_timestamp", "event_type", "description"}
)

// utf8BOM is stripped from the first header cell (spreadsheet exports add it).
const utf8BOM = string(rune(0xFEFF))

// maxReportedErrors bounds the row errors kept in a ValidationError.
const maxReportedErrors = 25

// RowError describes one invalid value in a source file.
type RowError struct {
	Line   int
	Column string
	Reason string
}

func (e RowError) String() string {
	if e.Column == "" {
		return fmt.Sprintf("line %d: %s", e.Line, e.Reason)
	}
	return fmt.Sprintf("line %d, column %s: %s", e.Line, e.Column, e.Reason)
}

// ValidationError reports every rejected row of a source file (up to maxReportedErrors).
type ValidationError struct {
	File    string
	Errors  []RowError
	Omitted int
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, re := range e.Errors {
		parts = append(parts, re.String())
	}
	msg := fmt.Sprintf("%s: %d invalid value(s): %s", e.File, len(e.Errors)+e.Omitted, strings.Join(parts, "; "))
	if e.Omitted > 0 {
		msg += fmt.Sprintf("; and %d more", e.Omitted)
	}
	return msg
}

func (e *ValidationError) add(line int, column, reason string) {
	if len(e.Errors) < maxReportedErrors {
		e.Errors = append(e.Errors, RowError{Line: line, Column: column, Reason: reason})
		return
	}
	e.Omitted++
}

func (e *ValidationError) orNil() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e
}

// ParseReadings parses and validates readings.csv content.
func ParseReadings(r io.Reader) ([]Reading, error) {
	rows, err := readRows(r, "readings.csv", readingsColumns)
	if err != nil {
		return nil, err
	}

	verr := &ValidationError{File: "readings.csv"}
	type key struct {
		meterID string
		ts      time.Time
	}
	firstLine := make(map[key]int, len(rows.records))
	readings := make([]Reading, 0, len(rows.records))

	for _, rec := range rows.records {
		f := rowFields{rec: rec, errs: verr}
		reading := Reading{
			MeterID:        f.identifier("meter_id"),
			Timestamp:      f.timestamp("timestamp"),
			ConsumptionKWh: f.measurement("consumption_kwh"),
			VoltageV:       f.measurement("voltage_v"),
			CurrentA:       f.measurement("current_a"),
			PowerFactor:    f.measurement("power_factor"),
			SourceStatus:   f.required("status"),
		}
		if f.failed {
			continue
		}
		k := key{meterID: reading.MeterID, ts: reading.Timestamp}
		if prev, dup := firstLine[k]; dup {
			verr.add(rec.line, "", fmt.Sprintf("duplicate reading for meter %s at %s (first seen on line %d)",
				reading.MeterID, reading.Timestamp.Format(time.DateTime), prev))
			continue
		}
		firstLine[k] = rec.line
		readings = append(readings, reading)
	}

	if err := verr.orNil(); err != nil {
		return nil, err
	}
	if len(readings) == 0 {
		return nil, errors.New("readings.csv: contains no data rows")
	}
	return readings, nil
}

// ParseEvents parses and validates events.csv content. A file with only the
// header is valid: a period may have no known events.
func ParseEvents(r io.Reader) ([]Event, error) {
	rows, err := readRows(r, "events.csv", eventsColumns)
	if err != nil {
		return nil, err
	}

	verr := &ValidationError{File: "events.csv"}
	firstLine := make(map[Event]int, len(rows.records))
	events := make([]Event, 0, len(rows.records))

	for _, rec := range rows.records {
		f := rowFields{rec: rec, errs: verr}
		event := Event{
			MeterID:     f.identifier("meter_id"),
			Timestamp:   f.timestamp("event_timestamp"),
			Type:        f.required("event_type"),
			Description: f.required("description"),
		}
		if f.failed {
			continue
		}
		if prev, dup := firstLine[event]; dup {
			verr.add(rec.line, "", fmt.Sprintf("duplicate event (first seen on line %d)", prev))
			continue
		}
		firstLine[event] = rec.line
		events = append(events, event)
	}

	if err := verr.orNil(); err != nil {
		return nil, err
	}
	return events, nil
}

type record struct {
	line   int
	values map[string]string
}

type rows struct {
	records []record
}

// readRows validates the header and returns every data row keyed by column
// name. Structural CSV problems (wrong field count, broken quoting) are
// reported as validation errors with line numbers.
func readRows(r io.Reader, file string, columns []string) (rows, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return rows{}, fmt.Errorf("%s: file is empty (missing header)", file)
	}
	if err != nil {
		return rows{}, fmt.Errorf("%s: read header: %w", file, err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], utf8BOM)
	}
	index, err := indexHeader(file, header, columns)
	if err != nil {
		return rows{}, err
	}

	verr := &ValidationError{File: file}
	var out rows
	for {
		fields, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var perr *csv.ParseError
			if !errors.As(err, &perr) {
				return rows{}, fmt.Errorf("%s: read: %w", file, err)
			}
			verr.add(perr.Line, "", perr.Err.Error())
			if errors.Is(perr.Err, csv.ErrFieldCount) {
				continue // the reader stays aligned; keep collecting errors
			}
			return rows{}, verr // broken quoting: later lines cannot be trusted
		}
		line, _ := cr.FieldPos(0)
		values := make(map[string]string, len(columns))
		for name, i := range index {
			values[name] = fields[i]
		}
		out.records = append(out.records, record{line: line, values: values})
	}
	if err := verr.orNil(); err != nil {
		return rows{}, err
	}
	return out, nil
}

func indexHeader(file string, header, columns []string) (map[string]int, error) {
	expected := make(map[string]bool, len(columns))
	for _, c := range columns {
		expected[c] = true
	}

	index := make(map[string]int, len(header))
	var problems []string
	for i, name := range header {
		if !expected[name] {
			problems = append(problems, fmt.Sprintf("unexpected column %q", name))
			continue
		}
		if _, seen := index[name]; seen {
			problems = append(problems, fmt.Sprintf("duplicate column %q", name))
			continue
		}
		index[name] = i
	}
	for _, c := range columns {
		if _, ok := index[c]; !ok {
			problems = append(problems, fmt.Sprintf("missing column %q", c))
		}
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s: invalid header: %s (expected %s)", file, strings.Join(problems, ", "), strings.Join(columns, ","))
	}
	return index, nil
}

// rowFields converts the values of one row, recording every problem found.
type rowFields struct {
	rec    record
	errs   *ValidationError
	failed bool
}

func (f *rowFields) fail(column, reason string) {
	f.failed = true
	f.errs.add(f.rec.line, column, reason)
}

func (f *rowFields) required(column string) string {
	v := f.rec.values[column]
	if strings.TrimSpace(v) == "" {
		f.fail(column, "value is required")
	}
	return v
}

// identifier rejects blank values and surrounding whitespace, which would
// otherwise silently create distinct meters for the same identifier.
func (f *rowFields) identifier(column string) string {
	v := f.required(column)
	if v != "" && v != strings.TrimSpace(v) {
		f.fail(column, fmt.Sprintf("identifier %q has leading or trailing whitespace", v))
	}
	return v
}

func (f *rowFields) timestamp(column string) time.Time {
	v := f.rec.values[column]
	if strings.TrimSpace(v) == "" {
		f.fail(column, "value is required")
		return time.Time{}
	}
	t, err := ParseSourceTimestamp(v)
	if err != nil {
		f.fail(column, err.Error())
	}
	return t
}

// decimalLiteral is plain decimal notation with an optional exponent; it
// excludes NaN, Inf and hexadecimal forms that strconv.ParseFloat accepts.
var decimalLiteral = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)

// measurement accepts any finite decimal number. Physically unusual values
// (negative, zero, very large, power factor outside 0..1) are kept on purpose.
func (f *rowFields) measurement(column string) float64 {
	v := f.rec.values[column]
	if strings.TrimSpace(v) == "" {
		f.fail(column, "value is required")
		return 0
	}
	if !decimalLiteral.MatchString(v) {
		f.fail(column, fmt.Sprintf("%q is not a decimal number", v))
		return 0
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsInf(n, 0) {
		f.fail(column, fmt.Sprintf("%q is out of range", v))
		return 0
	}
	return n
}

package ingestion

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const readingsHeaderLine = "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status\n"

func TestParseReadings_ValidRow_ReturnsTypedReading(t *testing.T) {
	input := readingsHeaderLine + "TEST-A,2026-01-05 13:00:00,23.5,221.9,101.28,0.954,OK\n"

	readings, err := ParseReadings(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, readings, 1)
	assert.Equal(t, Reading{
		MeterID:        "TEST-A",
		Timestamp:      time.Date(2026, 1, 5, 13, 0, 0, 0, time.UTC),
		ConsumptionKWh: 23.5,
		VoltageV:       221.9,
		CurrentA:       101.28,
		PowerFactor:    0.954,
		SourceStatus:   "OK",
	}, readings[0])
}

func TestParseReadings_ColumnsInAnyOrderWithBOM_AreAccepted(t *testing.T) {
	input := utf8BOM + "status,power_factor,current_a,voltage_v,consumption_kwh,timestamp,meter_id\n" +
		"OK,0.9,10,220,5,2026-01-05 00:00:00,TEST-A\n"

	readings, err := ParseReadings(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, readings, 1)
	assert.Equal(t, "TEST-A", readings[0].MeterID)
	assert.Equal(t, 220.0, readings[0].VoltageV)
}

// Ingestion must not clean away future anomalies: values that are physically
// unusual but well-formed are loaded unchanged.
func TestParseReadings_UnusualButWellFormedValues_AreKept(t *testing.T) {
	input := readingsHeaderLine +
		"TEST-A,2026-01-05 00:00:00,0,0,-3.5,1.7,OK\n" +
		"TEST-A,2026-01-05 01:00:00,9999.25,480,0.001,-0.2,ALERT\n" +
		"TEST-A,2026-01-05 02:00:00,1e3,+220,.5,0.58,OK\n"

	readings, err := ParseReadings(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, readings, 3)
	assert.Equal(t, -3.5, readings[0].CurrentA)
	assert.Equal(t, 1.7, readings[0].PowerFactor)
	assert.Equal(t, "ALERT", readings[1].SourceStatus)
	assert.Equal(t, 1000.0, readings[2].ConsumptionKWh)
}

func TestParseReadings_MalformedValues_ReportLineAndColumn(t *testing.T) {
	tests := map[string]struct {
		row        string
		wantColumn string
		wantReason string
	}{
		"non-numeric voltage":   {"TEST-A,2026-01-05 00:00:00,1,abc,1,0.9,OK", "voltage_v", "not a decimal number"},
		"NaN consumption":       {"TEST-A,2026-01-05 00:00:00,NaN,220,1,0.9,OK", "consumption_kwh", "not a decimal number"},
		"infinite current":      {"TEST-A,2026-01-05 00:00:00,1,220,Inf,0.9,OK", "current_a", "not a decimal number"},
		"hexadecimal float":     {"TEST-A,2026-01-05 00:00:00,1,220,1,0x1p-2,OK", "power_factor", "not a decimal number"},
		"overflowing number":    {"TEST-A,2026-01-05 00:00:00,1e400,220,1,0.9,OK", "consumption_kwh", "out of range"},
		"invalid timestamp":     {"TEST-A,2026-13-05 00:00:00,1,220,1,0.9,OK", "timestamp", "invalid timestamp"},
		"timestamp with offset": {"TEST-A,2026-01-05T00:00:00Z,1,220,1,0.9,OK", "timestamp", "invalid timestamp"},
		"missing meter id":      {",2026-01-05 00:00:00,1,220,1,0.9,OK", "meter_id", "value is required"},
		"padded meter id":       {" TEST-A,2026-01-05 00:00:00,1,220,1,0.9,OK", "meter_id", "whitespace"},
		"missing status":        {"TEST-A,2026-01-05 00:00:00,1,220,1,0.9,", "status", "value is required"},
		"missing measurement":   {"TEST-A,2026-01-05 00:00:00,,220,1,0.9,OK", "consumption_kwh", "value is required"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseReadings(strings.NewReader(readingsHeaderLine + tc.row + "\n"))

			var verr *ValidationError
			require.ErrorAs(t, err, &verr)
			require.NotEmpty(t, verr.Errors)
			assert.Equal(t, 2, verr.Errors[0].Line)
			assert.Equal(t, tc.wantColumn, verr.Errors[0].Column)
			assert.Contains(t, verr.Errors[0].Reason, tc.wantReason)
		})
	}
}

func TestParseReadings_SeveralBadRows_ReportsAllOfThem(t *testing.T) {
	input := readingsHeaderLine +
		"TEST-A,2026-01-05 00:00:00,1,220,1,0.9,OK\n" +
		"TEST-A,bad,1,220,1,0.9,OK\n" +
		"TEST-A,2026-01-05 02:00:00,x,220,1,0.9,OK\n"

	_, err := ParseReadings(strings.NewReader(input))

	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Len(t, verr.Errors, 2)
	assert.Equal(t, 3, verr.Errors[0].Line)
	assert.Equal(t, 4, verr.Errors[1].Line)
}

func TestParseReadings_ManyBadRows_ReportsFirst25AndCountsTheRest(t *testing.T) {
	var b strings.Builder
	b.WriteString(readingsHeaderLine)
	for i := 0; i < 30; i++ {
		b.WriteString("TEST-A,,1,220,1,0.9,OK\n") // missing timestamp on every row
	}

	_, err := ParseReadings(strings.NewReader(b.String()))

	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	assert.Len(t, verr.Errors, 25)
	assert.Equal(t, 5, verr.Omitted)
	assert.Equal(t, "timestamp", verr.Errors[0].Column)
	assert.Contains(t, verr.Errors[0].Reason, "value is required")
	assert.Contains(t, err.Error(), "30 invalid value(s)")
	assert.Contains(t, err.Error(), "and 5 more")
}

func TestParseReadings_DuplicateMeterAndTimestamp_IsRejected(t *testing.T) {
	input := readingsHeaderLine +
		"TEST-A,2026-01-05 00:00:00,1,220,1,0.9,OK\n" +
		"TEST-B,2026-01-05 00:00:00,1,220,1,0.9,OK\n" +
		"TEST-A,2026-01-05 00:00:00,2,221,1,0.9,OK\n"

	_, err := ParseReadings(strings.NewReader(input))

	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Len(t, verr.Errors, 1)
	assert.Equal(t, 4, verr.Errors[0].Line)
	assert.Contains(t, verr.Errors[0].Reason, "duplicate reading for meter TEST-A")
	assert.Contains(t, verr.Errors[0].Reason, "line 2")
}

func TestParseReadings_WrongFieldCount_IsReportedWithLine(t *testing.T) {
	input := readingsHeaderLine + "TEST-A,2026-01-05 00:00:00,1,220\n"

	_, err := ParseReadings(strings.NewReader(input))

	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	assert.Equal(t, 2, verr.Errors[0].Line)
	assert.Contains(t, verr.Errors[0].Reason, "wrong number of fields")
}

func TestParseReadings_InvalidHeader_IsRejected(t *testing.T) {
	tests := map[string]struct {
		header string
		want   string
	}{
		"missing column":    {"meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor", `missing column "status"`},
		"unexpected column": {"meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status,notes", `unexpected column "notes"`},
		"duplicate column":  {"meter_id,meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status", `duplicate column "meter_id"`},
		"renamed column":    {"meter,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status", `missing column "meter_id"`},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseReadings(strings.NewReader(tc.header + "\n"))

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid header")
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestParseReadings_EmptyInputs_AreRejected(t *testing.T) {
	_, err := ParseReadings(strings.NewReader(""))
	require.ErrorContains(t, err, "file is empty")

	_, err = ParseReadings(strings.NewReader(readingsHeaderLine))
	require.ErrorContains(t, err, "contains no data rows")
}

const eventsHeaderLine = "meter_id,event_timestamp,event_type,description\n"

func TestParseEvents_ValidRows_AcceptMinuteAndSecondPrecision(t *testing.T) {
	input := eventsHeaderLine +
		"TEST-A,2026-01-05 14:00,OPERATIONAL_CHANGE,New line activated\n" +
		"TEST-B,2026-01-06 08:30:15,SOME_FUTURE_TYPE,\"Description, with comma\"\n"

	events, err := ParseEvents(strings.NewReader(input))

	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, Event{
		MeterID:     "TEST-A",
		Timestamp:   time.Date(2026, 1, 5, 14, 0, 0, 0, time.UTC),
		Type:        "OPERATIONAL_CHANGE",
		Description: "New line activated",
	}, events[0])
	assert.Equal(t, "SOME_FUTURE_TYPE", events[1].Type, "event types are not restricted to a fixed list")
	assert.Equal(t, "Description, with comma", events[1].Description)
}

func TestParseEvents_HeaderOnly_IsValidAndEmpty(t *testing.T) {
	events, err := ParseEvents(strings.NewReader(eventsHeaderLine))

	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestParseEvents_InvalidRows_AreRejected(t *testing.T) {
	tests := map[string]struct {
		rows string
		want string
	}{
		"missing type":        {"TEST-A,2026-01-05 14:00,,Something\n", "column event_type: value is required"},
		"missing description": {"TEST-A,2026-01-05 14:00,UNKNOWN,\n", "column description: value is required"},
		"bad timestamp":       {"TEST-A,05/01/2026 14:00,UNKNOWN,Something\n", "column event_timestamp: invalid timestamp"},
		"duplicate event": {
			"TEST-A,2026-01-05 14:00,UNKNOWN,Something\nTEST-A,2026-01-05 14:00,UNKNOWN,Something\n",
			"line 3: duplicate event (first seen on line 2)",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseEvents(strings.NewReader(eventsHeaderLine + tc.rows))

			var verr *ValidationError
			require.ErrorAs(t, err, &verr)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestParseDir_MissingFile_ReportsPath(t *testing.T) {
	dir := t.TempDir()

	_, err := ParseDir(dir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), ReadingsFile)
	assert.False(t, errors.As(err, new(*ValidationError)), "a missing file is not a validation error")
}

func TestParseDir_Fixtures_ParsesBothFiles(t *testing.T) {
	dataset, err := ParseDir("testdata/valid")

	require.NoError(t, err)
	assert.Len(t, dataset.Readings, 6)
	assert.Len(t, dataset.Events, 2)
	assert.Equal(t, []string{"TEST-A", "TEST-B", "TEST-C"}, dataset.MeterIDs())
}

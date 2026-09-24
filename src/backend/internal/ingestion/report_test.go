package ingestion

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func at(hour int) time.Time { return time.Date(2026, 1, 5, hour, 0, 0, 0, time.UTC) }

func TestBuildReport_SummarizesCoverageGapsAndDistributions(t *testing.T) {
	d := Dataset{
		Readings: []Reading{
			{MeterID: "TEST-A", Timestamp: at(2), SourceStatus: "OK"},
			{MeterID: "TEST-A", Timestamp: at(0), SourceStatus: "OK"}, // unsorted input
			{MeterID: "TEST-A", Timestamp: at(1), SourceStatus: "OK"},
			{MeterID: "TEST-B", Timestamp: at(0), SourceStatus: "OK"},
			{MeterID: "TEST-B", Timestamp: at(3), SourceStatus: "ALERT"}, // two missing hours
			{MeterID: "TEST-B", Timestamp: at(3).Add(30 * time.Minute), SourceStatus: "OK"},
		},
		Events: []Event{
			{MeterID: "TEST-A", Type: "UNKNOWN"},
			{MeterID: "TEST-Z", Type: "DATA_QUALITY"}, // meter without readings
		},
	}

	rep := BuildReport(d)

	assert.Equal(t, 6, rep.Readings)
	assert.Equal(t, 2, rep.Events)
	assert.Equal(t, 3, rep.Meters)
	assert.Equal(t, map[string]int{"TEST-A": 3, "TEST-B": 3}, rep.ReadingsPerMeter)
	assert.Equal(t, at(0), rep.FirstReading)
	assert.Equal(t, at(3).Add(30*time.Minute), rep.LastReading)
	assert.Equal(t, 2, rep.HourlyGaps)
	assert.Equal(t, 1, rep.IrregularSteps)
	assert.Equal(t, map[string]int{"OK": 5, "ALERT": 1}, rep.SourceStatuses)
	assert.Equal(t, map[string]int{"UNKNOWN": 1, "DATA_QUALITY": 1}, rep.EventTypes)
	assert.Equal(t, []string{"TEST-Z"}, rep.EventsWithoutData)
}

func TestParseSourceTimestamp_KeepsWallClockWithoutConversion(t *testing.T) {
	got, err := ParseSourceTimestamp("2026-09-14 23:00:00")

	assert.NoError(t, err)
	assert.Equal(t, "2026-09-14 23:00:00", got.Format(time.DateTime))
}

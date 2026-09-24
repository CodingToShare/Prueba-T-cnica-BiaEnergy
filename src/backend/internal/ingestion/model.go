// Package ingestion parses, validates and loads the challenge source files
// (readings.csv and events.csv) into PostgreSQL. It never classifies data:
// unusual values are kept because they are analytical signals.
package ingestion

import "time"

// Reading is one hourly measurement from readings.csv.
type Reading struct {
	MeterID        string
	Timestamp      time.Time // timezone-naive source wall clock (see ParseSourceTimestamp)
	ConsumptionKWh float64
	VoltageV       float64
	CurrentA       float64
	PowerFactor    float64
	SourceStatus   string // the CSV status column; never the analytical meter health
}

// Event is one known operational or data event from events.csv.
type Event struct {
	MeterID     string
	Timestamp   time.Time // timezone-naive source wall clock
	Type        string
	Description string
}

// Dataset is a fully parsed and validated pair of source files.
type Dataset struct {
	Readings []Reading
	Events   []Event
}

// MeterIDs returns the distinct meter identifiers referenced by readings and events, sorted.
func (d Dataset) MeterIDs() []string {
	seen := make(map[string]struct{})
	for _, r := range d.Readings {
		seen[r.MeterID] = struct{}{}
	}
	for _, e := range d.Events {
		seen[e.MeterID] = struct{}{}
	}
	return sortedKeys(seen)
}

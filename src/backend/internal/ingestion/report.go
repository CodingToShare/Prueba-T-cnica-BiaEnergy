package ingestion

import (
	"slices"
	"time"
)

// Report summarizes structural facts of a parsed dataset: counts, coverage,
// hourly continuity and the source status distribution. It describes the
// data; it does not judge whether values are anomalous.
type Report struct {
	Readings          int
	Events            int
	Meters            int
	ReadingsPerMeter  map[string]int
	FirstReading      time.Time
	LastReading       time.Time
	HourlyGaps        int // missing hours between consecutive readings of a meter
	IrregularSteps    int // consecutive readings not aligned to a whole number of hours
	SourceStatuses    map[string]int
	EventTypes        map[string]int
	EventsWithoutData []string // meter IDs that have events but no readings
}

// BuildReport computes the Report for d.
func BuildReport(d Dataset) Report {
	rep := Report{
		Readings:         len(d.Readings),
		Events:           len(d.Events),
		Meters:           len(d.MeterIDs()),
		ReadingsPerMeter: make(map[string]int),
		SourceStatuses:   make(map[string]int),
		EventTypes:       make(map[string]int),
	}

	byMeter := make(map[string][]time.Time)
	for _, r := range d.Readings {
		rep.ReadingsPerMeter[r.MeterID]++
		rep.SourceStatuses[r.SourceStatus]++
		byMeter[r.MeterID] = append(byMeter[r.MeterID], r.Timestamp)
		if rep.FirstReading.IsZero() || r.Timestamp.Before(rep.FirstReading) {
			rep.FirstReading = r.Timestamp
		}
		if r.Timestamp.After(rep.LastReading) {
			rep.LastReading = r.Timestamp
		}
	}

	for _, stamps := range byMeter {
		slices.SortFunc(stamps, func(a, b time.Time) int { return a.Compare(b) })
		for i := 1; i < len(stamps); i++ {
			step := stamps[i].Sub(stamps[i-1])
			switch {
			case step%time.Hour != 0:
				rep.IrregularSteps++
			case step > time.Hour:
				rep.HourlyGaps += int(step/time.Hour) - 1
			}
		}
	}

	eventMeters := make(map[string]struct{})
	for _, e := range d.Events {
		rep.EventTypes[e.Type]++
		if _, ok := byMeter[e.MeterID]; !ok {
			eventMeters[e.MeterID] = struct{}{}
		}
	}
	rep.EventsWithoutData = sortedKeys(eventMeters)
	return rep
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

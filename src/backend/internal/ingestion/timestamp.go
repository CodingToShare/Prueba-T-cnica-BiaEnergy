package ingestion

import (
	"fmt"
	"time"
)

// Source timestamp layouts. readings.csv uses seconds, events.csv uses minutes;
// both layouts are accepted for either file.
var sourceTimestampLayouts = []string{"2006-01-02 15:04:05", "2006-01-02 15:04"}

// ParseSourceTimestamp parses a timezone-naive source timestamp.
//
// The source files carry no offset, so the value is a wall-clock time whose
// zone is unknown (ADR-008). time.UTC is used only as a neutral carrier: no
// conversion is applied, and the value is stored in a PostgreSQL
// "timestamp without time zone" column that keeps the same wall clock.
// This is the single place where source timestamp semantics are applied.
func ParseSourceTimestamp(raw string) (time.Time, error) {
	for _, layout := range sourceTimestampLayouts {
		if t, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q (expected YYYY-MM-DD HH:MM:SS or YYYY-MM-DD HH:MM)", raw)
}

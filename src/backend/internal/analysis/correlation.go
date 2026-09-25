package analysis

import (
	"cmp"
	"slices"
	"time"
)

// eventIndex holds each meter's events in time order.
type eventIndex map[string][]Event

func newEventIndex(events []Event) eventIndex {
	idx := make(eventIndex)
	for _, ev := range events {
		idx[ev.MeterID] = append(idx[ev.MeterID], ev)
	}
	for _, evs := range idx {
		slices.SortFunc(evs, func(a, b Event) int {
			return cmp.Or(
				a.Timestamp.Compare(b.Timestamp),
				cmp.Compare(a.Type, b.Type),
				cmp.Compare(a.Description, b.Description),
			)
		})
	}
	return idx
}

// near returns the meter's events within window of t, in time order.
func (idx eventIndex) near(meterID string, t time.Time, window time.Duration) []Event {
	evs := idx[meterID]
	from, _ := slices.BinarySearchFunc(evs, t.Add(-window), func(ev Event, target time.Time) int {
		return ev.Timestamp.Compare(target)
	})
	var out []Event
	for _, ev := range evs[from:] {
		if ev.Timestamp.After(t.Add(window)) {
			break
		}
		out = append(out, ev)
	}
	return out
}

// correlate relates the events near an episode's onset to it. The decision
// uses only the structured event type and the observed behavior; the
// description is kept verbatim as evidence and never interpreted.
func correlate(k kind, onset time.Time, events []Event) []RelatedEvent {
	out := make([]RelatedEvent, 0, len(events))
	for _, ev := range events {
		out = append(out, RelatedEvent{
			Timestamp:   ev.Timestamp,
			Type:        ev.Type,
			Description: ev.Description,
			Offset:      ev.Timestamp.Sub(onset),
			Role:        roleOf(ev.Type, k),
		})
	}
	return out
}

// roleOf encodes event semantics (OD-08):
//   - OPERATIONAL_CHANGE can explain a consumption shift in either direction;
//   - SCHEDULED_OUTAGE can explain a consumption drop, never a rise;
//   - DATA_QUALITY corroborates an inconsistency finding and explains nothing;
//   - UNKNOWN and unrecognized types are context only.
func roleOf(t EventType, k kind) EventRole {
	switch {
	case k == kindInconsistent && t == EventDataQuality:
		return RoleCorroborates
	case k == kindLoadUp && t == EventOperationalChange,
		k == kindLoadDown && (t == EventOperationalChange || t == EventScheduledOutage):
		return RoleExplains
	default:
		return RoleContext
	}
}

// explanation returns the explaining event closest to the onset, if any.
// Ties keep the earlier event.
func explanation(related []RelatedEvent) (RelatedEvent, bool) {
	var best RelatedEvent
	found := false
	for _, re := range related {
		if re.Role != RoleExplains {
			continue
		}
		if !found || absDuration(re.Offset) < absDuration(best.Offset) {
			best, found = re, true
		}
	}
	return best, found
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

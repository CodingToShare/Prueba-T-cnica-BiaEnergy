package analysis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleOf_EncodesEventSemanticsByTypeAndObservedBehavior(t *testing.T) {
	tests := []struct {
		event EventType
		kind  kind
		want  EventRole
	}{
		{EventOperationalChange, kindLoadUp, RoleExplains},
		{EventOperationalChange, kindLoadDown, RoleExplains},
		{EventScheduledOutage, kindLoadDown, RoleExplains},
		{EventScheduledOutage, kindLoadUp, RoleContext},         // an outage cannot explain a rise
		{EventUnknown, kindLoadUp, RoleContext},                 // "no event reported" is not an explanation
		{EventDataQuality, kindInconsistent, RoleCorroborates},  // supports, never creates
		{EventDataQuality, kindLoadUp, RoleContext},             // does not explain a consumption change
		{EventOperationalChange, kindInconsistent, RoleContext}, // operations do not explain bad measurements
		{EventType("FIRMWARE_UPDATE"), kindLoadUp, RoleContext}, // unrecognized types are kept, never explain
		{EventType("FIRMWARE_UPDATE"), kindInconsistent, RoleContext},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, roleOf(tc.event, tc.kind), "%s for kind %d", tc.event, tc.kind)
	}
}

func TestEventIndex_NearIsPerMeterAndWithinTheWindow(t *testing.T) {
	onset := at(8, 10)
	idx := newEventIndex([]Event{
		{MeterID: "TEST-A", Timestamp: onset.Add(4 * time.Hour), Type: EventUnknown},
		{MeterID: "TEST-A", Timestamp: onset.Add(-3 * time.Hour), Type: EventOperationalChange},
		{MeterID: "TEST-B", Timestamp: onset, Type: EventOperationalChange},
		{MeterID: "TEST-A", Timestamp: onset.Add(-5 * 24 * time.Hour), Type: EventScheduledOutage},
		{MeterID: "TEST-A", Timestamp: onset.Add(time.Hour), Type: EventDataQuality},
	})

	near := idx.near("TEST-A", onset, 3*time.Hour)

	require.Len(t, near, 2, "window is inclusive; other meters and distant events are excluded")
	assert.Equal(t, EventOperationalChange, near[0].Type)
	assert.Equal(t, EventDataQuality, near[1].Type)
	assert.Empty(t, idx.near("TEST-C", onset, 3*time.Hour))
}

func TestCorrelate_PreservesEventsAndOffsets(t *testing.T) {
	onset := at(8, 10)
	events := []Event{{MeterID: "TEST-A", Timestamp: onset.Add(-90 * time.Minute), Type: "FIRMWARE_UPDATE", Description: "free text kept verbatim"}}

	related := correlate(kindLoadUp, onset, events)

	require.Len(t, related, 1)
	assert.Equal(t, RelatedEvent{
		Timestamp:   onset.Add(-90 * time.Minute),
		Type:        "FIRMWARE_UPDATE",
		Description: "free text kept verbatim",
		Offset:      -90 * time.Minute,
		Role:        RoleContext,
	}, related[0])
}

func TestExplanation_PicksTheExplainingEventClosestToOnset(t *testing.T) {
	related := []RelatedEvent{
		{Type: EventUnknown, Offset: 0, Role: RoleContext},
		{Type: EventOperationalChange, Offset: -2 * time.Hour, Role: RoleExplains},
		{Type: EventScheduledOutage, Offset: time.Hour, Role: RoleExplains},
	}

	ex, ok := explanation(related)

	require.True(t, ok)
	assert.Equal(t, EventScheduledOutage, ex.Type)

	_, ok = explanation(related[:1])
	assert.False(t, ok, "context events never explain")
}

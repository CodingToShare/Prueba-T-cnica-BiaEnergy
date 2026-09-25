package jsontime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource_IsWrittenWithoutAnyOffset(t *testing.T) {
	wall := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)

	data, err := json.Marshal(struct {
		At Source `json:"at"`
	}{Source(wall)})

	require.NoError(t, err)
	assert.JSONEq(t, `{"at":"2026-09-12T14:00:00"}`, string(data))
	assert.NotContains(t, string(data), "Z", "a source time must not claim UTC")
}

func TestSource_KeepsTheWallClockEvenInAnotherLocation(t *testing.T) {
	wall := time.Date(2026, 9, 12, 14, 0, 0, 0, time.FixedZone("carrier", -5*3600))

	data, err := json.Marshal(Source(wall))

	require.NoError(t, err)
	assert.Equal(t, `"2026-09-12T14:00:00"`, string(data), "the wall clock is not converted")
}

func TestSource_RoundTripAndParsing(t *testing.T) {
	var s Source
	require.NoError(t, json.Unmarshal([]byte(`"2026-09-08T00:00:00"`), &s))
	assert.Equal(t, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), time.Time(s))

	for _, bad := range []string{"2026-09-08T00:00:00Z", "2026-09-08T00:00:00-05:00", "2026-09-08 00:00:00", "2026-09-08", "tomorrow", ""} {
		_, err := ParseSource(bad)
		assert.Error(t, err, bad)
	}
	assert.Error(t, json.Unmarshal([]byte(`12`), &s))
	assert.Nil(t, SourcePtr(nil))
}

func TestSystem_IsRFC3339InUTC(t *testing.T) {
	instant := time.Date(2026, 9, 24, 18, 18, 0, 0, time.FixedZone("COT", -5*3600))

	data, err := json.Marshal(System(instant))
	require.NoError(t, err)
	assert.Equal(t, `"2026-09-24T23:18:00Z"`, string(data))

	fractional, err := json.Marshal(System(time.Date(2026, 9, 24, 23, 18, 0, 123456000, time.UTC)))
	require.NoError(t, err)
	assert.Equal(t, `"2026-09-24T23:18:00.123456Z"`, string(fractional))

	var back System
	require.NoError(t, json.Unmarshal(data, &back))
	assert.True(t, time.Time(back).Equal(instant))
	assert.Error(t, json.Unmarshal([]byte(`"2026-09-24 23:18:00"`), &back))

	nilPtr, err := json.Marshal(SystemPtr(nil))
	require.NoError(t, err)
	assert.Equal(t, "null", string(nilPtr))
}

func TestSource_AuditRejectsFractionsAndNonCanonicalHour(t *testing.T) {
	for _, raw := range []string{"2026-09-08T00:00:00.123", "2026-09-08T00:00:00,123", "2026-09-08T0:00:00"} {
		_, err := ParseSource(raw)
		require.Error(t, err, raw)
	}
}

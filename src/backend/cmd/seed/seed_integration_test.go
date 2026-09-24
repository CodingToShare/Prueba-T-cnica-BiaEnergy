//go:build integration

package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bia-energy.local/backend/internal/config"
	"bia-energy.local/backend/internal/platform/postgres/pgtest"
)

// The command resolves data/input itself and can be re-run safely.
func TestSeed_SuppliedDatasetTwice_KeepsExactCounts(t *testing.T) {
	db := pgtest.StartMigrated(t)
	cfg := config.Config{DatabaseURL: db.URL}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for attempt := 1; attempt <= 2; attempt++ {
		require.NoError(t, run(context.Background(), cfg, "", logger), "import attempt %d", attempt)
		assert.Equal(t, 12, pgtest.Count(t, db.Pool, "meters"), "attempt %d", attempt)
		assert.Equal(t, 4032, pgtest.Count(t, db.Pool, "readings"), "attempt %d", attempt)
		assert.Equal(t, 4, pgtest.Count(t, db.Pool, "events"), "attempt %d", attempt)
	}
}

func TestSeed_InvalidInput_WritesNothing(t *testing.T) {
	db := pgtest.StartMigrated(t)
	cfg := config.Config{DatabaseURL: db.URL}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := run(context.Background(), cfg, t.TempDir(), logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "readings.csv")
	assert.Equal(t, 0, pgtest.Count(t, db.Pool, "readings"))
}

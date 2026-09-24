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

func TestMigrateCommand_UpStatusDownUp(t *testing.T) {
	ctx := context.Background()
	db := pgtest.Start(t)
	cfg := config.Config{DatabaseURL: db.URL}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	require.NoError(t, run(ctx, cfg, "", "up", logger))
	assert.Equal(t, 0, pgtest.Count(t, db.Pool, "readings"), "tables exist and are empty")
	require.NoError(t, run(ctx, cfg, "", "status", logger))
	require.NoError(t, run(ctx, cfg, "", "down", logger))
	require.NoError(t, run(ctx, cfg, "", "up", logger))
	assert.Equal(t, 0, pgtest.Count(t, db.Pool, "meters"))

	err := run(ctx, cfg, "", "sideways", logger)
	assert.ErrorContains(t, err, `unknown command "sideways"`)
}

//go:build integration

// Package pgtest starts disposable PostgreSQL containers for integration tests.
// It is compiled only with the "integration" build tag.
package pgtest

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"bia-energy.local/backend/internal/ingestion"
	"bia-energy.local/backend/internal/platform/postgres"
	"bia-energy.local/backend/internal/workspace"
)

// Image is the PostgreSQL image validated in docs/quality/environment-readiness.md.
const Image = "postgres:18.6-alpine"

// windowsDefaultDockerHost is the Docker SDK's default host on Windows.
const windowsDefaultDockerHost = "npipe:////./pipe/docker_engine"

func init() {
	// Without DOCKER_HOST, testcontainers detects Docker on Windows by calling
	// os.Stat on the \\.\pipe\docker_engine named pipe. That stat fails while
	// the pipe is busy, which happens when several test packages start in
	// parallel, and detection then aborts ("rootless Docker is not supported
	// on Windows"). Supplying the same default host explicitly makes detection
	// deterministic; testcontainers still verifies it with a Docker info call.
	// An explicit DOCKER_HOST set by the developer is always respected.
	if runtime.GOOS == "windows" && os.Getenv("DOCKER_HOST") == "" {
		_ = os.Setenv("DOCKER_HOST", windowsDefaultDockerHost)
	}
}

// DB is a running disposable database.
type DB struct {
	Pool      *pgxpool.Pool
	URL       string
	Container *tcpostgres.PostgresContainer
}

// Start runs an empty PostgreSQL container that is removed when the test ends.
// Credentials are throwaway values for the disposable container only.
func Start(t *testing.T) *DB {
	t.Helper()
	return StartWithPassword(t, "bia_test_only")
}

// StartWithPassword is Start with a caller-chosen throwaway password, used by
// tests that assert the password never appears in output.
func StartWithPassword(t *testing.T, password string) *DB {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, Image,
		tcpostgres.WithDatabase("bia_test"),
		tcpostgres.WithUsername("bia_test"),
		tcpostgres.WithPassword(password),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err, "start PostgreSQL container")

	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := postgres.Connect(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return &DB{Pool: pool, URL: url, Container: ctr}
}

// StartMigrated runs a container with every migration applied.
func StartMigrated(t *testing.T) *DB {
	t.Helper()
	db := Start(t)
	m := NewMigrator(t, db.Pool)
	_, err := m.Up(context.Background())
	require.NoError(t, err, "apply migrations")
	return db
}

// NewMigrator returns a migrator for the repository's database/migrations.
func NewMigrator(t *testing.T, pool *pgxpool.Pool) *postgres.Migrator {
	t.Helper()
	dir, err := workspace.FindDir("database/migrations")
	require.NoError(t, err)
	m, err := postgres.NewMigrator(pool, dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// Count returns SELECT count(*) for a trusted table name.
func Count(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n))
	return n
}

// StartSeeded runs a migrated container with the supplied dataset
// (data/input) loaded through the Phase 01 ingestion.
func StartSeeded(t *testing.T) *DB {
	t.Helper()
	db := StartMigrated(t)
	Seed(t, db.Pool)
	return db
}

// Seed loads the supplied dataset into pool.
func Seed(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	dir, err := workspace.FindDir("data/input")
	require.NoError(t, err)
	ds, err := ingestion.ParseDir(dir)
	require.NoError(t, err)
	_, err = ingestion.Load(context.Background(), pool, ds)
	require.NoError(t, err, "load the supplied dataset")
}

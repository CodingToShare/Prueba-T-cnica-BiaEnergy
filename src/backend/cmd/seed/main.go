// Command seed imports the challenge source dataset (data/input/readings.csv
// and data/input/events.csv) into PostgreSQL. It does not generate data.
//
// Both files are fully validated before anything is written, the load runs
// in one transaction, and re-running it with the same files changes nothing.
//
// Usage: seed [-input-dir path]
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"bia-energy.local/backend/internal/config"
	"bia-energy.local/backend/internal/ingestion"
	"bia-energy.local/backend/internal/platform/postgres"
	"bia-energy.local/backend/internal/workspace"
)

func main() {
	inputDir := flag.String("input-dir", "", "directory containing readings.csv and events.csv (default: data/input found from the working directory)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error:\n%v\n", err)
		os.Exit(2)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	if err := run(ctx, cfg, *inputDir, logger); err != nil {
		logger.Error("import failed; nothing was written", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, inputDir string, logger *slog.Logger) error {
	start := time.Now()
	if inputDir == "" {
		found, err := workspace.FindDir("data/input")
		if err != nil {
			return err
		}
		inputDir = found
	}

	dataset, err := ingestion.ParseDir(inputDir)
	if err != nil {
		return err
	}
	report := ingestion.BuildReport(dataset)
	logger.Info("source files validated",
		"input_dir", inputDir,
		"readings", report.Readings,
		"events", report.Events,
		"meters", report.Meters,
		"first_reading", report.FirstReading.Format(time.DateTime),
		"last_reading", report.LastReading.Format(time.DateTime),
		"hourly_gaps", report.HourlyGaps,
		"irregular_steps", report.IrregularSteps,
		"source_statuses", formatCounts(report.SourceStatuses),
		"event_types", formatCounts(report.EventTypes),
	)

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	result, err := ingestion.Load(ctx, pool, dataset)
	if err != nil {
		return err
	}
	logger.Info("import complete",
		"meters", counts(result.Meters),
		"readings", counts(result.Readings),
		"events", counts(result.Events),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

func counts(c ingestion.Counts) string {
	return fmt.Sprintf("inserted=%d updated=%d unchanged=%d", c.Inserted, c.Updated, c.Unchanged)
}

func formatCounts(m map[string]int) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%d", k, v))
	}
	slices.Sort(parts)
	return strings.Join(parts, " ")
}

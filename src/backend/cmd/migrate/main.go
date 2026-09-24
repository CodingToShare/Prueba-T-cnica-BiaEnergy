// Command migrate applies or inspects the database schema migrations in
// database/migrations using goose (pinned in go.mod).
//
// Usage: migrate [-dir path] up|down|status
//
// "down" rolls back the most recent migration only.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"bia-energy.local/backend/internal/config"
	"bia-energy.local/backend/internal/platform/postgres"
	"bia-energy.local/backend/internal/workspace"
)

func main() {
	dir := flag.String("dir", "", "migrations directory (default: database/migrations found from the working directory)")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: migrate [-dir path] up|down|status")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error:\n%v\n", err)
		os.Exit(2)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	if err := run(ctx, cfg, *dir, flag.Arg(0), logger); err != nil {
		logger.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, dir, command string, logger *slog.Logger) error {
	if dir == "" {
		found, err := workspace.FindDir("database/migrations")
		if err != nil {
			return err
		}
		dir = found
	}

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	migrator, err := postgres.NewMigrator(pool, dir)
	if err != nil {
		return err
	}
	defer migrator.Close()

	switch command {
	case "up":
		applied, err := migrator.Up(ctx)
		if err != nil {
			return err
		}
		logger.Info("migrations applied", "count", len(applied), "versions", applied)
	case "down":
		rolledBack, err := migrator.Down(ctx)
		if err != nil {
			return err
		}
		logger.Info("migration rolled back", "version", rolledBack)
	case "status":
		status, err := migrator.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range status {
			fmt.Printf("%-8s %05d  %s\n", s.State, s.Source.Version, s.Source.Path)
		}
	default:
		return fmt.Errorf("unknown command %q (use up, down or status)", command)
	}
	return nil
}

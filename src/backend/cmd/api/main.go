// Command api runs the Bia Energy HTTP service: the versioned product API
// (/api/v1), the operational endpoints /healthz and /readyz, and the
// in-process analysis worker.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"bia-energy.local/backend/internal/analysis"
	"bia-energy.local/backend/internal/analysisrun"
	"bia-energy.local/backend/internal/anomaly"
	"bia-energy.local/backend/internal/auth"
	"bia-energy.local/backend/internal/config"
	"bia-energy.local/backend/internal/dashboard"
	"bia-energy.local/backend/internal/explanation"
	"bia-energy.local/backend/internal/httpapi"
	"bia-energy.local/backend/internal/meter"
	"bia-energy.local/backend/internal/platform/postgres"
)

const shutdownTimeout = 10 * time.Second

// ollamaStageBudget is the time the explanation stage may spend on a local
// model, on top of the run timeout. Findings still unexplained when it runs
// out get the deterministic text; the run itself is not failed.
const ollamaStageBudget = 3 * time.Minute

// explanationSetup selects the explanation providers (ADR-006): the
// deterministic provider alone, or Ollama with the deterministic provider as
// fallback.
func explanationSetup(cfg config.ExplanationConfig) (analysisrun.Explainers, analysisrun.Options, error) {
	deterministic := explanation.Deterministic{}
	if cfg.Provider != config.ProviderOllama {
		return analysisrun.Explainers{Primary: deterministic, Settings: explanation.DeterministicSettings()}, analysisrun.Options{}, nil
	}
	ollama, err := explanation.NewOllama(explanation.OllamaConfig{BaseURL: cfg.OllamaBaseURL, Model: cfg.OllamaModel, Timeout: cfg.OllamaTimeout})
	if err != nil {
		return analysisrun.Explainers{}, analysisrun.Options{}, err
	}
	return analysisrun.Explainers{Primary: ollama, Fallback: deterministic, Settings: ollama.Settings()},
		analysisrun.Options{ExplanationTimeout: ollamaStageBudget}, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadAPI(os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error:\n%v\n", err)
		os.Exit(2)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	if err := run(ctx, cfg, logger, nil); err != nil {
		logger.Error("api stopped with an error", "error", err)
		os.Exit(1)
	}
}

// run serves HTTP and runs the analysis worker until ctx is cancelled, then
// stops accepting requests, stops the worker and returns. onListening, when
// non-nil, receives the bound address (used by tests that listen on port 0).
func run(ctx context.Context, cfg config.APIConfig, logger *slog.Logger, onListening func(net.Addr)) error {
	authManager, err := auth.NewManager(cfg.Auth, nil)
	if err != nil {
		return err
	}
	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	logger.Info("connected to PostgreSQL", "database", cfg.RedactedDatabaseURL())

	engine, err := analysis.New(analysis.DefaultConfig())
	if err != nil {
		return err
	}
	explainers, opts, err := explanationSetup(cfg.Explanation)
	if err != nil {
		return err
	}
	logger.Info("explanation provider configured", "provider", explainers.Settings.Provider, "model", explainers.Settings.Model, "prompt_version", explainers.Settings.PromptVersion)
	runs, err := analysisrun.NewService(pool, engine, explainers, analysis.EngineVersion, engine.Config(), logger, opts)
	if err != nil {
		return err
	}
	if _, err := runs.RecoverInterrupted(ctx); err != nil {
		return err
	}

	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.HTTPAddr, err)
	}

	server := &http.Server{
		Handler: httpapi.NewRouter(httpapi.Deps{
			Logger: logger, DB: pool, Auth: authManager, Runs: runs,
			Meters: meter.NewService(pool), Anomalies: anomaly.NewService(pool), Dashboard: dashboard.NewService(pool),
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	// The worker outlives the signal context: it is stopped explicitly after
	// the HTTP server has stopped accepting requests.
	workerCtx, stopWorker := context.WithCancel(context.WithoutCancel(ctx))
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		runs.Work(workerCtx)
	}()
	var stopOnce sync.Once
	stopAnalysisWorker := func() {
		stopOnce.Do(func() {
			stopWorker()
			<-workerDone
			logger.Info("analysis worker stopped")
		})
	}
	defer stopAnalysisWorker()

	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	logger.Info("api listening", "addr", listener.Addr().String())
	if onListening != nil {
		onListening(listener.Addr())
	}

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutdown started")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	stopAnalysisWorker()
	logger.Info("shutdown complete")
	return nil
}

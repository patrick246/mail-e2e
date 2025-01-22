package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/patrick246/mail-e2e/internal/config"
	"github.com/patrick246/mail-e2e/internal/logging"
	"github.com/patrick246/mail-e2e/internal/metrics"
	"github.com/patrick246/mail-e2e/internal/monitoring"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fatal: %v", err)

		os.Exit(1)
	}
}

func run() error {
	log := logging.CreateLogger("main")

	cfg, err := config.Get()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	targetMonitors := make([]*monitoring.TargetMonitor, 0, len(cfg.Targets))
	prometheusMetrics := metrics.NewMetrics()

	for i := range cfg.Targets {
		prometheusMetrics.InitTarget(cfg.Targets[i].Name)

		tm := monitoring.NewTargetMonitor(&cfg.Targets[i], prometheusMetrics)
		go tm.Start(context.Background())
		targetMonitors = append(targetMonitors, tm)
	}

	registry, err := prometheusMetrics.Registry()
	if err != nil {
		return fmt.Errorf("prometheus registry: %w", err)
	}

	srv := metrics.NewServer(fmt.Sprintf(":%d", cfg.MetricsPort), registry)
	go func() {
		log.Info("listening", "port", cfg.MetricsPort)

		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen error", "port", cfg.MetricsPort, "error", err)

			os.Exit(1)
		}
	}()

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)

	<-interruptChan

	const shutdownTimeout = 30 * time.Second

	log.Info("shutting down", "timeout", shutdownTimeout.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	for _, tm := range targetMonitors {
		err := tm.Shutdown(shutdownCtx)
		if err != nil {
			log.Error("target monitor shutdown", "error", err)
		}
	}

	err = srv.Shutdown(shutdownCtx)
	if err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	return nil
}

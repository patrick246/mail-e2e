package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/patrick246/mail-e2e/internal/config"
	"github.com/patrick246/mail-e2e/internal/logging"
	"github.com/patrick246/mail-e2e/internal/metrics"
	"github.com/patrick246/mail-e2e/internal/monitoring"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var log = logging.CreateLogger("main")

func main() {
	cfg, err := config.Get()
	if err != nil {
		log.Fatalw("config error", "error", err)
	}

	srv := metrics.NewServer(fmt.Sprintf(":%d", cfg.MetricsPort))
	go func() {
		log.Infow("listening", "port", cfg.MetricsPort)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalw("listen error", "port", cfg.MetricsPort, "error", err)
		}
	}()

	var targetMonitors []*monitoring.TargetMonitor
	for _, target := range cfg.Targets {
		metrics.MailSent.WithLabelValues(target.Name).Add(0)
		metrics.MailSentError.WithLabelValues(target.Name).Add(0)
		metrics.MailReceived.WithLabelValues(target.Name).Add(0)
		metrics.MailReceivedError.WithLabelValues(target.Name).Add(0)

		tm := monitoring.NewTargetMonitor(target)
		go tm.Start()
		targetMonitors = append(targetMonitors, tm)
	}

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)

	<-interruptChan
	log.Infow("shutting down", "timeout", "30s")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, tm := range targetMonitors {
		err := tm.Shutdown(shutdownCtx)
		if err != nil {
			log.Errorw("target monitor shutdown", "error", err)
		}
	}

	err = srv.Shutdown(shutdownCtx)
	if err != nil {
		log.Fatalw("graceful shutdown failed", "error", err)
	}
}

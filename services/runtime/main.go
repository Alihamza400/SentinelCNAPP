package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/config"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/finding"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/metrics"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/queue"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/scanner"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/runtime/internal/api"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/runtime/internal/engine"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg := config.New()
	log := logging.New(
		cfg.GetDefault(config.ServiceName, "sentinel-runtime"),
		logging.LevelInfo,
		os.Stdout,
	)
	m := metrics.New("runtime_protection")

	log.Info("starting runtime protection service", "version", cfg.GetDefault(config.ServiceVersion, "0.1.0"))

	// ── NATS ───────────────────────────────────────────────
	natsURL := cfg.GetDefault(config.NATSURL, "nats://localhost:4222")
	natsQueue, err := queue.NewNATS(natsURL)
	if err != nil {
		return fmt.Errorf("connecting to NATS: %w", err)
	}
	defer natsQueue.Close()
	log.Info("connected to NATS", "url", natsURL)

	// ── Engine ─────────────────────────────────────────────
	emitter := scanner.NewEmitter(natsQueue, log, m, finding.SourceFalco)
	eng := engine.New(log, emitter, natsQueue)

	// ── HTTP API ─────────────────────────────────────────────
	runtimeAPI := api.NewServer(eng, log)
	port := cfg.GetDefault(config.ServicePort, "8080")

	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "runtime-protection",
		})
	})
	mux.Handle("/api/v1/runtime", runtimeAPI.Handler())
	mux.Handle("/api/v1/runtime/", runtimeAPI.Handler())

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	log.Info("listening for runtime events", "port", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

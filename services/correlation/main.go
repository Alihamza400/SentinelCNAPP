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
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/config"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/graph"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/metrics"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/queue"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/correlation/internal/api"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/correlation/internal/cache"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/correlation/internal/engine"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/correlation/internal/graphdb"
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
		cfg.GetDefault(config.ServiceName, "sentinel-correlation"),
		logging.LevelInfo,
		os.Stdout,
	)
	m := metrics.New("correlation")

	log.Info("starting correlation service",
		"version", cfg.GetDefault(config.ServiceVersion, "0.1.0"))

	// ── Neo4j ──────────────────────────────────────────────
	neo4jURI := cfg.GetDefault(config.Neo4jURI, "bolt://localhost:7687")
	neo4jUser := cfg.GetDefault(config.Neo4jUser, "neo4j")
	neo4jPassword := cfg.GetDefault(config.Neo4jPassword, "changeme")

	graphClient, err := graph.New(ctx, neo4jURI, neo4jUser, neo4jPassword)
	if err != nil {
		return fmt.Errorf("connecting to neo4j: %w", err)
	}
	defer graphClient.Close(ctx)
	log.Info("connected to neo4j", "uri", neo4jURI)

	graphWriter := graphdb.NewWriter(graphClient, log)
	graphReader := graphdb.NewReader(graphClient, log)

	// ── NATS ───────────────────────────────────────────────
	natsURL := cfg.GetDefault(config.NATSURL, "nats://localhost:4222")
	natsQueue, err := queue.NewNATS(natsURL)
	if err != nil {
		return fmt.Errorf("connecting to NATS: %w", err)
	}
	defer natsQueue.Close()
	log.Info("connected to NATS", "url", natsURL)

	// ── Redis Cache ────────────────────────────────────────
	redisURL := cfg.GetDefault(config.RedisURL, "localhost:6379")
	redisPassword := cfg.GetDefault(config.RedisPassword, "")
	redisClient, err := cache.New(ctx, redisURL, redisPassword)
	if err != nil {
		log.Warn("redis not available, running without cache", "error", err)
		redisClient = nil
	} else {
		defer redisClient.Close()
		log.Info("connected to redis", "url", redisURL)
	}

	// ── Correlation Engine ─────────────────────────────────
	eng := engine.New(natsQueue, graphWriter, log, m, engine.DefaultConfig())
	if err := eng.Start(ctx); err != nil {
		return fmt.Errorf("starting correlation engine: %w", err)
	}
	log.Info("correlation engine started")

	// ── HTTP API ───────────────────────────────────────────
	corrAPI := api.NewServer(graphReader, redisClient, log)
	port := cfg.GetDefault(config.ServicePort, "8080")

	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := "ok"
		neo4jStatus := "connected"
		if err := graphClient.Health(ctx); err != nil {
			neo4jStatus = "disconnected"
			status = "degraded"
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status":  status,
			"service": "correlation",
			"neo4j":   neo4jStatus,
		})
	})
	mux.Handle("/api/v1/", corrAPI.Handler())

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		log.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
		eng.Stop()
	}()

	log.Info("listening", "port", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}


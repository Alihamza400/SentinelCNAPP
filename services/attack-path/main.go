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
	"github.com/sentinel-cnapp/sentinel-cnapp/services/attack-path/internal/api"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/attack-path/internal/engine"
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
		cfg.GetDefault(config.ServiceName, "sentinel-attack-path"),
		logging.LevelInfo,
		os.Stdout,
	)
	m := metrics.New("attack_path")

	log.Info("starting attack path engine", "version", cfg.GetDefault(config.ServiceVersion, "0.1.0"))

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

	// ── Attack Path Engine ─────────────────────────────────
	eng := engine.New(graphClient, log)

	// Run initial scan on startup
	go func() {
		log.Info("running initial attack path analysis")
		if paths, err := eng.FindAll(ctx, 50); err != nil {
			log.Warn("initial analysis failed", "error", err)
		} else {
			log.Info("initial analysis complete", "paths", len(paths))
		}
	}()

	// Run scan every 30 minutes
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Info("running scheduled attack path analysis")
				if paths, err := eng.FindAll(ctx, 100); err != nil {
					log.Warn("scheduled analysis failed", "error", err)
				} else {
					log.Info("scheduled analysis complete", "paths", len(paths))
				}
			}
		}
	}()

	// ── HTTP API ─────────────────────────────────────────────
	attackAPI := api.NewServer(eng, log)
	port := cfg.GetDefault(config.ServicePort, "8080")

	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := "ok"
		if err := graphClient.Health(ctx); err != nil {
			status = "degraded"
		}
		json.NewEncoder(w).Encode(map[string]string{"status": status, "service": "attack-path"})
	})
	mux.Handle("/api/v1/attack-paths", attackAPI.Handler())
	mux.Handle("/api/v1/attack-paths/", attackAPI.Handler())

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	log.Info("listening", "port", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

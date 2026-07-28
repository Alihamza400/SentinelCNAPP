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
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/graph"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/metrics"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/ai-assistant/internal/api"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/ai-assistant/internal/engine"
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
		cfg.GetDefault(config.ServiceName, "sentinel-ai-assistant"),
		logging.LevelInfo,
		os.Stdout,
	)
	m := metrics.New("ai_assistant")

	log.Info("starting AI security assistant", "version", cfg.GetDefault(config.ServiceVersion, "0.1.0"))

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

	// ── LLM Config (optional) ──────────────────────────────
	llmAPIKey := cfg.GetDefault(config.LLMApiKey, "")
	llmEndpoint := cfg.GetDefault(config.LLMEndpoint, "")

	// ── AI Query Engine ────────────────────────────────────
	eng := engine.New(graphClient, log, llmAPIKey, llmEndpoint)

	// ── HTTP API ─────────────────────────────────────────────
	aiAPI := api.NewServer(eng, log)
	port := cfg.GetDefault(config.ServicePort, "8080")

	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		llmStatus := "disabled"
		if llmAPIKey != "" {
			llmStatus = "enabled"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "ai-assistant",
			"llm":     llmStatus,
		})
	})
	mux.Handle("/api/v1/ai", aiAPI.Handler())
	mux.Handle("/api/v1/ai/", aiAPI.Handler())

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	log.Info("listening for AI queries", "port", port, "llm_enabled", llmAPIKey != "")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

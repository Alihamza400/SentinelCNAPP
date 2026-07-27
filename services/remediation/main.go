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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/config"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/graph"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/metrics"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/queue"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/remediation/internal/actions"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/remediation/internal/api"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/remediation/internal/engine"
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
		cfg.GetDefault(config.ServiceName, "sentinel-remediation"),
		logging.LevelInfo,
		os.Stdout,
	)
	m := metrics.New("remediation")

	log.Info("starting remediation service", "version", cfg.GetDefault(config.ServiceVersion, "0.1.0"))

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

	// ── NATS ───────────────────────────────────────────────
	natsURL := cfg.GetDefault(config.NATSURL, "nats://localhost:4222")
	natsQueue, err := queue.NewNATS(natsURL)
	if err != nil {
		return fmt.Errorf("connecting to NATS: %w", err)
	}
	defer natsQueue.Close()

	// ── AWS Config ─────────────────────────────────────────
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Warn("aws config not available, running in dry-run mode", "error", err)
	}

	// ── Executors ──────────────────────────────────────────
	awsExecutor := actions.NewAWSExecutor(awsCfg)
	k8sExecutor := actions.NewK8sExecutor("kubectl")
	exec := actions.NewExecutor(awsExecutor, k8sExecutor)

	// ── Remediation Engine ─────────────────────────────────
	eng := engine.New(log, graphClient, natsQueue, exec)

	// Auto-execute low-severity remediations on startup
	go func() {
		time.Sleep(5 * time.Second)
		count, err := eng.ExecuteAllAutoRemediations(ctx)
		if err != nil {
			log.Warn("auto-execution failed", "error", err)
		} else {
			log.Info("auto-executed remediations", "count", count)
		}
	}()

	// ── HTTP API ─────────────────────────────────────────────
	remAPI := api.NewServer(eng, log)
	port := cfg.GetDefault(config.ServicePort, "8080")

	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "remediation"})
	})
	mux.Handle("/api/v1/remediation", remAPI.Handler())
	mux.Handle("/api/v1/remediation/", remAPI.Handler())

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	log.Info("listening for remediation requests", "port", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

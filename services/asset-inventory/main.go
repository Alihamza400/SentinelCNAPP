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

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/config"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/metrics"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/api"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/discovery"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/store"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/syncer"
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
		cfg.GetDefault(config.ServiceName, "sentinel-asset-inventory"),
		logging.LevelInfo,
		os.Stdout,
	)
	m := metrics.New("asset_inventory")

	log.Info("starting asset inventory service",
		"version", cfg.GetDefault(config.ServiceVersion, "0.1.0"))

	// ── PostgreSQL ──────────────────────────────────────────────
	dbURL := cfg.MustGet(config.PostgresURL)
	repo, err := store.NewRepository(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer repo.Close()

	// Run migrations
	if err := store.RunMigrations(ctx, repo.Pool()); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	log.Info("database migrations complete")

	// ── AWS SDK ─────────────────────────────────────────────────
	awsRegion := cfg.GetDefault(config.AWSRegion, "us-east-1")
	awsCfg, err := awscfg.LoadDefaultConfig(ctx,
		awscfg.WithRegion(awsRegion),
	)
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	accountID := cfg.GetDefault(config.AWSAccountID, "")
	if accountID == "" {
		// Try to discover account ID from STS
		stsClient := sts.NewFromConfig(awsCfg)
		caller, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return fmt.Errorf("getting AWS account ID: %w", err)
		}
		accountID = *caller.Account
	}
	log.Info("aws account", "id", accountID)

	// ── Discovery ───────────────────────────────────────────────
	regions := cfg.GetStringSlice(config.AWSRegions, []string{"us-east-1", "us-west-2", "eu-west-1"})

	discoverers := []discovery.Discoverer{
		discovery.NewEC2Discoverer(awsCfg, accountID),
		discovery.NewEKSDiscoverer(awsCfg, accountID),
		discovery.NewS3Discoverer(awsCfg, accountID),
		discovery.NewIAMDiscoverer(awsCfg, accountID),
		discovery.NewLambdaDiscoverer(awsCfg, accountID),
		discovery.NewECRDiscoverer(awsCfg, accountID),
		discovery.NewRDSDiscoverer(awsCfg, accountID),
	}

	orch := discovery.NewOrchestrator(discoverers, regions)

	// ── Syncer ──────────────────────────────────────────────────
	syncInterval := cfg.GetDuration(config.SyncInterval, 15*time.Minute)
	s := syncer.New(orch, repo, log, syncInterval)

	go s.RunForever(ctx)

	// ── HTTP API ───────────────────────────────────────────────
	assetAPI := api.NewHTTPServer(repo, log)
	port := cfg.GetDefault(config.ServicePort, "8080")

	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"asset-inventory","account":"` + accountID + `"}`))
	})
	mux.Handle("/api/v1/assets", assetAPI.Handler())
	mux.Handle("/api/v1/assets/", assetAPI.Handler())

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		log.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	log.Info("listening", "port", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

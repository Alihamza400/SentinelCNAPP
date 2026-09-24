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
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/finding"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/metrics"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/queue"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/scanner"
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
		cfg.GetDefault(config.ServiceName, "sentinel-scanner-secrets"),
		logging.LevelInfo,
		os.Stdout,
	)
	m := metrics.New("scanner_secrets")

	log.Info("starting secrets scanner service", "version", cfg.GetDefault(config.ServiceVersion, "0.1.0"))

	// Check dependencies
	if err := scanner.CheckDependencies(log, "gitleaks"); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	// Connect to NATS
	natsURL := cfg.GetDefault(config.NATSURL, "nats://localhost:4222")
	natsQueue, err := queue.NewNATS(natsURL)
	if err != nil {
		return fmt.Errorf("connecting to NATS: %w", err)
	}
	defer natsQueue.Close()

	emitter := scanner.NewEmitter(natsQueue, log, m, finding.SourceGitleaks)
	executor := scanner.NewExecutor(log, scanner.WithTimeout(30*time.Minute))

	// Webhook receiver
	webhookSecret := cfg.GetDefault(config.WebhookSecret, "")
	webhook := scanner.NewWebhookReceiver(log, webhookSecret)
	webhook.RegisterHandler("push", func(ctx context.Context, event *scanner.WebhookEvent) error {
		return runGitleaksScan(ctx, executor, emitter, log, event.RepoURL, event.CommitSHA)
	})

	// ── HTTP Server ──────────────────────────────
	port := cfg.GetDefault(config.ServicePort, "8080")
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"scanner-secrets"}`))
	})
	mux.HandleFunc("/webhook", webhook.Handler())

	// Manual scan endpoint
	mux.HandleFunc("/api/v1/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			RepoURL string `json:"repo_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		go func() {
			scanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if err := runGitleaksScan(scanCtx, executor, emitter, log, req.RepoURL, ""); err != nil {
				log.Error("manual secret scan failed", err, "repo", req.RepoURL)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "scan_started"})
	})

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

// GitleaksFinding represents a single secret finding from Gitleaks.
type GitleaksFinding struct {
	RuleID       string `json:"RuleID"`
	Description  string `json:"Description"`
	StartLine    int    `json:"StartLine"`
	EndLine      int    `json:"EndLine"`
	File         string `json:"File"`
	Match        string `json:"Match"`
	Secret       string `json:"Secret"`
	Commit       string `json:"Commit"`
	Author       string `json:"Author"`
	Email        string `json:"Email"`
	Date         string `json:"Date"`
	Message      string `json:"Message"`
	Fingerprint  string `json:"Fingerprint"`
	Repo         string `json:"Repo"`
}

// GitleaksOutput represents the full Gitleaks JSON output.
type GitleaksOutput struct {
	Results []GitleaksFinding `json:"results"`
	Summary struct {
		Total         int `json:"total"`
		LeakCount     int `json:"leak_count"`
		ScanTime      int `json:"scan_time_ms"`
	}
}

func runGitleaksScan(ctx context.Context, exec *scanner.Executor, emitter *scanner.Emitter, log *logging.Logger, repoURL, commitSHA string) error {
	start := time.Now()
	log.Info("starting Gitleaks scan", "repo", repoURL)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gitleaks-scan-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Clone repo
	cloneResult, err := exec.Run(ctx, "git", "clone", "--depth", "1", repoURL, tmpDir)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	log.Debug("clone complete", "output", truncate(cloneResult.Stderr, 200))

	// Run Gitleaks
	args := []string{"detect", "--source", tmpDir, "--format", "json", "--no-git"}
	if commitSHA != "" {
		args = []string{"detect", "--source", tmpDir, "--format", "json", "--commit-from", commitSHA}
	}

	// Run gitleaks with --verbose to capture JSON output to stdout
	result, err := exec.Run(ctx, "gitleaks", args...)
	if err != nil {
		// Gitleaks returns exit code 1 when secrets are found
		log.Debug("gitleaks completed", "exit_code", result.ExitCode)
	}

	// Gitleaks outputs JSON to stdout even on "failure"
	var output GitleaksOutput
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		// Try parsing as array of findings (alternate Gitleaks output format)
		var findings []GitleaksFinding
		if err2 := json.Unmarshal([]byte(result.Stdout), &findings); err2 != nil {
			return fmt.Errorf("parsing Gitleaks output (stdout: %s): %w", truncate(result.Stdout, 500), err)
		}
		output.Results = findings
	}

	// Convert to normalized findings
	var normalized []finding.Finding
	assetID := fmt.Sprintf("git:%s", repoURL)

	for _, secret := range output.Results {
		f := gitleaksToFinding(secret, assetID, repoURL)
		normalized = append(normalized, f)
	}

	// Emit findings
	emitted, err := emitter.EmitBatch(ctx, normalized)
	if err != nil {
		log.Warn("partial emit", "emitted", emitted, "total", len(normalized), "error", err)
	}

	duration := time.Since(start)
	log.Info("Gitleaks scan complete",
		"repo", repoURL,
		"secrets", len(normalized),
		"emitted", emitted,
		"duration", duration)

	return nil
}

func gitleaksToFinding(secret GitleaksFinding, assetID, repoURL string) finding.Finding {
	severity := finding.SeverityCritical // Secrets are always critical

	title := fmt.Sprintf("Secret found: %s", secret.RuleID)
	if secret.Description != "" {
		title = secret.Description
	}

	metadata := map[string]string{
		"rule_id":     secret.RuleID,
		"file":        secret.File,
		"line":        fmt.Sprintf("%d", secret.StartLine),
		"commit":      secret.Commit,
		"author":      secret.Author,
		"fingerprint": secret.Fingerprint,
		"repo_url":    repoURL,
	}

	return finding.Finding{
		ID:          finding.NewFindingID(finding.SourceGitleaks, assetID, secret.Fingerprint),
		Source:      finding.SourceGitleaks,
		Type:        finding.TypeSecret,
		Severity:    severity,
		AssetID:     assetID,
		Title:       title,
		Description: fmt.Sprintf("Secret of type %s detected in %s at line %d", secret.RuleID, secret.File, secret.StartLine),
		Remediation: "Remove the secret from the codebase, rotate the credential, and consider using a secrets manager (Vault, AWS Secrets Manager).",
		Metadata:    metadata,
		DetectedAt:  time.Now().UTC(),
		Status:      finding.StatusOpen,
		Tags:        []string{"secret", "git", secret.RuleID},
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

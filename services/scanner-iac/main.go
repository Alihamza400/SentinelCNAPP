package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
		cfg.GetDefault(config.ServiceName, "sentinel-scanner-iac"),
		logging.LevelInfo,
		os.Stdout,
	)
	m := metrics.New("scanner_iac")

	log.Info("starting IaC scanner service", "version", cfg.GetDefault(config.ServiceVersion, "0.1.0"))

	// Check dependencies
	if err := scanner.CheckDependencies(log, "checkov"); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	// Connect to NATS
	natsURL := cfg.GetDefault(config.NATSURL, "nats://localhost:4222")
	natsQueue, err := queue.NewNATS(natsURL)
	if err != nil {
		return fmt.Errorf("connecting to NATS: %w", err)
	}
	defer natsQueue.Close()

	emitter := scanner.NewEmitter(natsQueue, log, m, finding.SourceCheckov)
	executor := scanner.NewExecutor(log, scanner.WithTimeout(20*time.Minute))

	// Webhook receiver for GitHub/GitLab push events
	webhookSecret := cfg.GetDefault(config.WebhookSecret, "")
	webhook := scanner.NewWebhookReceiver(log, webhookSecret)
	webhook.RegisterHandler("push", func(ctx context.Context, event *scanner.WebhookEvent) error {
		return runCheckovScan(ctx, executor, emitter, log, event.RepoURL, event.Branch)
	})

	// ── HTTP Server ──────────────────────────────
	port := cfg.GetDefault(config.ServicePort, "8080")
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"scanner-iac"}`))
	})
	mux.HandleFunc("/webhook", webhook.Handler())

	// Manual scan endpoint
	mux.HandleFunc("/api/v1/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			RepoURL  string `json:"repo_url"`
			Branch   string `json:"branch"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		go func() {
			scanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if err := runCheckovScan(scanCtx, executor, emitter, log, req.RepoURL, req.Branch); err != nil {
				log.Error("manual scan failed", err, "repo", req.RepoURL)
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

// CheckovFinding represents a single finding in Checkov JSON output.
type CheckovFinding struct {
	CheckID        string            `json:"check_id"`
	CheckName      string            `json:"check_name"`
	File           string            `json:"file"`
	Resource       string            `json:"resource"`
	Severity       string            `json:"severity"`
	Description    string            `json:"description"`
	Guideline      string            `json:"guideline"`
	Status         string            `json:"status"` // "FAILED", "PASSED"
	RepoID         string            `json:"repo_id"`
	ResourceTags   map[string]string `json:"resource_tags"`
}

// CheckovOutput represents the full Checkov JSON output.
type CheckovOutput struct {
	Results struct {
		PassedChecks  []CheckovFinding `json:"passed_checks"`
		FailedChecks  []CheckovFinding `json:"failed_checks"`
		ParsingErrors []struct {
			File    string `json:"file"`
			Message string `json:"message"`
		} `json:"parsing_errors"`
	} `json:"results"`
}

func runCheckovScan(ctx context.Context, exec *scanner.Executor, emitter *scanner.Emitter, log *logging.Logger, repoURL, branch string) error {
	start := time.Now()
	log.Info("starting Checkov scan", "repo", repoURL, "branch", branch)

	// Create temp directory for repo clone
	tmpDir, err := os.MkdirTemp("", "checkov-scan-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Git clone
	cloneResult, err := exec.Run(ctx, "git", "clone", "--depth", "1", "--branch", branch, repoURL, tmpDir)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	log.Debug("clone complete", "output", truncate(cloneResult.Stderr, 200))

	// Run Checkov
	result, err := exec.Run(ctx, "checkov",
		"-d", tmpDir,
		"--framework", "terraform,cloudformation,kubernetes",
		"--output", "json",
		"--quiet",
		"--compact",
	)
	if err != nil {
		// Checkov exits 1 when findings are found — that's expected
		log.Debug("checkov completed", "exit_code", result.ExitCode, "stderr", truncate(result.Stderr, 200))
	}

	var output CheckovOutput
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		return fmt.Errorf("parsing Checkov output: %w", err)
	}

	// Convert failed checks to normalized findings
	var findings []finding.Finding
	for _, check := range output.Results.FailedChecks {
		f := checkovToFinding(check, repoURL, branch)
		findings = append(findings, f)
	}

	// Emit findings
	emitted, err := emitter.EmitBatch(ctx, findings)
	if err != nil {
		log.Warn("partial emit", "emitted", emitted, "total", len(findings), "error", err)
	}

	duration := time.Since(start)
	log.Info("Checkov scan complete",
		"repo", repoURL,
		"findings", len(findings),
		"emitted", emitted,
		"duration", duration)

	return nil
}

func checkovToFinding(check CheckovFinding, repoURL, branch string) finding.Finding {
	severity := finding.NormalizeSeverity(check.Severity)
	assetID := fmt.Sprintf("github:%s", check.RepoID)

	// Build resource path from file and resource
	resourcePath := check.Resource
	if resourcePath == "" {
		resourcePath = check.File
	}

	title := fmt.Sprintf("%s - %s", check.CheckID, check.CheckName)

	metadata := map[string]string{
		"check_id":   check.CheckID,
		"file":       check.File,
		"resource":   check.Resource,
		"guideline":  check.Guideline,
		"repo_url":   repoURL,
		"branch":     branch,
		"status":     check.Status,
	}

	return finding.Finding{
		ID:          finding.NewFindingID(finding.SourceCheckov, assetID, title),
		Source:      finding.SourceCheckov,
		Type:        finding.TypeMisconfiguration,
		Severity:    severity,
		AssetID:     assetID,
		Title:       title,
		Description: check.Description,
		Remediation: check.Guideline,
		Metadata:    metadata,
		DetectedAt:  time.Now().UTC(),
		Status:      finding.StatusOpen,
		Tags:        []string{"iac", check.CheckID},
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

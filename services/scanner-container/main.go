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
		cfg.GetDefault(config.ServiceName, "sentinel-scanner-container"),
		logging.LevelInfo,
		os.Stdout,
	)
	m := metrics.New("scanner_container")

	log.Info("starting container scanner service", "version", cfg.GetDefault(config.ServiceVersion, "0.1.0"))

	// Check dependencies
	if err := scanner.CheckDependencies(log, "trivy"); err != nil {
		return fmt.Errorf("dependency check failed: %w", err)
	}

	// Connect to NATS
	natsURL := cfg.GetDefault(config.NATSURL, "nats://localhost:4222")
	natsQueue, err := queue.NewNATS(natsURL)
	if err != nil {
		return fmt.Errorf("connecting to NATS: %w", err)
	}
	defer natsQueue.Close()

	emitter := scanner.NewEmitter(natsQueue, log, m, finding.SourceTrivy)
	executor := scanner.NewExecutor(log, scanner.WithTimeout(30*time.Minute))

	// ── HTTP Server ──────────────────────────────
	port := cfg.GetDefault(config.ServicePort, "8080")
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"scanner-container"}`))
	})

	// Manual scan endpoint
	mux.HandleFunc("/api/v1/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Image   string `json:"image"`
			Tag     string `json:"tag"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		imageRef := req.Image
		if req.Tag != "" {
			imageRef = fmt.Sprintf("%s:%s", req.Image, req.Tag)
		}

		go func() {
			scanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if err := runTrivyScan(scanCtx, executor, emitter, log, imageRef); err != nil {
				log.Error("container scan failed", err, "image", imageRef)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "scan_started", "image": imageRef})
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

// TrivyVulnerability represents a vulnerability finding in Trivy JSON output.
type TrivyVulnerability struct {
	VulnerabilityID  string            `json:"VulnerabilityID"`
	PkgName          string            `json:"PkgName"`
	InstalledVersion string            `json:"InstalledVersion"`
	FixedVersion     string            `json:"FixedVersion"`
	Severity         string            `json:"Severity"`
	Title            string            `json:"Title"`
	Description      string            `json:"Description"`
	CVSSScore        float64           `json:"CVSSScore,omitempty"`
	PrimaryURL       string            `json:"PrimaryURL"`
	References       []string          `json:"References"`
	CweIDs           []string          `json:"CweIDs"`
}

// TrivyResult represents a single result from Trivy.
type TrivyResult struct {
	Target          string               `json:"Target"`
	Type            string               `json:"Type"` // "container", "library", "filesystem"
	Vulnerabilities []TrivyVulnerability `json:"Vulnerabilities"`
	Misconfigurations []struct {
		ID          string `json:"ID"`
		Title       string `json:"Title"`
		Severity    string `json:"Severity"`
		Description string `json:"Description"`
		Message     string `json:"Message"`
	} `json:"Misconfigurations"`
	Secrets []struct {
		RuleID    string `json:"RuleID"`
		Category  string `json:"Category"`
		Severity  string `json:"Severity"`
		Title     string `json:"Title"`
		Match     string `json:"Match"`
	} `json:"Secrets"`
}

// TrivyOutput represents the full Trivy JSON output.
type TrivyOutput struct {
	ArtifactName string         `json:"ArtifactName"`
	ArtifactType string         `json:"ArtifactType"`
	Results      []TrivyResult  `json:"Results"`
}

func runTrivyScan(ctx context.Context, exec *scanner.Executor, emitter *scanner.Emitter, log *logging.Logger, imageRef string) error {
	start := time.Now()
	log.Info("starting Trivy scan", "image", imageRef)

	// Run Trivy with full output
	result, err := exec.Run(ctx, "trivy", "image",
		"--format", "json",
		"--quiet",
		"--severity", "CRITICAL,HIGH,MEDIUM,LOW",
		"--ignore-unfixed",
		"--no-progress",
		imageRef,
	)
	if err != nil {
		log.Debug("trivy completed", "exit_code", result.ExitCode, "stderr", truncate(result.Stderr, 200))
	}

	var output TrivyOutput
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		return fmt.Errorf("parsing Trivy output: %w", err)
	}

	// Convert vulnerabilities to normalized findings
	var findings []finding.Finding
	assetID := fmt.Sprintf("image:%s", output.ArtifactName)

	for _, res := range output.Results {
		for _, vuln := range res.Vulnerabilities {
			f := trivyVulnToFinding(vuln, assetID, imageRef, res.Target)
			findings = append(findings, f)
		}

		for _, mis := range res.Misconfigurations {
			f := trivyMisconfigToFinding(mis, assetID, imageRef)
			findings = append(findings, f)
		}
	}

	// Emit findings
	emitted, err := emitter.EmitBatch(ctx, findings)
	if err != nil {
		log.Warn("partial emit", "emitted", emitted, "total", len(findings), "error", err)
	}

	duration := time.Since(start)
	log.Info("Trivy scan complete",
		"image", imageRef,
		"vulnerabilities", len(findings),
		"emitted", emitted,
		"duration", duration)

	return nil
}

func trivyVulnToFinding(vuln TrivyVulnerability, assetID, imageRef, target string) finding.Finding {
	severity := finding.NormalizeSeverity(vuln.Severity)

	title := vuln.Title
	if title == "" {
		title = vuln.VulnerabilityID
	}

	metadata := map[string]string{
		"vulnerability_id":  vuln.VulnerabilityID,
		"package":           vuln.PkgName,
		"installed_version": vuln.InstalledVersion,
		"fixed_version":     vuln.FixedVersion,
		"target":            target,
		"image":             imageRef,
		"primary_url":       vuln.PrimaryURL,
	}

	return finding.Finding{
		ID:          finding.NewFindingID(finding.SourceTrivy, assetID, vuln.VulnerabilityID),
		Source:      finding.SourceTrivy,
		Type:        finding.TypeVulnerability,
		Severity:    severity,
		CVSSScore:   vuln.CVSSScore,
		AssetID:     assetID,
		Title:       title,
		Description: vuln.Description,
		Metadata:    metadata,
		DetectedAt:  time.Now().UTC(),
		Status:      finding.StatusOpen,
		References:  vuln.References,
		Tags:        []string{"container", "vulnerability", vuln.VulnerabilityID},
	}
}

func trivyMisconfigToFinding(mis struct {
	ID          string `json:"ID"`
	Title       string `json:"Title"`
	Severity    string `json:"Severity"`
	Description string `json:"Description"`
	Message     string `json:"Message"`
}, assetID, imageRef string) finding.Finding {
	severity := finding.NormalizeSeverity(mis.Severity)

	metadata := map[string]string{
		"misconfig_id": mis.ID,
		"image":        imageRef,
	}

	return finding.Finding{
		ID:          finding.NewFindingID(finding.SourceTrivy, assetID, mis.ID),
		Source:      finding.SourceTrivy,
		Type:        finding.TypeMisconfiguration,
		Severity:    severity,
		AssetID:     assetID,
		Title:       fmt.Sprintf("%s - %s", mis.ID, mis.Title),
		Description: mis.Description,
		Metadata:    metadata,
		DetectedAt:  time.Now().UTC(),
		Status:      finding.StatusOpen,
		Tags:        []string{"container", "misconfiguration", mis.ID},
	}
}

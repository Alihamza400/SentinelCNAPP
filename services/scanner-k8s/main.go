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
		cfg.GetDefault(config.ServiceName, "sentinel-scanner-k8s"),
		logging.LevelInfo,
		os.Stdout,
	)
	m := metrics.New("scanner_k8s")

	log.Info("starting K8s scanner service", "version", cfg.GetDefault(config.ServiceVersion, "0.1.0"))

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
		w.Write([]byte(`{"status":"ok","service":"scanner-k8s"}`))
	})

	// Manual scan endpoint
	mux.HandleFunc("/api/v1/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Namespace   string `json:"namespace"`
			Context     string `json:"context"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		go func() {
			scanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if err := runK8sScan(scanCtx, executor, emitter, log, req.Namespace, req.Context); err != nil {
				log.Error("K8s scan failed", err, "namespace", req.Namespace)
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

// K8sScanResult represents a custom K8s security check result.
type K8sScanResult struct {
	CheckID     string `json:"check_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Kind        string `json:"kind"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Message     string `json:"message"`
	Remediation string `json:"remediation"`
}

func runK8sScan(ctx context.Context, exec *scanner.Executor, emitter *scanner.Emitter, log *logging.Logger, namespace, kubeContext string) error {
	start := time.Now()
	log.Info("starting K8s scan", "namespace", namespace, "context", kubeContext)

	var findings []finding.Finding

	// Run Trivy K8s scan
	findings, err := runTrivyK8s(ctx, exec, log, namespace, kubeContext)
	if err != nil {
		log.Warn("trivy K8s scan had issues", "error", err)
	}

	// Run custom K8s security checks
	customFindings := runCustomK8sChecks(ctx, exec, log, namespace, kubeContext)
	findings = append(findings, customFindings...)

	// Emit findings
	emitted, err := emitter.EmitBatch(ctx, findings)
	if err != nil {
		log.Warn("partial emit", "emitted", emitted, "total", len(findings), "error", err)
	}

	duration := time.Since(start)
	log.Info("K8s scan complete",
		"namespace", namespace,
		"findings", len(findings),
		"emitted", emitted,
		"duration", duration)

	return nil
}

func runTrivyK8s(ctx context.Context, exec *scanner.Executor, log *logging.Logger, namespace, kubeContext string) ([]finding.Finding, error) {
	args := []string{"kubernetes", "--format", "json", "--quiet", "--no-progress"}
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}

	result, err := exec.Run(ctx, "trivy", args...)
	if err != nil {
		return nil, fmt.Errorf("trivy k8s: %w", err)
	}

	var output TrivyOutput
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		return nil, fmt.Errorf("parsing trivy k8s output: %w", err)
	}

	var findings []finding.Finding
	for _, res := range output.Results {
		assetID := fmt.Sprintf("k8s:%s", res.Target)

		for _, vuln := range res.Vulnerabilities {
			finding := finding.Finding{
				ID:          finding.NewFindingID(finding.SourceTrivy, assetID, vuln.VulnerabilityID),
				Source:      finding.SourceTrivy,
				Type:        finding.TypeVulnerability,
				Severity:    finding.NormalizeSeverity(vuln.Severity),
				AssetID:     assetID,
				Title:       vuln.Title,
				Description: vuln.Description,
				Metadata: map[string]string{
					"target": res.Target,
					"vuln_id": vuln.VulnerabilityID,
					"package": vuln.PkgName,
				},
				DetectedAt: time.Now().UTC(),
				Status:     finding.StatusOpen,
				Tags:       []string{"kubernetes", "vulnerability"},
			}
			findings = append(findings, finding)
		}
	}

	return findings, nil
}

func runCustomK8sChecks(ctx context.Context, exec *scanner.Executor, log *logging.Logger, namespace, kubeContext string) []finding.Finding {
	var findings []finding.Finding

	kubectlArgs := []string{}
	if namespace != "" {
		kubectlArgs = append(kubectlArgs, "-n", namespace)
	}

	// Check 1: Pods running as root
	rootPodsResult, err := exec.Run(ctx, "kubectl", append(kubectlArgs,
		"get", "pods",
		"--field-selector=status.phase=Running",
		"-o=jsonpath={range .items[*]}{.metadata.name}{\" \"}{.spec.containers[*].securityContext.runAsNonRoot}{\"\\n\"}{end}",
	)...)
	if err == nil {
		// Parse and flag non-root violations
		lines := strings.Split(strings.TrimSpace(rootPodsResult.Stdout), "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] == "false" {
				findings = append(findings, finding.Finding{
					ID:          finding.NewFindingID("k8s-custom", "k8s:"+namespace, "pod-run-as-root-"+parts[0]),
					Source:      "k8s-custom",
					Type:        finding.TypeMisconfiguration,
					Severity:    finding.SeverityHigh,
					AssetID:     fmt.Sprintf("k8s:%s/pod/%s", namespace, parts[0]),
					Title:       "Pod running as root",
					Description: fmt.Sprintf("Pod %s/%s is running as root without runAsNonRoot set", namespace, parts[0]),
					Remediation: "Set securityContext.runAsNonRoot: true and runAsUser: 1000+ in the pod spec",
					DetectedAt:  time.Now().UTC(),
					Status:      finding.StatusOpen,
					Tags:        []string{"kubernetes", "pod-security", "root"},
				})
			}
		}
	}

	// Check 2: Pods with privileged containers
	privResult, err := exec.Run(ctx, "kubectl", append(kubectlArgs,
		"get", "pods",
		"-o=jsonpath={range .items[?(@.spec.containers[*].securityContext.privileged==true)]}{.metadata.name}{\" \"}{.spec.containers[*].name}{\"\\n\"}{end}",
	)...)
	if err == nil && strings.TrimSpace(privResult.Stdout) != "" {
		lines := strings.Split(strings.TrimSpace(privResult.Stdout), "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				findings = append(findings, finding.Finding{
					ID:          finding.NewFindingID("k8s-custom", "k8s:"+namespace, "privileged-container-"+parts[0]),
					Source:      "k8s-custom",
					Type:        finding.TypeMisconfiguration,
					Severity:    finding.SeverityCritical,
					AssetID:     fmt.Sprintf("k8s:%s/pod/%s", namespace, parts[0]),
					Title:       "Privileged container running",
					Description: fmt.Sprintf("Pod %s has privileged containers: %v", parts[0], parts[1:]),
					Remediation: "Remove securityContext.privileged: true. Use more granular capabilities instead.",
					DetectedAt:  time.Now().UTC(),
					Status:      finding.StatusOpen,
					Tags:        []string{"kubernetes", "pod-security", "privileged"},
				})
			}
		}
	}

	// Check 3: Services with exposed load balancers
	svcResult, err := exec.Run(ctx, "kubectl", append(kubectlArgs,
		"get", "svc",
		"-o=jsonpath={range .items[?(@.spec.type==\"LoadBalancer\")]}{.metadata.name}{\" \"}{.spec.ports[*].port}{\"\\n\"}{end}",
	)...)
	if err == nil && strings.TrimSpace(svcResult.Stdout) != "" {
		lines := strings.Split(strings.TrimSpace(svcResult.Stdout), "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				findings = append(findings, finding.Finding{
					ID:          finding.NewFindingID("k8s-custom", "k8s:"+namespace, "loadbalancer-svc-"+parts[0]),
					Source:      "k8s-custom",
					Type:        finding.TypeMisconfiguration,
					Severity:    finding.SeverityMedium,
					AssetID:     fmt.Sprintf("k8s:%s/service/%s", namespace, parts[0]),
					Title:       "Service exposed via LoadBalancer",
					Description: fmt.Sprintf("Service %s/%s is publicly exposed via a LoadBalancer", namespace, parts[0]),
					Remediation: "Verify that this service needs to be publicly accessible. Consider using an Ingress with auth instead.",
					DetectedAt:  time.Now().UTC(),
					Status:      finding.StatusOpen,
					Tags:        []string{"kubernetes", "networking", "exposure"},
				})
			}
		}
	}

	// Check 4: Namespace without resource quotas
	nqResult, err := exec.Run(ctx, "kubectl", append(kubectlArgs,
		"get", "resourcequotas",
		"-o=name",
	)...)
	if err == nil && strings.TrimSpace(nqResult.Stdout) == "" {
		ns := namespace
		if ns == "" {
			ns = "default"
		}
		assetID := fmt.Sprintf("k8s:%s", ns)
		findings = append(findings, finding.Finding{
			ID:          finding.NewFindingID("k8s-custom", assetID, "no-resource-quota"),
			Source:      "k8s-custom",
			Type:        finding.TypeMisconfiguration,
			Severity:    finding.SeverityLow,
			AssetID:     assetID,
			Title:       "Namespace without resource quota",
			Description: fmt.Sprintf("Namespace %s does not have resource quotas configured", ns),
			Remediation: "Create a ResourceQuota to prevent resource exhaustion by a single team or application.",
			DetectedAt:  time.Now().UTC(),
			Status:      finding.StatusOpen,
			Tags:        []string{"kubernetes", "governance", "resource-quota"},
		})
	}

	return findings
}

// TrivyVulnerability represents a vulnerability finding in Trivy JSON output.
type TrivyVulnerability struct {
	VulnerabilityID  string   `json:"VulnerabilityID"`
	PkgName          string   `json:"PkgName"`
	InstalledVersion string   `json:"InstalledVersion"`
	FixedVersion     string   `json:"FixedVersion"`
	Severity         string   `json:"Severity"`
	Title            string   `json:"Title"`
	Description      string   `json:"Description"`
}

// TrivyResult represents a single result from Trivy.
type TrivyResult struct {
	Target          string               `json:"Target"`
	Type            string               `json:"Type"`
	Vulnerabilities []TrivyVulnerability `json:"Vulnerabilities"`
}

// TrivyOutput represents the full Trivy JSON output.
type TrivyOutput struct {
	ArtifactName string        `json:"ArtifactName"`
	ArtifactType string        `json:"ArtifactType"`
	Results      []TrivyResult `json:"Results"`
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

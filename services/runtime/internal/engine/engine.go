package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/finding"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/queue"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/scanner"
)

// FalcoEvent represents a runtime event from Falco.
type FalcoEvent struct {
	UUID       string            `json:"uuid"`
	Output     string            `json:"output"`
	Priority   string            `json:"priority"`
	Rule       string            `json:"rule"`
	Time       string            `json:"time"`
	Source     string            `json:"source"` // "syscall", "k8s_audit"
	Tags       []string          `json:"tags"`
	Hostname   string            `json:"hostname"`
	OutputFields map[string]any `json:"output_fields"`
}

// FalcoOutput represents the full Falco JSON output stream.
type FalcoOutput struct {
	Events []FalcoEvent `json:"events"`
}

// Engine monitors runtime events from Falco and converts them to findings.
type Engine struct {
	log     *logging.Logger
	emitter *scanner.Emitter
	queue   queue.Publisher
}

// New creates a new runtime protection engine.
func New(log *logging.Logger, emitter *scanner.Emitter, q queue.Publisher) *Engine {
	return &Engine{
		log:     log,
		emitter: emitter,
		queue:   q,
	}
}

// ProcessEvent processes a single Falco event and emits a normalized finding.
func (e *Engine) ProcessEvent(ctx context.Context, event FalcoEvent) error {
	start := time.Now()

	f := e.falcoEventToFinding(event)

	if err := e.emitter.Emit(ctx, f); err != nil {
		return fmt.Errorf("emitting runtime finding: %w", err)
	}

	e.log.Info("runtime event processed",
		"rule", event.Rule,
		"priority", event.Priority,
		"finding_id", f.ID,
		"duration", time.Since(start))

	return nil
}

// ProcessEvents processes a batch of Falco events.
func (e *Engine) ProcessEvents(ctx context.Context, events []FalcoEvent) (int, error) {
	success := 0
	for _, ev := range events {
		if err := e.ProcessEvent(ctx, ev); err != nil {
			e.log.Warn("failed to process runtime event", "error", err, "rule", ev.Rule)
			continue
		}
		success++
	}
	return success, nil
}

func (e *Engine) falcoEventToFinding(event FalcoEvent) finding.Finding {
	// Map Falco priority to our severity
	severity := mapFalcoPriority(event.Priority)

	// Build asset ID from hostname and k8s context
	assetID := fmt.Sprintf("host:%s", event.Hostname)
	if podName, ok := event.OutputFields["k8s.pod.name"]; ok {
		if ns, ok := event.OutputFields["k8s.ns.name"]; ok {
			assetID = fmt.Sprintf("k8s:%s/pod/%s", ns, podName)
		} else {
			assetID = fmt.Sprintf("k8s:/pod/%s", podName)
		}
	}

	// Build metadata from output fields
	metadata := make(map[string]string)
	for k, v := range event.OutputFields {
		metadata[k] = fmt.Sprintf("%v", v)
	}
	metadata["rule"] = event.Rule
	metadata["hostname"] = event.Hostname
	metadata["source"] = event.Source
	metadata["falco_uuid"] = event.UUID

	// Parse time
	detectedAt := time.Now().UTC()
	if event.Time != "" {
		if t, err := time.Parse(time.RFC3339, event.Time); err == nil {
			detectedAt = t
		}
	}

	title := fmt.Sprintf("Runtime: %s", event.Rule)
	if len(title) > 200 {
		title = title[:200]
	}

	return finding.Finding{
		ID:      finding.NewFindingID(finding.SourceFalco, assetID, event.UUID),
		Source:  finding.SourceFalco,
		Type:    finding.TypeRuntimeAlert,
		Severity: severity,
		AssetID: assetID,
		Title:   title,
		Description: event.Output,
		Remediation: "Investigate the runtime event. Check container security context and network policies.",
		Metadata:    metadata,
		DetectedAt:  detectedAt,
		Status:      finding.StatusOpen,
		Tags:        append(event.Tags, "runtime", event.Source),
	}
}

// Priority mapping from Falco to SentinelCNAPP.
var priorityMap = map[string]finding.Severity{
	"emergency": finding.SeverityCritical,
	"alert":     finding.SeverityCritical,
	"critical":  finding.SeverityCritical,
	"error":     finding.SeverityHigh,
	"warning":   finding.SeverityMedium,
	"notice":    finding.SeverityLow,
	"informational": finding.SeverityInfo,
	"debug":     finding.SeverityInfo,
}

func mapFalcoPriority(priority string) finding.Severity {
	if sev, ok := priorityMap[priority]; ok {
		return sev
	}
	return finding.SeverityInfo
}

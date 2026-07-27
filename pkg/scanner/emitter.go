package scanner

import (
	"context"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/finding"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/metrics"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/queue"
)

// Emitter publishes normalized findings to the event bus.
type Emitter struct {
	queue   queue.Publisher
	log     *logging.Logger
	metrics *metrics.Metrics
	source  finding.Source
}

// NewEmitter creates a new finding emitter.
func NewEmitter(q queue.Publisher, log *logging.Logger, m *metrics.Metrics, source finding.Source) *Emitter {
	return &Emitter{
		queue:   q,
		log:     log,
		metrics: m,
		source:  source,
	}
}

// Emit publishes a single finding to NATS.
func (e *Emitter) Emit(ctx context.Context, f finding.Finding) error {
	f.Source = e.source

	if err := f.Validate(); err != nil {
		e.log.Warn("invalid finding, skipping", "error", err, "title", f.Title)
		return err
	}

	start := time.Now()
	if err := e.queue.PublishJSON(ctx, queue.TopicFindingIngested, f); err != nil {
		e.log.Error("failed to publish finding", err, "id", f.ID)
		return err
	}

	e.metrics.RecordFinding(e.source, f.Type, f.Severity)
	e.log.Debug("finding emitted",
		"id", f.ID,
		"type", f.Type,
		"severity", f.Severity,
		"asset", f.AssetID,
		"duration", time.Since(start))

	return nil
}

// EmitBatch publishes multiple findings to NATS.
func (e *Emitter) EmitBatch(ctx context.Context, findings []finding.Finding) (int, error) {
	success := 0
	for _, f := range findings {
		if err := e.Emit(ctx, f); err != nil {
			e.log.Warn("failed to emit finding in batch", "error", err, "title", f.Title)
			continue
		}
		success++
	}

	if success < len(findings) {
		return success, ErrPartialEmit
	}
	return success, nil
}

// ScanResult wraps findings and metadata from a scan.
type ScanResult struct {
	Source        finding.Source
	Target        string
	StartedAt     time.Time
	CompletedAt   time.Time
	Findings      []finding.Finding
	Error         error
}

// Scanner is the interface that all scanner modules implement.
type Scanner interface {
	// Scan executes a scan and returns findings.
	Scan(ctx context.Context, target string) (*ScanResult, error)

	// Source returns the scanner identifier.
	Source() finding.Source

	// Name returns a human-readable name.
	Name() string
}

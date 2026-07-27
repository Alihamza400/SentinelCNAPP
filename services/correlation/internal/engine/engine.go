package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/finding"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/metrics"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/queue"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/correlation/internal/graphdb"
)

// Config configures the correlation engine.
type Config struct {
	BatchSize     int
	FlushInterval time.Duration
	WorkerCount   int
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		BatchSize:     50,
		FlushInterval: 5 * time.Second,
		WorkerCount:   5,
	}
}

// Engine consumes findings from NATS and writes them to the Neo4j graph.
type Engine struct {
	queue      queue.Subscriber
	writer     *graphdb.Writer
	log        *logging.Logger
	metrics    *metrics.Metrics
	cfg        Config
	batch      chan finding.Finding
	done       chan struct{}
}

// New creates a new correlation engine.
func New(
	sub queue.Subscriber,
	writer *graphdb.Writer,
	log *logging.Logger,
	m *metrics.Metrics,
	cfg Config,
) *Engine {
	return &Engine{
		queue:   sub,
		writer:  writer,
		log:     log,
		metrics: m,
		cfg:     cfg,
		batch:   make(chan finding.Finding, cfg.BatchSize*10),
		done:    make(chan struct{}),
	}
}

// Start begins consuming findings and writing them to the graph.
func (e *Engine) Start(ctx context.Context) error {
	e.log.Info("starting correlation engine",
		"batch_size", e.cfg.BatchSize,
		"flush_interval", e.cfg.FlushInterval,
		"workers", e.cfg.WorkerCount)

	// Start the batch flusher
	go e.batchFlusher(ctx)

	// Subscribe to findings topic
	err := e.queue.Subscribe(ctx, queue.TopicFindingIngested, "correlation-service",
		func(ctx context.Context, msg *queue.Message) error {
			var f finding.Finding
			if err := json.Unmarshal(msg.Data, &f); err != nil {
				e.log.Warn("failed to unmarshal finding", "error", err)
				return nil // Don't retry unparseable messages
			}

			// Validate
			if err := f.Validate(); err != nil {
				e.log.Warn("invalid finding received", "error", err, "id", f.ID)
				return nil
			}

			// Send to batch channel
			select {
			case e.batch <- f:
			case <-ctx.Done():
				return ctx.Err()
			}

			e.metrics.MessagesConsumed.WithLabelValues(queue.TopicFindingIngested, "correlation-service").Inc()
			return nil
		})
	if err != nil {
		return fmt.Errorf("subscribing to findings: %w", err)
	}

	e.log.Info("correlation engine started")
	return nil
}

// batchFlusher periodically flushes buffered findings to Neo4j.
func (e *Engine) batchFlusher(ctx context.Context) {
	ticker := time.NewTicker(e.cfg.FlushInterval)
	defer ticker.Stop()

	var buffer []finding.Finding

	flush := func() {
		if len(buffer) == 0 {
			return
		}

		start := time.Now()
		batch := buffer
		buffer = nil

		count, err := e.writer.WriteFindingBatch(ctx, batch)
		if err != nil {
			e.log.Error("failed to flush batch", err, "count", len(batch))
			return
		}

		e.log.Debug("batch flushed to graph",
			"count", count,
			"duration", time.Since(start))
		e.metrics.GraphNodes.Add(float64(count))
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			close(e.done)
			return

		case f := <-e.batch:
			buffer = append(buffer, f)
			if len(buffer) >= e.cfg.BatchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// Stop gracefully shuts down the engine.
func (e *Engine) Stop() {
	<-e.done
	e.log.Info("correlation engine stopped")
}

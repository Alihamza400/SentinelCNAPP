package syncer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/discovery"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/store"
)

// Syncer orchestrates asset discovery and persistence.
type Syncer struct {
	orchestrator *discovery.Orchestrator
	repo         *store.Repository
	log          *logging.Logger
	interval     time.Duration
	mu           sync.Mutex
	running      bool
}

// New creates a new Syncer.
func New(
	orchestrator *discovery.Orchestrator,
	repo *store.Repository,
	log *logging.Logger,
	interval time.Duration,
) *Syncer {
	return &Syncer{
		orchestrator: orchestrator,
		repo:         repo,
		log:          log,
		interval:     interval,
	}
}

// RunOnce performs a single discovery and sync cycle.
func (s *Syncer) RunOnce(ctx context.Context) error {
	s.log.Info("starting discovery cycle")

	results := s.orchestrator.DiscoverAll(ctx)

	totalAssets := 0
	var errs []error

	for _, result := range results {
		if result.Error != nil {
			errs = append(errs, fmt.Errorf("%s/%s: %w", result.Service, result.Region, result.Error))
			s.log.Warn("discovery error",
				"service", result.Service,
				"region", result.Region,
				"error", result.Error,
				"duration", result.Duration)
			continue
		}

		s.log.Info("discovered assets",
			"service", result.Service,
			"region", result.Region,
			"count", len(result.Assets),
			"duration", result.Duration)

		if len(result.Assets) > 0 {
			if err := s.repo.BatchUpsertAssets(ctx, result.Assets); err != nil {
				errs = append(errs, fmt.Errorf("storing %s/%s: %w", result.Service, result.Region, err))
				continue
			}
			totalAssets += len(result.Assets)
		}
	}

	s.log.Info("discovery cycle complete",
		"total_assets", totalAssets,
		"errors", len(errs))

	if len(errs) > 0 {
		return fmt.Errorf("discovery completed with %d errors: %v", len(errs), errs[0])
	}
	return nil
}

// RunForever runs the sync on a timer until the context is cancelled.
func (s *Syncer) RunForever(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	// Run immediately on start
	if err := s.RunOnce(ctx); err != nil {
		s.log.Warn("initial sync failed", "error", err)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("sync stopped")
			return
		case <-ticker.C:
			if err := s.RunOnce(ctx); err != nil {
				s.log.Warn("sync cycle failed", "error", err)
			}
		}
	}
}

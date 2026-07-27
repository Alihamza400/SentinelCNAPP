package discovery

import (
	"context"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/store"
)

// Discoverer defines the interface for discovering assets from a cloud provider.
type Discoverer interface {
	// Discover discovers assets in the given region.
	Discover(ctx context.Context, region string) ([]store.Asset, error)

	// Provider returns the cloud provider name (e.g., "aws").
	Provider() string

	// Service returns the asset type this discoverer handles (e.g., "ec2").
	Service() string
}

// Result wraps a discovery result with metadata.
type Result struct {
	Assets    []store.Asset
	Region    string
	Service   string
	Duration  time.Duration
	Error     error
}

// Orchestrator manages multiple discoverers across regions.
type Orchestrator struct {
	discoverers []Discoverer
	regions     []string
	concurrency int
}

// OrchestratorOption configures the orchestrator.
type OrchestratorOption func(*Orchestrator)

// WithConcurrency sets the maximum number of parallel discovery operations.
func WithConcurrency(n int) OrchestratorOption {
	return func(o *Orchestrator) {
		if n > 0 {
			o.concurrency = n
		}
	}
}

// NewOrchestrator creates a new discovery orchestrator.
func NewOrchestrator(discoverers []Discoverer, regions []string, opts ...OrchestratorOption) *Orchestrator {
	o := &Orchestrator{
		discoverers: discoverers,
		regions:     regions,
		concurrency: 10,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// DiscoverAll runs all discoverers across all regions and returns results.
func (o *Orchestrator) DiscoverAll(ctx context.Context) []Result {
	type workItem struct {
		discoverer Discoverer
		region     string
	}

	items := make([]workItem, 0, len(o.discoverers)*len(o.regions))
	for _, d := range o.discoverers {
		for _, region := range o.regions {
			items = append(items, workItem{discoverer: d, region: region})
		}
	}

	sem := make(chan struct{}, o.concurrency)
	results := make([]Result, 0, len(items))
	resultCh := make(chan Result, len(items))

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	for _, item := range items {
		sem <- struct{}{}
		go func(item workItem) {
			defer func() { <-sem }()

			start := time.Now()
			assets, err := item.discoverer.Discover(ctx, item.region)
			resultCh <- Result{
				Assets:  assets,
				Region:  item.region,
				Service: item.discoverer.Service(),
				Duration: time.Since(start),
				Error:   err,
			}
		}(item)
	}

	// Wait for all goroutines to complete
	for i := 0; i < len(items); i++ {
		results = append(results, <-resultCh)
	}

	return results
}

// GetRegions returns the configured regions.
func (o *Orchestrator) GetRegions() []string {
	return o.regions
}

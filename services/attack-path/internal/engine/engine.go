package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/graph"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
)

// AttackPath represents a single attack path from a finding to an exposed resource.
type AttackPath struct {
	ID          string            `json:"id"`
	RiskScore   float64           `json:"risk_score"`
	Steps       []AttackStep      `json:"steps"`
	TotalSteps  int               `json:"total_steps"`
	MaxSeverity string            `json:"max_severity"`
	DiscoveredAt time.Time        `json:"discovered_at"`
}

// AttackStep represents one step in an attack path.
type AttackStep struct {
	NodeID    string `json:"node_id"`
	NodeType  string `json:"node_type"`
	Label     string `json:"label"`
	Name      string `json:"name"`
	Detail    string `json:"detail"`
	Severity  string `json:"severity,omitempty"`
}

// Query to find all attack paths from critical/high findings.
const queryFindAttackPaths = `
	MATCH path = (f:Finding)-[:FOUND_IN]->(a:Asset)
		-[:HAS_IDENTITY]->(i:Identity)-[:CAN_ACCESS]->(r:Asset)
	WHERE f.severity IN ['critical', 'high']
		AND f.status = 'open'
		AND (r.internet_facing = true OR r.name CONTAINS 'public' OR r.type CONTAINS 'bucket')
	RETURN
		f.id AS finding_id,
		f.severity AS finding_severity,
		f.title AS finding_title,
		f.risk_score AS risk_score,
		a.id AS asset_id,
		a.name AS asset_name,
		a.type AS asset_type,
		a.internet_facing AS asset_internet_facing,
		i.arn AS identity_arn,
		i.name AS identity_name,
		r.id AS resource_id,
		r.name AS resource_name,
		r.type AS resource_type,
		r.internet_facing AS resource_internet_facing
	ORDER BY f.risk_score DESC
	LIMIT $limit
`

// Query to find paths from a specific finding.
const queryFindPathsFromFinding = `
	MATCH path = (f:Finding {id: $finding_id})-[:FOUND_IN]->(a:Asset)
		-[:HAS_IDENTITY]->(i:Identity)-[:CAN_ACCESS]->(r:Asset)
	WHERE r.internet_facing = true
	RETURN
		a.id AS asset_id,
		a.name AS asset_name,
		a.type AS asset_type,
		i.arn AS identity_arn,
		i.name AS identity_name,
		r.id AS resource_id,
		r.name AS resource_name,
		r.type AS resource_type
`

// Engine finds attack paths in the Neo4j graph.
type Engine struct {
	client *graph.Client
	log    *logging.Logger
}

// New creates a new attack path engine.
func New(client *graph.Client, log *logging.Logger) *Engine {
	return &Engine{client: client, log: log}
}

// FindAll finds all attack paths across the graph.
func (e *Engine) FindAll(ctx context.Context, limit int) ([]*AttackPath, error) {
	start := time.Now()

	records, err := e.client.Read(ctx, queryFindAttackPaths, map[string]any{
		"limit": limit,
	})
	if err != nil {
		return nil, fmt.Errorf("finding attack paths: %w", err)
	}

	paths := e.buildPaths(records)

	e.log.Info("attack path analysis complete",
		"paths_found", len(paths),
		"duration", time.Since(start))

	return paths, nil
}

// FindFromFinding finds all attack paths starting from a specific finding.
func (e *Engine) FindFromFinding(ctx context.Context, findingID string) ([]*AttackPath, error) {
	records, err := e.client.Read(ctx, queryFindPathsFromFinding, map[string]any{
		"finding_id": findingID,
	})
	if err != nil {
		return nil, fmt.Errorf("finding paths from finding: %w", err)
	}

	paths := e.buildPaths(records)

	return paths, nil
}

// GetSummary returns aggregate attack path statistics.
type Summary struct {
	TotalPaths      int            `json:"total_paths"`
	UniqueFindings  int            `json:"unique_findings"`
	UniqueAssets    int            `json:"unique_assets"`
	ExposedResources int           `json:"exposed_resources"`
	BySeverity      map[string]int `json:"by_severity"`
	TopPaths        []*AttackPath  `json:"top_paths"`
}

func (e *Engine) GetSummary(ctx context.Context) (*Summary, error) {
	paths, err := e.FindAll(ctx, 100)
	if err != nil {
		return nil, err
	}

	summary := &Summary{
		TotalPaths:     len(paths),
		BySeverity:     make(map[string]int),
		TopPaths:       paths,
	}

	findingSet := make(map[string]bool)
	assetSet := make(map[string]bool)
	resourceSet := make(map[string]bool)

	for _, p := range paths {
		findingSet[p.Steps[0].NodeID] = true
		summary.BySeverity[p.MaxSeverity]++

		for _, step := range p.Steps {
			if step.NodeType == "Asset" {
				assetSet[step.NodeID] = true
			}
			if step.NodeType == "Resource" {
				resourceSet[step.NodeID] = true
			}
		}
	}

	summary.UniqueFindings = len(findingSet)
	summary.UniqueAssets = len(assetSet)
	summary.ExposedResources = len(resourceSet)

	return summary, nil
}

func (e *Engine) buildPaths(records []interface{ Get(key string) (any, bool) }) []*AttackPath {
	pathMap := make(map[string]*AttackPath)

	for _, rec := range records {
		findingID := getStr(rec, "finding_id")
		if findingID == "" {
			continue
		}

		path, exists := pathMap[findingID]
		if !exists {
			riskScore := getFloat(rec, "risk_score")
			severity := getStr(rec, "finding_severity")

			path = &AttackPath{
				ID:           fmt.Sprintf("path-%s", findingID),
				RiskScore:    riskScore,
				MaxSeverity:  severity,
				DiscoveredAt: time.Now().UTC(),
			}

			// Step 1: Finding
			path.Steps = append(path.Steps, AttackStep{
				NodeID:   findingID,
				NodeType: "Finding",
				Label:    "Security Finding",
				Name:     getStr(rec, "finding_title"),
				Detail:   fmt.Sprintf("Severity: %s, Risk: %.1f", severity, riskScore),
				Severity: severity,
			})

			// Step 2: Asset (where finding was found)
			path.Steps = append(path.Steps, AttackStep{
				NodeID:   getStr(rec, "asset_id"),
				NodeType: "Asset",
				Label:    getStr(rec, "asset_type"),
				Name:     getStr(rec, "asset_name"),
				Detail:   fmt.Sprintf("Internet facing: %v", getBool(rec, "asset_internet_facing")),
			})

			// Step 3: Identity
			path.Steps = append(path.Steps, AttackStep{
				NodeID:   getStr(rec, "identity_arn"),
				NodeType: "Identity",
				Label:    "IAM Role",
				Name:     getStr(rec, "identity_name"),
				Detail:   "Over-privileged identity",
			})

			pathMap[findingID] = path
		}

		// Step 4: Exposed Resource
		resourceStep := AttackStep{
			NodeID:   getStr(rec, "resource_id"),
			NodeType: "Resource",
			Label:    getStr(rec, "resource_type"),
			Name:     getStr(rec, "resource_name"),
			Detail:   fmt.Sprintf("Internet facing: %v", getBool(rec, "resource_internet_facing")),
		}

		// Only add resource if it's not already in the path
		exists = false
		for _, step := range path.Steps {
			if step.NodeID == resourceStep.NodeID {
				exists = true
				break
			}
		}
		if !exists {
			path.Steps = append(path.Steps, resourceStep)
		}

		path.TotalSteps = len(path.Steps)
	}

	// Convert map to slice
	var paths []*AttackPath
	for _, p := range pathMap {
		paths = append(paths, p)
	}

	return paths
}

func getStr(rec interface{ Get(key string) (any, bool) }, key string) string {
	if v, ok := rec.Get(key); ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func getFloat(rec interface{ Get(key string) (any, bool) }, key string) float64 {
	if v, ok := rec.Get(key); ok && v != nil {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0.0
}

func getBool(rec interface{ Get(key string) (any, bool) }, key string) bool {
	if v, ok := rec.Get(key); ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

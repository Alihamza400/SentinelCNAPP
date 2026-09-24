package engine

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/graph"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
)

// RiskFactor defines a single factor in the risk computation.
type RiskFactor struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
	Score  float64 `json:"score"` // 0.0 to 1.0
}

// RiskEvaluation holds the complete risk assessment for a finding.
type RiskEvaluation struct {
	FindingID      string       `json:"finding_id"`
	OverallScore   float64      `json:"overall_score"`   // 0.0 to 10.0
	Severity       string       `json:"severity"`
	Factors        []RiskFactor `json:"factors"`
	AttackPaths    int          `json:"attack_paths"`
	AffectedAssets int          `json:"affected_assets"`
	ComputedAt     time.Time    `json:"computed_at"`
}

// Cypher query to compute risk factors for a single finding.
const queryRiskFactors = `
	MATCH (f:Finding {id: $finding_id})-[:FOUND_IN]->(a:Asset)
	OPTIONAL MATCH (a)-[:HAS_IDENTITY]->(i:Identity)-[:CAN_ACCESS]->(r:Asset)
	OPTIONAL MATCH (a) WHERE a.internet_facing = true
	OPTIONAL MATCH (a) WHERE a.data_classification IN ['confidential', 'restricted']
	OPTIONAL MATCH (f) WHERE f.cvss_score >= 7.0
	OPTIONAL MATCH (f) WHERE f.severity IN ['critical', 'high']

	RETURN
		a.internet_facing AS internet_facing,
		a.data_classification AS data_classification,
		a.environment AS environment,
		f.cvss_score AS cvss_score,
		f.severity AS severity,
		count(DISTINCT r) AS exposed_resource_count,
		f.type AS finding_type
`

// Cypher query to count attack paths from a specific finding.
const queryAttackPathCount = `
	MATCH path = (f:Finding {id: $finding_id})-[:FOUND_IN]->(a:Asset)
		-[:HAS_IDENTITY]->(i:Identity)-[:CAN_ACCESS]->(r:Asset)
	WHERE r.internet_facing = true
	RETURN count(path) AS path_count
`

// Cypher query to write the risk score back to the finding node.
const queryWriteRiskScore = `
	MATCH (f:Finding {id: $finding_id})
	SET f.risk_score = $risk_score,
		f.risk_factors = $risk_factors,
		f.risk_computed_at = $computed_at
	RETURN f
`

// Engine computes context-aware risk scores for findings.
type Engine struct {
	client *graph.Client
	log    *logging.Logger
}

// New creates a new risk scoring engine.
func New(client *graph.Client, log *logging.Logger) *Engine {
	return &Engine{client: client, log: log}
}

// Evaluate computes the risk score for a single finding.
func (e *Engine) Evaluate(ctx context.Context, findingID string) (*RiskEvaluation, error) {
	start := time.Now()
	e.log.Debug("evaluating risk", "finding_id", findingID)

	// Get risk factors from graph
	record, err := e.client.ReadSingle(ctx, queryRiskFactors, map[string]any{
		"finding_id": findingID,
	})
	if err != nil {
		return nil, fmt.Errorf("querying risk factors: %w", err)
	}
	if record == nil {
		return nil, fmt.Errorf("finding %s not found in graph", findingID)
	}

	// Extract factors
	factors := e.extractFactors(record)
	overallScore := e.computeOverallScore(factors)

	// Count attack paths
	pathRecord, err := e.client.ReadSingle(ctx, queryAttackPathCount, map[string]any{
		"finding_id": findingID,
	})
	attackPathCount := 0
	if err == nil && pathRecord != nil {
		if v, ok := pathRecord.Get("path_count"); ok {
			attackPathCount = toInt(v)
		}
	}

	// Determine severity based on score
	severity := scoreToSeverity(overallScore)

	evaluation := &RiskEvaluation{
		FindingID:    findingID,
		OverallScore: math.Round(overallScore*10) / 10,
		Severity:     severity,
		Factors:      factors,
		AttackPaths:  attackPathCount,
		ComputedAt:   time.Now().UTC(),
	}

	// Write risk score back to Neo4j
	e.writeRiskToGraph(ctx, findingID, evaluation)

	e.log.Info("risk evaluated",
		"finding_id", findingID,
		"score", evaluation.OverallScore,
		"severity", evaluation.Severity,
		"attack_paths", attackPathCount,
		"duration", time.Since(start))

	return evaluation, nil
}

// EvaluateBatch computes risk scores for multiple findings in parallel.
func (e *Engine) EvaluateBatch(ctx context.Context, findingIDs []string) ([]*RiskEvaluation, error) {
	type result struct {
		eval *RiskEvaluation
		err  error
	}

	results := make(chan result, len(findingIDs))
	sem := make(chan struct{}, 10) // max 10 concurrent evaluations

	for _, id := range findingIDs {
		go func(fid string) {
			sem <- struct{}{}
			defer func() { <-sem }()

			eval, err := e.Evaluate(ctx, fid)
			results <- result{eval: eval, err: err}
		}(id)
	}

	var evals []*RiskEvaluation
	for i := 0; i < len(findingIDs); i++ {
		r := <-results
		if r.err != nil {
			e.log.Warn("failed to evaluate risk", "error", r.err)
			continue
		}
		evals = append(evals, r.eval)
	}

	return evals, nil
}

// EvaluateAll evaluates risk for all open findings.
func (e *Engine) EvaluateAll(ctx context.Context) ([]*RiskEvaluation, error) {
	// Get all open finding IDs
	records, err := e.client.Read(ctx, `
		MATCH (f:Finding)
		WHERE f.status = 'open'
		RETURN f.id AS id
		LIMIT 1000
	`, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("querying open findings: %w", err)
	}

	var findingIDs []string
	for _, rec := range records {
		if id, ok := rec.Get("id"); ok {
			findingIDs = append(findingIDs, id.(string))
		}
	}

	if len(findingIDs) == 0 {
		return []*RiskEvaluation{}, nil
	}

	return e.EvaluateBatch(ctx, findingIDs)
}

func (e *Engine) extractFactors(record interface{ Get(key string) (any, bool) }) []RiskFactor {
	factors := []RiskFactor{}

	// Factor 1: Internet-facing asset
	if v, ok := record.Get("internet_facing"); ok && v != nil {
		if b, ok := v.(bool); ok && b {
			factors = append(factors, RiskFactor{
				Name: "internet_facing", Weight: 0.25, Score: 1.0,
			})
		}
	}

	// Factor 2: Data classification
	if v, ok := record.Get("data_classification"); ok && v != nil {
		dc := fmt.Sprintf("%v", v)
		if dc == "confidential" || dc == "restricted" {
			factors = append(factors, RiskFactor{
				Name: "sensitive_data", Weight: 0.20, Score: 1.0,
			})
		}
	}

	// Factor 3: Environment
	if v, ok := record.Get("environment"); ok && v != nil {
		env := fmt.Sprintf("%v", v)
		if env == "production" {
			factors = append(factors, RiskFactor{
				Name: "production", Weight: 0.10, Score: 0.8,
			})
		} else if env == "staging" {
			factors = append(factors, RiskFactor{
				Name: "staging", Weight: 0.10, Score: 0.4,
			})
		}
	}

	// Factor 4: CVSS score
	if v, ok := record.Get("cvss_score"); ok && v != nil {
		if cvss, ok := v.(float64); ok && cvss > 0 {
			score := cvss / 10.0 // normalize to 0-1
			factors = append(factors, RiskFactor{
				Name: "cvss_score", Weight: 0.20, Score: score,
			})
		}
	}

	// Factor 5: Finding severity
	if v, ok := record.Get("severity"); ok && v != nil {
		sev := fmt.Sprintf("%v", v)
		switch sev {
		case "critical":
			factors = append(factors, RiskFactor{
				Name: "severity", Weight: 0.15, Score: 1.0,
			})
		case "high":
			factors = append(factors, RiskFactor{
				Name: "severity", Weight: 0.15, Score: 0.8,
			})
		case "medium":
			factors = append(factors, RiskFactor{
				Name: "severity", Weight: 0.15, Score: 0.5,
			})
		}
	}

	// Factor 6: Exposed resource count
	if v, ok := record.Get("exposed_resource_count"); ok && v != nil {
		count := toInt(v)
		if count > 0 {
			score := math.Min(float64(count)/10.0, 1.0)
			factors = append(factors, RiskFactor{
				Name: "exposed_resources", Weight: 0.10, Score: score,
			})
		}
	}

	return factors
}

func (e *Engine) computeOverallScore(factors []RiskFactor) float64 {
	if len(factors) == 0 {
		// Default baseline risk based on severity alone
		return 1.0
	}

	weightedSum := 0.0
	totalWeight := 0.0

	for _, f := range factors {
		weightedSum += f.Weight * f.Score
		totalWeight += f.Weight
	}

	if totalWeight == 0 {
		return 0.0
	}

	// Normalize to 0-10 scale
	normalized := (weightedSum / totalWeight) * 10.0
	return math.Min(normalized, 10.0)
}

func (e *Engine) writeRiskToGraph(ctx context.Context, findingID string, eval *RiskEvaluation) {
	factorJSON := ""
	for i, f := range eval.Factors {
		if i > 0 {
			factorJSON += ", "
		}
		factorJSON += fmt.Sprintf("%s:%.1f", f.Name, f.Score)
	}

	err := e.client.Write(ctx, queryWriteRiskScore, map[string]any{
		"finding_id":   findingID,
		"risk_score":   eval.OverallScore,
		"risk_factors": factorJSON,
		"computed_at":  eval.ComputedAt.Format(time.RFC3339),
	})
	if err != nil {
		e.log.Warn("failed to write risk score to graph", "error", err, "finding_id", findingID)
	}
}

func scoreToSeverity(score float64) string {
	switch {
	case score >= 8.0:
		return "critical"
	case score >= 6.0:
		return "high"
	case score >= 4.0:
		return "medium"
	case score >= 2.0:
		return "low"
	default:
		return "info"
	}
}

func toInt(v any) int {
	switch val := v.(type) {
	case int64:
		return int(val)
	case float64:
		return int(val)
	case int:
		return val
	default:
		return 0
	}
}

package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/graph"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
)

// QueryTemplate defines a pre-built natural language query with its Cypher translation.
type QueryTemplate struct {
	ID          string   `json:"id"`
	Question    string   `json:"question"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Cypher      string   `json:"cypher"`
	Params      []string `json:"params"`
}

// QueryResult holds the result of executing a query.
type QueryResult struct {
	Query     string         `json:"query"`
	Cypher    string         `json:"cypher"`
	Results   []map[string]any `json:"results"`
	Rows      int            `json:"rows"`
	Duration  time.Duration  `json:"duration"`
	Error     string         `json:"error,omitempty"`
}

// Engine translates natural language questions to Cypher queries.
type Engine struct {
	client    *graph.Client
	log       *logging.Logger
	templates []QueryTemplate
	llm       *LLMClient // optional
}

// New creates a new AI query engine with pre-built templates.
func New(client *graph.Client, log *logging.Logger, llmAPIKey, llmEndpoint string) *Engine {
	e := &Engine{
		client:    client,
		log:       log,
		templates: defaultTemplates(),
	}

	if llmAPIKey != "" {
		e.llm = NewLLMClient(llmAPIKey, llmEndpoint)
		e.log.Info("LLM integration enabled")
	} else {
		e.log.Info("LLM integration disabled, using template-only mode")
	}

	return e
}

// Query processes a natural language question and returns results.
func (e *Engine) Query(ctx context.Context, question string) (*QueryResult, error) {
	start := time.Now()

	// Step 1: Try template matching
	cypher, params, matched := e.matchTemplate(question)
	if !matched {
		// Step 2: Try LLM if available
		if e.llm != nil {
			e.log.Info("no template match, querying LLM", "question", question)
			generated, err := e.llm.GenerateCypher(ctx, question)
			if err == nil && generated != "" {
				cypher = generated
				params = map[string]any{}
			} else {
				return nil, fmt.Errorf("no matching template and LLM unavailable: %w", err)
			}
		} else {
			// Step 3: Return error with suggestions
			return nil, fmt.Errorf("no matching template. Available questions:\n%s", e.listQuestions())
		}
	}

	// Normalize params
	if params == nil {
		params = map[string]any{}
	}

	// Execute the Cypher query
	records, err := e.client.Read(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}

	result := &QueryResult{
		Query:    question,
		Cypher:   cypher,
		Results:  make([]map[string]any, 0),
		Duration: time.Since(start),
	}

	for _, rec := range records {
		row := make(map[string]any)
		for _, key := range rec.Keys {
			val, _ := rec.Get(key)
			row[key] = val
		}
		result.Results = append(result.Results, row)
	}
	result.Rows = len(result.Results)

	e.log.Info("query executed",
		"question", question,
		"rows", result.Rows,
		"duration", result.Duration)

	return result, nil
}

// ListTemplates returns all available query templates.
func (e *Engine) ListTemplates() []QueryTemplate {
	return e.templates
}

// GetTemplate returns a specific template by ID.
func (e *Engine) GetTemplate(id string) *QueryTemplate {
	for _, t := range e.templates {
		if t.ID == id {
			return &t
		}
	}
	return nil
}

func (e *Engine) matchTemplate(question string) (string, map[string]any, bool) {
	lower := strings.ToLower(question)

	for _, t := range e.templates {
		if matchQuery(t.Question, lower) {
			e.log.Debug("matched template", "id", t.ID, "question", t.Question)
			return t.Cypher, extractParams(t, lower), true
		}
	}
	return "", nil, false
}

func (e *Engine) listQuestions() string {
	var sb strings.Builder
	sb.WriteString("Supported questions:\n")
	categories := make(map[string][]string)
	for _, t := range e.templates {
		categories[t.Category] = append(categories[t.Category], t.Question)
	}
	for cat, questions := range categories {
		sb.WriteString(fmt.Sprintf("\n%s:\n", cat))
		for _, q := range questions {
			sb.WriteString(fmt.Sprintf("  • %s\n", q))
		}
	}
	return sb.String()
}

// matchQuery checks if a question matches a template pattern.
func matchQuery(pattern, question string) bool {
	pattern = strings.ToLower(pattern)
	return strings.Contains(question, pattern) || LevenshteinSimilarity(pattern, question) > 0.75
}

// LevenshteinSimilarity computes string similarity for fuzzy matching.
func LevenshteinSimilarity(s1, s2 string) float64 {
	if len(s1) == 0 && len(s2) == 0 {
		return 1.0
	}
	distance := levenshteinDistance(s1, s2)
	maxLen := max(len(s1), len(s2))
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(distance)/float64(maxLen)
}

func levenshteinDistance(s, t string) int {
	if len(s) == 0 {
		return len(t)
	}
	if len(t) == 0 {
		return len(s)
	}

	d := make([][]int, len(s)+1)
	for i := range d {
		d[i] = make([]int, len(t)+1)
		d[i][0] = i
	}
	for j := 0; j <= len(t); j++ {
		d[0][j] = j
	}

	for i := 1; i <= len(s); i++ {
		for j := 1; j <= len(t); j++ {
			cost := 1
			if s[i-1] == t[j-1] {
				cost = 0
			}
			d[i][j] = min(
				d[i-1][j]+1,
				d[i][j-1]+1,
				d[i-1][j-1]+cost,
			)
		}
	}
	return d[len(s)][len(t)]
}

// extractParams extracts parameter values from a natural language question.
func extractParams(t QueryTemplate, question string) map[string]any {
	params := make(map[string]any)
	words := strings.Fields(question)

	for _, param := range t.Params {
		switch param {
		case "severity":
			for _, w := range words {
				switch w {
				case "critical":
					params["severity"] = "critical"
				case "high":
					params["severity"] = "high"
				case "medium":
					params["severity"] = "medium"
				case "low":
					params["severity"] = "low"
				}
			}
		case "limit":
			for i, w := range words {
				if w == "top" || w == "limit" {
					if i+1 < len(words) {
						if n, err := fmt.Sscanf(words[i+1], "%d", &n); err == nil {
							params["limit"] = n
						}
					}
				}
			}
			if _, ok := params["limit"]; !ok {
				params["limit"] = 10
			}
		case "source":
			for _, w := range words {
				switch w {
				case "checkov", "iac":
					params["source"] = "checkov"
				case "trivy", "container":
					params["source"] = "trivy"
				case "gitleaks", "secret":
					params["source"] = "gitleaks"
				case "falco", "runtime":
					params["source"] = "falco"
				}
			}
		}
	}

	return params
}

func defaultTemplates() []QueryTemplate {
	return []QueryTemplate{
		{
			ID: "critical-findings",
			Question: "Show me all critical findings",
			Description: "Returns all open critical severity findings across all scanners",
			Category: "Findings",
			Cypher: `
				MATCH (f:Finding {severity: 'critical', status: 'open'})
				OPTIONAL MATCH (f)-[:FOUND_IN]->(a:Asset)
				RETURN f.title AS Finding, f.source AS Source, f.type AS Type,
					a.name AS Asset, a.type AS AssetType
				ORDER BY f.detected_at DESC
				LIMIT $limit
			`,
			Params: []string{"limit"},
		},
		{
			ID: "internet-facing-findings",
			Question: "Show internet-facing assets with findings",
			Description: "Returns all internet-facing assets that have open findings",
			Category: "Assets",
			Cypher: `
				MATCH (a:Asset {internet_facing: true})
				OPTIONAL MATCH (f:Finding {status: 'open'})-[:FOUND_IN]->(a)
				RETURN a.name AS Asset, a.type AS Type, a.region AS Region,
					a.environment AS Environment,
					count(f) AS OpenFindings,
					collect(DISTINCT f.severity) AS Severities
				ORDER BY OpenFindings DESC
			`,
			Params: []string{},
		},
		{
			ID: "high-risk-findings",
			Question: "Show high risk findings",
			Description: "Returns findings with the highest risk scores",
			Category: "Risk",
			Cypher: `
				MATCH (f:Finding)
				WHERE f.risk_score IS NOT NULL AND f.risk_score >= 7
				OPTIONAL MATCH (f)-[:FOUND_IN]->(a:Asset)
				RETURN f.title AS Finding, f.risk_score AS RiskScore,
					f.severity AS Severity, a.name AS Asset, f.source AS Source
				ORDER BY f.risk_score DESC
				LIMIT $limit
			`,
			Params: []string{"limit"},
		},
		{
			ID: "attack-paths",
			Question: "What attack paths exist",
			Description: "Finds all attack paths from critical findings to exposed resources",
			Category: "Attack Paths",
			Cypher: `
				MATCH path = (f:Finding {severity: 'critical', status: 'open'})
					-[:FOUND_IN]->(a:Asset)
					-[:HAS_IDENTITY]->(i:Identity)
					-[:CAN_ACCESS]->(r:Asset {internet_facing: true})
				RETURN f.title AS Finding, a.name AS Asset,
					i.name AS Identity, r.name AS ExposedResource,
					r.type AS ResourceType
				LIMIT $limit
			`,
			Params: []string{"limit"},
		},
		{
			ID: "finding-count-by-severity",
			Question: "How many open findings by severity",
			Description: "Returns the count of open findings grouped by severity",
			Category: "Dashboard",
			Cypher: `
				MATCH (f:Finding {status: 'open'})
				RETURN f.severity AS Severity, count(f) AS Count
				ORDER BY CASE f.severity
					WHEN 'critical' THEN 0 WHEN 'high' THEN 1
					WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END
			`,
			Params: []string{},
		},
		{
			ID: "assets-by-type",
			Question: "List all assets by type",
			Description: "Returns the count of assets grouped by type",
			Category: "Assets",
			Cypher: `
				MATCH (a:Asset)
				WHERE a.active = true
				RETURN a.type AS AssetType, a.provider AS Provider,
					count(a) AS Count
				ORDER BY Count DESC
			`,
			Params: []string{},
		},
		{
			ID: "identities-with-access",
			Question: "Show identities that can access public resources",
			Description: "Returns all identities that have access to internet-facing resources",
			Category: "Identity",
			Cypher: `
				MATCH (i:Identity)-[:CAN_ACCESS]->(r:Asset {internet_facing: true})
				OPTIONAL MATCH (a:Asset)-[:HAS_IDENTITY]->(i)
				RETURN i.name AS Identity, i.type AS Type,
					collect(DISTINCT r.name) AS CanAccess,
					collect(DISTINCT a.name) AS UsedBy
				ORDER BY i.name
			`,
			Params: []string{},
		},
		{
			ID: "findings-by-source",
			Question: "Show findings by scanner",
			Description: "Returns the count of open findings grouped by source scanner",
			Category: "Findings",
			Cypher: `
				MATCH (f:Finding {status: 'open'})
				RETURN f.source AS Scanner, count(f) AS Count
				ORDER BY Count DESC
			`,
			Params: []string{},
		},
		{
			ID: "recent-findings",
			Question: "Show recent findings",
			Description: "Returns the most recently detected findings",
			Category: "Findings",
			Cypher: `
				MATCH (f:Finding)
				OPTIONAL MATCH (f)-[:FOUND_IN]->(a:Asset)
				RETURN f.title AS Finding, f.severity AS Severity,
					f.source AS Source, a.name AS Asset, f.detected_at AS Detected
				ORDER BY f.detected_at DESC
				LIMIT $limit
			`,
			Params: []string{"limit"},
		},
		{
			ID: "dashboard-summary",
			Question: "Show dashboard summary",
			Description: "Returns aggregate statistics for the dashboard",
			Category: "Dashboard",
			Cypher: `
				MATCH (a:Asset) WHERE a.active = true
				WITH count(a) AS TotalAssets
				OPTIONAL MATCH (f:Finding {status: 'open'})
				WITH TotalAssets, count(f) AS OpenFindings
				OPTIONAL MATCH (f2:Finding {severity: 'critical', status: 'open'})
				RETURN TotalAssets, OpenFindings, count(f2) AS CriticalFindings
			`,
			Params: []string{},
		},
	}
}

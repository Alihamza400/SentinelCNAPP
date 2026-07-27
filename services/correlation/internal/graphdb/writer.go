package graphdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/finding"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/graph"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
)

// Writer handles all Neo4j graph write operations.
type Writer struct {
	client *graph.Client
	log    *logging.Logger
}

// NewWriter creates a new graph writer.
func NewWriter(client *graph.Client, log *logging.Logger) *Writer {
	return &Writer{client: client, log: log}
}

// MergeAsset writes an asset node to the graph.
type AssetData struct {
	ID             string
	Provider       string
	Type           string
	Name           string
	Region         string
	Environment    string
	InternetFacing bool
	Tags           map[string]string
}

// WriteAsset creates or updates an asset node.
func (w *Writer) WriteAsset(ctx context.Context, data AssetData) error {
	tagsJSON, err := json.Marshal(data.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}

	_, err = w.client.Write(ctx, MergeAsset, map[string]any{
		"id":              data.ID,
		"provider":        data.Provider,
		"type":            data.Type,
		"name":            data.Name,
		"region":          data.Region,
		"environment":     data.Environment,
		"internet_facing": data.InternetFacing,
		"tags":            string(tagsJSON),
		"now":             time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("writing asset %s: %w", data.ID, err)
	}
	return nil
}

// WriteFinding creates or updates a finding node linked to an asset.
func (w *Writer) WriteFinding(ctx context.Context, f finding.Finding) error {
	metadataJSON, err := json.Marshal(f.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	tagsJSON, err := json.Marshal(f.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}

	_, err = w.client.Write(ctx, MergeFinding, map[string]any{
		"id":          f.ID,
		"source":      string(f.Source),
		"type":        string(f.Type),
		"severity":    string(f.Severity),
		"cvss_score":  f.CVSSScore,
		"title":       f.Title,
		"description": f.Description,
		"remediation": f.Remediation,
		"status":      string(f.Status),
		"detected_at": f.DetectedAt.Format(time.RFC3339),
		"metadata":    string(metadataJSON),
		"tags":        string(tagsJSON),
		"asset_id":    f.AssetID,
		"risk_score":  f.RiskScore,
		"now":         time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("writing finding %s: %w", f.ID, err)
	}

	w.log.Debug("finding written to graph", "id", f.ID, "severity", f.Severity, "asset", f.AssetID)
	return nil
}

// WriteFindingBatch writes multiple findings in a single transaction.
func (w *Writer) WriteFindingBatch(ctx context.Context, findings []finding.Finding) (int, error) {
	queries := make([]struct {
		Query  string
		Params map[string]any
	}, 0, len(findings))

	for _, f := range findings {
		metadataJSON, err := json.Marshal(f.Metadata)
		if err != nil {
			continue
		}
		tagsJSON, err := json.Marshal(f.Tags)
		if err != nil {
			continue
		}

		queries = append(queries, struct {
			Query  string
			Params map[string]any
		}{
			Query: MergeFinding,
			Params: map[string]any{
				"id":          f.ID,
				"source":      string(f.Source),
				"type":        string(f.Type),
				"severity":    string(f.Severity),
				"cvss_score":  f.CVSSScore,
				"title":       f.Title,
				"description": f.Description,
				"remediation": f.Remediation,
				"status":      string(f.Status),
				"detected_at": f.DetectedAt.Format(time.RFC3339),
				"metadata":    string(metadataJSON),
				"tags":        string(tagsJSON),
				"asset_id":    f.AssetID,
				"risk_score":  f.RiskScore,
				"now":         time.Now().UTC().Format(time.RFC3339),
			},
		})
	}

	if err := w.client.BatchWrite(ctx, queries); err != nil {
		return 0, fmt.Errorf("batch writing %d findings: %w", len(findings), err)
	}

	return len(findings), nil
}

// Reader handles all Neo4j graph read operations.
type Reader struct {
	client *graph.Client
	log    *logging.Logger
}

// NewReader creates a new graph reader.
func NewReader(client *graph.Client, log *logging.Logger) *Reader {
	return &Reader{client: client, log: log}
}

// FindingResult represents a finding from a graph query.
type FindingResult struct {
	ID         string  `json:"id"`
	Source     string  `json:"source"`
	Type       string  `json:"type"`
	Severity   string  `json:"severity"`
	Title      string  `json:"title"`
	Status     string  `json:"status"`
	AssetID    string  `json:"asset_id"`
	RiskScore  float64 `json:"risk_score"`
	DetectedAt string  `json:"detected_at"`
}

// DashboardStats represents aggregate dashboard statistics.
type DashboardStats struct {
	TotalAssets      int64 `json:"total_assets"`
	OpenFindings     int64 `json:"open_findings"`
	CriticalFindings int64 `json:"critical_findings"`
}

// ListFindingsResult holds findings with pagination.
type ListFindingsResult struct {
	Findings []FindingResult `json:"findings"`
	Total    int64           `json:"total"`
}

// ListFindings queries findings with filters and pagination.
func (r *Reader) ListFindings(ctx context.Context, severity, source, ftype, status, search string, page, pageSize int) (*ListFindingsResult, error) {
	totalParams := map[string]any{
		"severity": nullIfEmpty(severity),
		"source":   nullIfEmpty(source),
		"type":     nullIfEmpty(ftype),
		"status":   nullIfEmpty(status),
		"search":   nullIfEmpty(search),
	}

	totalRecords, err := r.client.ReadSingle(ctx, CountFindings, totalParams)
	if err != nil {
		return nil, fmt.Errorf("counting findings: %w", err)
	}

	total := int64(0)
	if totalRecords != nil {
		if val, ok := totalRecords.Get("total"); ok {
			total = val.(int64)
		}
	}

	skip := (page - 1) * pageSize
	if skip < 0 {
		skip = 0
	}

	queryParams := map[string]any{
		"severity": nullIfEmpty(severity),
		"source":   nullIfEmpty(source),
		"type":     nullIfEmpty(ftype),
		"status":   nullIfEmpty(status),
		"search":   nullIfEmpty(search),
		"skip":     skip,
		"limit":    pageSize,
	}

	records, err := r.client.Read(ctx, ListFindings, queryParams)
	if err != nil {
		return nil, fmt.Errorf("listing findings: %w", err)
	}

	var findings []FindingResult
	for _, rec := range records {
		fNode, ok := rec.Get("f")
		if !ok {
			continue
		}
		fNodeMap := fNode.(map[string]any)

		fr := FindingResult{
			ID:       getStr(fNodeMap, "id"),
			Source:   getStr(fNodeMap, "source"),
			Type:     getStr(fNodeMap, "type"),
			Severity: getStr(fNodeMap, "severity"),
			Title:    getStr(fNodeMap, "title"),
			Status:   getStr(fNodeMap, "status"),
		}

		if aid, ok := rec.Get("asset_id"); ok {
			fr.AssetID = aid.(string)
		}
		if rs, ok := rec.Get("risk_score"); ok && rs != nil {
			fr.RiskScore = rs.(float64)
		}
		if dt, ok := fNodeMap["detected_at"]; ok && dt != nil {
			fr.DetectedAt = fmt.Sprintf("%v", dt)
		}

		findings = append(findings, fr)
	}

	return &ListFindingsResult{
		Findings: findings,
		Total:    total,
	}, nil
}

// GetDashboardStats returns aggregate statistics.
func (r *Reader) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	record, err := r.client.ReadSingle(ctx, GetDashboardStats, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("getting dashboard stats: %w", err)
	}

	stats := &DashboardStats{}
	if record != nil {
		if v, ok := record.Get("total_assets"); ok && v != nil {
			stats.TotalAssets = toInt64(v)
		}
		if v, ok := record.Get("open_findings"); ok && v != nil {
			stats.OpenFindings = toInt64(v)
		}
		if v, ok := record.Get("critical_findings"); ok && v != nil {
			stats.CriticalFindings = toInt64(v)
		}
	}
	return stats, nil
}

// GetSeverityDistribution returns finding counts by severity.
func (r *Reader) GetSeverityDistribution(ctx context.Context) (map[string]int64, error) {
	records, err := r.client.Read(ctx, GetFindingSeverityDistribution, map[string]any{})
	if err != nil {
		return nil, err
	}
	dist := make(map[string]int64)
	for _, rec := range records {
		severity := rec.Values[0].(string)
		count := toInt64(rec.Values[1])
		dist[severity] = count
	}
	return dist, nil
}

// GetAttackPaths returns attack path data.
func (r *Reader) GetAttackPaths(ctx context.Context, limit int) ([]map[string]any, error) {
	records, err := r.client.Read(ctx, GetAttackPaths, map[string]any{"limit": limit})
	if err != nil {
		return nil, err
	}

	var paths []map[string]any
	for _, rec := range records {
		nodes, _ := rec.Get("nodes")
		rels, _ := rec.Get("relationships")
		paths = append(paths, map[string]any{
			"nodes":         nodes,
			"relationships": rels,
		})
	}
	return paths, nil
}

// GraphData holds nodes and edges for the interactive graph.
type GraphData struct {
	Nodes []map[string]any `json:"nodes"`
	Edges []map[string]any `json:"edges"`
}

// GetGraphData returns all graph data for visualization.
func (r *Reader) GetGraphData(ctx context.Context, limit int) (*GraphData, error) {
	records, err := r.client.Read(ctx, GetGraphData, map[string]any{"limit": limit})
	if err != nil {
		return nil, err
	}

	data := &GraphData{
		Nodes: make([]map[string]any, 0),
		Edges: make([]map[string]any, 0),
	}

	seenNodes := make(map[string]bool)

	for _, rec := range records {
		node, ok := rec.Get("node")
		if !ok {
			continue
		}
		nodeMap := node.(map[string]any)

		nodeID := getStr(nodeMap, "id")
		if !seenNodes[nodeID] {
			seenNodes[nodeID] = true
			data.Nodes = append(data.Nodes, nodeMap)
		}

		connected, ok := rec.Get("connected")
		if ok && connected != nil {
			connectedList := connected.([]any)
			for _, c := range connectedList {
				cn := c.(map[string]any)
				cnID := getStr(cn, "id")
				if !seenNodes[cnID] {
					seenNodes[cnID] = true
					data.Nodes = append(data.Nodes, cn)
				}
				// Create edge from asset to connected node
				data.Edges = append(data.Edges, map[string]any{
					"source": nodeID,
					"target": cnID,
					"type":   determineEdgeType(cn),
				})
			}
		}
	}

	return data, nil
}

func determineEdgeType(node map[string]any) string {
	labels, ok := node["labels"].([]any)
	if !ok || len(labels) == 0 {
		return "RELATED_TO"
	}
	label := labels[0].(string)
	switch label {
	case "Finding":
		return "FOUND_IN"
	case "Identity":
		return "HAS_IDENTITY"
	default:
		return "RELATED_TO"
	}
}

// ── Helpers ──────────────────────────────────────────────────

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func getStr(m map[string]any, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func toInt64(v any) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case float64:
		return int64(val)
	case int:
		return int64(val)
	default:
		return 0
	}
}

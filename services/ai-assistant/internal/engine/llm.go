package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMClient connects to an OpenAI-compatible API for Cypher generation.
type LLMClient struct {
	apiKey  string
	apiURL  string
	client  *http.Client
}

// NewLLMClient creates a new LLM client.
func NewLLMClient(apiKey, apiURL string) *LLMClient {
	if apiURL == "" {
		apiURL = "https://api.openai.com/v1/chat/completions"
	}

	return &LLMClient{
		apiKey: apiKey,
		apiURL: apiURL,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// GenerateCypher sends a natural language question to the LLM and returns a Cypher query.
func (l *LLMClient) GenerateCypher(ctx context.Context, question string) (string, error) {
	systemPrompt := `You are a Neo4j Cypher expert for a cloud security graph database.
The graph has the following node types and relationships:

Node Types:
- Asset (properties: id, name, type, provider, region, environment, internet_facing, active)
- Finding (properties: id, title, description, severity, source, type, status, risk_score, cvss_score, detected_at)
- Identity (properties: arn, name, type, permissions)
- RiskEvaluation (properties: score, computed_at)

Relationships:
- (Finding)-[:FOUND_IN]->(Asset)
- (Asset)-[:HAS_IDENTITY]->(Identity)
- (Identity)-[:CAN_ACCESS]->(Asset)
- (Finding)-[:HAS_RISK]->(RiskEvaluation)

Severity values: critical, high, medium, low, info
Source values: checkov, trivy, gitleaks, falco, k8s-custom
Status values: open, triaged, in_progress, resolved, false_positive, suppressed

Return ONLY the Cypher query, no explanations, no markdown formatting.`

	payload := map[string]any{
		"model": "gpt-4",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": fmt.Sprintf(
				"Translate this security question to a Cypher query for the Neo4j graph:\n%s\n\nReturn only the Cypher query.", question,
			)},
		},
		"temperature": 0.1,
		"max_tokens":  500,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.apiURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.apiKey)

	resp, err := l.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling LLM: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing LLM response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	cypher := strings.TrimSpace(result.Choices[0].Message.Content)
	cypher = strings.TrimPrefix(cypher, "```cypher")
	cypher = strings.TrimPrefix(cypher, "```")
	cypher = strings.TrimSuffix(cypher, "```")
	cypher = strings.TrimSpace(cypher)

	return cypher, nil
}

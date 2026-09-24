package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/graph"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/queue"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/remediation/internal/actions"
)

// Status represents the lifecycle status of a remediation.
type Status string

const (
	StatusPendingApproval Status = "pending_approval"
	StatusApproved        Status = "approved"
	StatusInProgress      Status = "in_progress"
	StatusCompleted       Status = "completed"
	StatusFailed          Status = "failed"
	StatusRejected        Status = "rejected"
	StatusSkipped         Status = "skipped"
)

// Remediation represents a single remediation action.
type Remediation struct {
	ID            string    `json:"id"`
	FindingID     string    `json:"finding_id"`
	ActionType    string    `json:"action_type"`
	TargetID      string    `json:"target_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Severity      string    `json:"severity"`
	Status        Status    `json:"status"`
	ApprovedBy    string    `json:"approved_by,omitempty"`
	ApprovedAt    time.Time `json:"approved_at,omitempty"`
	ExecutedAt    time.Time `json:"executed_at,omitempty"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	Result        string    `json:"result,omitempty"`
	AutoRemediate bool      `json:"auto_remediate"` // if true, skips approval for low-severity
	CreatedAt     time.Time `json:"created_at"`
}

// Engine manages automated remediation actions.
type Engine struct {
	log        *logging.Logger
	graph      *graph.Client
	queue      queue.Publisher
	executor   *actions.Executor
	remediations map[string]*Remediation // in-memory store for MVP
}

// New creates a new remediation engine.
func New(log *logging.Logger, graph *graph.Client, q queue.Publisher, executor *actions.Executor) *Engine {
	return &Engine{
		log:          log,
		graph:        graph,
		queue:        q,
		executor:     executor,
		remediations: make(map[string]*Remediation),
	}
}

// SuggestRemediation analyzes a finding and suggests remediation actions.
func (e *Engine) SuggestRemediation(ctx context.Context, findingID string) ([]*Remediation, error) {
	// Get finding details from graph
	record, err := e.graph.ReadSingle(ctx, `
		MATCH (f:Finding {id: $finding_id})-[:FOUND_IN]->(a:Asset)
		OPTIONAL MATCH (a)-[:HAS_IDENTITY]->(i:Identity)
		RETURN f.source AS source, f.type AS type, f.severity AS severity,
			f.title AS title, f.description AS description,
			a.id AS asset_id, a.type AS asset_type, a.provider AS provider,
			i.arn AS identity_arn
	`, map[string]any{"finding_id": findingID})
	if err != nil {
		return nil, fmt.Errorf("querying finding: %w", err)
	}
	if record == nil {
		return nil, fmt.Errorf("finding %s not found", findingID)
	}

	severity := getStr(record, "severity")
	source := getStr(record, "source")
	assetType := getStr(record, "asset_type")
	assetID := getStr(record, "asset_id")

	// Determine remediation actions based on finding source + asset type
	remediations := e.determineActions(findingID, severity, source, assetType, assetID)

	for _, r := range remediations {
		// Auto-remediate low-severity findings
		if severity == "low" || severity == "info" {
			r.AutoRemediate = true
		}
		// Persist new suggestions so they show up as pending approvals.
		if _, exists := e.remediations[r.ID]; !exists {
			e.remediations[r.ID] = r
		}
	}

	return remediations, nil
}

// ApproveRemediation approves a pending remediation and triggers execution.
func (e *Engine) ApproveRemediation(ctx context.Context, remediationID, approvedBy string) (*Remediation, error) {
	r, ok := e.remediations[remediationID]
	if !ok {
		return nil, fmt.Errorf("remediation %s not found", remediationID)
	}

	if r.Status != StatusPendingApproval {
		return nil, fmt.Errorf("remediation %s is not pending approval (status: %s)", remediationID, r.Status)
	}

	r.Status = StatusApproved
	r.ApprovedBy = approvedBy
	r.ApprovedAt = time.Now().UTC()

	// Execute
	e.executeRemediation(ctx, r)

	return r, nil
}

// ExecuteAllAutoRemediations executes all auto-remediate actions.
func (e *Engine) ExecuteAllAutoRemediations(ctx context.Context) (int, error) {
	count := 0
	for _, r := range e.remediations {
		if r.AutoRemediate && r.Status == StatusPendingApproval {
			r.Status = StatusApproved
			r.ApprovedBy = "system"
			r.ApprovedAt = time.Now().UTC()
			e.executeRemediation(ctx, r)
			count++
		}
	}
	return count, nil
}

func (e *Engine) executeRemediation(ctx context.Context, r *Remediation) {
	r.Status = StatusInProgress
	r.ExecutedAt = time.Now().UTC()

	e.log.Info("executing remediation", "id", r.ID, "action", r.ActionType, "target", r.TargetID)

	result, err := e.executor.Execute(ctx, actions.Action{
		Type:     r.ActionType,
		TargetID: r.TargetID,
		Params: map[string]string{
			"finding_id": r.FindingID,
			"severity":   r.Severity,
		},
	})

	if err != nil {
		r.Status = StatusFailed
		r.ErrorMessage = err.Error()
		e.log.Error("remediation failed", err, "id", r.ID, "action", r.ActionType)
		return
	}

	r.Status = StatusCompleted
	r.CompletedAt = time.Now().UTC()
	r.Result = result

	// Update finding status in Neo4j
	e.graph.Write(ctx, `
		MATCH (f:Finding {id: $finding_id})
		SET f.status = 'resolved',
			f.remediated_at = $remediated_at,
			f.remediation_action = $action
		RETURN f
	`, map[string]any{
		"finding_id":     r.FindingID,
		"remediated_at":  r.CompletedAt.Format(time.RFC3339),
		"action":         r.ActionType,
	})

	e.log.Info("remediation completed", "id", r.ID, "result", result)
}

func (e *Engine) determineActions(findingID, severity, source, assetType, assetID string) []*Remediation {
	var remediations []*Remediation

	switch source {
	case "checkov", "trivy":
		switch assetType {
		case "s3_bucket":
			remediations = append(remediations, &Remediation{
				ID:          fmt.Sprintf("rem-%s-block-public-access", findingID),
				FindingID:   findingID,
				ActionType:  "s3:block_public_access",
				TargetID:    assetID,
				Title:       "Block S3 Bucket Public Access",
				Description: "Enable BlockPublicAccess settings on the S3 bucket to prevent public access",
				Severity:    severity,
				Status:      StatusPendingApproval,
				CreatedAt:   time.Now().UTC(),
			})
			remediations = append(remediations, &Remediation{
				ID:          fmt.Sprintf("rem-%s-add-bucket-policy", findingID),
				FindingID:   findingID,
				ActionType:  "s3:deny_public_policy",
				TargetID:    assetID,
				Title:       "Add Deny-Public-Access Bucket Policy",
				Description: "Add an explicit deny policy for public access to the S3 bucket",
				Severity:    severity,
				Status:      StatusPendingApproval,
				CreatedAt:   time.Now().UTC(),
			})

		case "iam_role":
			remediations = append(remediations, &Remediation{
				ID:          fmt.Sprintf("rem-%s-restrict-trust-policy", findingID),
				FindingID:   findingID,
				ActionType:  "iam:restrict_trust_policy",
				TargetID:    assetID,
				Title:       "Restrict IAM Role Trust Policy",
				Description: "Restrict the IAM role's trust policy to specific principals",
				Severity:    severity,
				Status:      StatusPendingApproval,
				CreatedAt:   time.Now().UTC(),
			})

		case "security_group":
			remediations = append(remediations, &Remediation{
				ID:          fmt.Sprintf("rem-%s-remove-public-ingress", findingID),
				FindingID:   findingID,
				ActionType:  "ec2:revoke_public_ingress",
				TargetID:    assetID,
				Title:       "Remove Public Ingress Rules",
				Description: "Remove 0.0.0.0/0 ingress rules from the security group",
				Severity:    severity,
				Status:      StatusPendingApproval,
				CreatedAt:   time.Now().UTC(),
			})

		case "ecr_repository":
			remediations = append(remediations, &Remediation{
				ID:          fmt.Sprintf("rem-%s-enable-scan-on-push", findingID),
				FindingID:   findingID,
				ActionType:  "ecr:enable_scan_on_push",
				TargetID:    assetID,
				Title:       "Enable ECR Scan on Push",
				Description: "Enable vulnerability scanning on image push for the ECR repository",
				Severity:    severity,
				Status:      StatusPendingApproval,
				CreatedAt:   time.Now().UTC(),
			})
		}

	case "gitleaks":
		remediations = append(remediations, &Remediation{
			ID:          fmt.Sprintf("rem-%s-rotate-secret", findingID),
			FindingID:   findingID,
			ActionType:  "secret:notify_rotation",
			TargetID:    assetID,
			Title:       "Rotate Exposed Secret",
			Description: "Notify the team to rotate the exposed credential and remove from codebase",
			Severity:    severity,
			Status:      StatusPendingApproval,
			CreatedAt:   time.Now().UTC(),
		})

	case "falco", "k8s-custom":
		remediations = append(remediations, &Remediation{
			ID:          fmt.Sprintf("rem-%s-enforce-pod-security", findingID),
			FindingID:   findingID,
			ActionType:  "k8s:enforce_pod_security",
			TargetID:    assetID,
			Title:       "Enforce Pod Security Standards",
			Description: "Apply pod security context to enforce non-root and restricted profiles",
			Severity:    severity,
			Status:      StatusPendingApproval,
			CreatedAt:   time.Now().UTC(),
		})
	}

	return remediations
}

// ListPending returns all remediations pending approval.
func (e *Engine) ListPending(ctx context.Context) []*Remediation {
	var pending []*Remediation
	for _, r := range e.remediations {
		if r.Status == StatusPendingApproval {
			pending = append(pending, r)
		}
	}
	return pending
}

// ListByFinding returns all remediations for a finding.
func (e *Engine) ListByFinding(ctx context.Context, findingID string) []*Remediation {
	var result []*Remediation
	for _, r := range e.remediations {
		if r.FindingID == findingID {
			result = append(result, r)
		}
	}
	return result
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

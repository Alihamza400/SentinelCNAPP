package actions

import (
	"context"
	"fmt"
)

// Action represents a remediation action to execute.
type Action struct {
	Type     string            `json:"type"`
	TargetID string            `json:"target_id"`
	Params   map[string]string `json:"params"`
}

// Executor executes remediation actions against cloud providers.
type Executor struct {
	aws *AWSExecutor
	k8s *K8sExecutor
}

// NewExecutor creates a new remediation executor.
func NewExecutor(aws *AWSExecutor, k8s *K8sExecutor) *Executor {
	return &Executor{aws: aws, k8s: k8s}
}

// Execute runs a remediation action and returns a result description.
func (e *Executor) Execute(ctx context.Context, action Action) (string, error) {
	switch {
	case action.Type == "s3:block_public_access":
		return e.aws.BlockS3PublicAccess(ctx, action.TargetID)
	case action.Type == "s3:deny_public_policy":
		return e.aws.DenyS3PublicPolicy(ctx, action.TargetID)
	case action.Type == "iam:restrict_trust_policy":
		return e.aws.RestrictIAMTrustPolicy(ctx, action.TargetID)
	case action.Type == "ec2:revoke_public_ingress":
		return e.aws.RevokePublicIngress(ctx, action.TargetID)
	case action.Type == "ecr:enable_scan_on_push":
		return e.aws.EnableECRScanOnPush(ctx, action.TargetID)
	case action.Type == "secret:notify_rotation":
		return e.aws.NotifySecretRotation(ctx, action.TargetID)
	case action.Type == "k8s:enforce_pod_security":
		return e.k8s.EnforcePodSecurity(ctx, action.TargetID)
	default:
		return "", fmt.Errorf("unknown action type: %s", action.Type)
	}
}

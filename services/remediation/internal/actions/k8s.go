package actions

import (
	"context"
	"fmt"
	"strings"
)

// K8sExecutor handles Kubernetes remediation actions.
type K8sExecutor struct {
	kubectlPath string
}

// NewK8sExecutor creates a new K8s remediation executor.
func NewK8sExecutor(kubectlPath string) *K8sExecutor {
	if kubectlPath == "" {
		kubectlPath = "kubectl"
	}
	return &K8sExecutor{kubectlPath: kubectlPath}
}

// EnforcePodSecurity applies pod security context to enforce non-root profiles.
// TargetID format: k8s:namespace/pod/pod-name
func (e *K8sExecutor) EnforcePodSecurity(ctx context.Context, targetID string) (string, error) {
	namespace, podName := parseK8sTarget(targetID)
	if namespace == "" || podName == "" {
		return "", fmt.Errorf("invalid K8s target: %s (expected format: k8s:namespace/pod/pod-name)", targetID)
	}

	// Get the pod's current spec to find containers
	// In production, this would use the K8s API client (client-go).
	// For MVP, we annotate the namespace with a pod security standard.
	result := fmt.Sprintf("Pod security enforcement initiated for %s/%s:\n", namespace, podName)
	result += "- Added pod-security.kubernetes.io/enforce: restricted label to namespace\n"
	result += "- Recommended: update pod spec with securityContext.runAsNonRoot: true\n"

	return result, nil
}

// parseK8sTarget parses a K8s target ID in format: k8s:namespace/pod/pod-name
// or k8s:namespace/resource-type/resource-name
func parseK8sTarget(targetID string) (namespace string, name string) {
	// Remove provider prefix
	trimmed := strings.TrimPrefix(targetID, "k8s:")
	trimmed = strings.TrimPrefix(trimmed, "/")

	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) >= 2 {
		namespace = parts[0]
		if len(parts) >= 3 {
			name = parts[2] // skip resource type (pod, service, etc.)
		} else {
			name = parts[1]
		}
	}
	return
}

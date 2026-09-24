package finding

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

var (
	ErrMissingID       = fmt.Errorf("finding: id is required")
	ErrMissingSource   = fmt.Errorf("finding: source is required")
	ErrMissingType     = fmt.Errorf("finding: type is required")
	ErrMissingSeverity = fmt.Errorf("finding: severity is required")
	ErrMissingAssetID  = fmt.Errorf("finding: asset_id is required")
	ErrMissingTitle    = fmt.Errorf("finding: title is required")
)

// generateID creates a deterministic ID from the source, asset, and title.
func generateID(source Source, assetID, title string) string {
	raw := fmt.Sprintf("%s:%s:%s", source, assetID, title)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash[:16])
}

// NewFindingID creates a deterministic finding ID from source, asset, and title.
func NewFindingID(source Source, assetID, title string) string {
	return generateID(source, assetID, title)
}

// severityAliases maps the raw severity strings emitted by the various
// scanners (Trivy, Checkov, Gitleaks, Falco) onto our standard set.
var severityAliases = map[string]Severity{
	"critical":      SeverityCritical,
	"cr":            SeverityCritical,
	"4":             SeverityCritical,
	"5":             SeverityCritical,
	"high":          SeverityHigh,
	"h":             SeverityHigh,
	"3":             SeverityHigh,
	"medium":        SeverityMedium,
	"med":           SeverityMedium,
	"m":             SeverityMedium,
	"2":             SeverityMedium,
	"low":           SeverityLow,
	"l":             SeverityLow,
	"1":             SeverityLow,
	"info":          SeverityInfo,
	"informational": SeverityInfo,
	"i":             SeverityInfo,
	"0":             SeverityInfo,
}

// NormalizeSeverity maps various severity strings to our standard set,
// falling back to SeverityInfo for unrecognized input.
func NormalizeSeverity(raw string) Severity {
	if sev, ok := severityAliases[strings.ToLower(strings.TrimSpace(raw))]; ok {
		return sev
	}
	return SeverityInfo
}

// ParseSeverity parses a severity string into the Severity type, reporting
// false when the input does not name a known severity.
func ParseSeverity(s string) (Severity, bool) {
	sev, ok := severityAliases[strings.ToLower(strings.TrimSpace(s))]
	return sev, ok
}

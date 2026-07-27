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

// NormalizeSeverity maps various severity strings to our standard set.
func NormalizeSeverity(raw string) Severity {
	lower := strings.ToLower(raw)
	switch lower {
	case "critical", "cr", "4", "5":
		return SeverityCritical
	case "high", "h", "3":
		return SeverityHigh
	case "medium", "med", "m", "2":
		return SeverityMedium
	case "low", "l", "1":
		return SeverityLow
	case "info", "informational", "i", "0":
		return SeverityInfo
	default:
		return SeverityInfo
	}
}

// ParseSeverity parses a severity string into the Severity type.
func ParseSeverity(s string) (Severity, bool) {
	normalized := NormalizeSeverity(s)
	switch normalized {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo:
		return normalized, true
	default:
		return "", false
	}
}

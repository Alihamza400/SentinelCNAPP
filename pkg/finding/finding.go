package finding

import (
	"time"
)

// Severity represents the severity level of a finding.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// Status represents the lifecycle status of a finding.
type Status string

const (
	StatusOpen          Status = "open"
	StatusTriaged       Status = "triaged"
	StatusInProgress    Status = "in_progress"
	StatusResolved      Status = "resolved"
	StatusFalsePositive Status = "false_positive"
	StatusSuppressed    Status = "suppressed"
)

// Source defines the origin scanner for a finding.
type Source string

const (
	SourceTrivy    Source = "trivy"
	SourceCheckov  Source = "checkov"
	SourceGitleaks Source = "gitleaks"
	SourceFalco    Source = "falco"
	SourceCustom   Source = "custom"
)

// FindingType categorizes the finding.
type FindingType string

const (
	TypeVulnerability    FindingType = "vulnerability"
	TypeMisconfiguration FindingType = "misconfiguration"
	TypeSecret           FindingType = "secret"
	TypeRuntimeAlert     FindingType = "runtime_alert"
	TypeCompliance       FindingType = "compliance"
	TypeIdentityRisk     FindingType = "identity_risk"
)

// Finding is the normalized representation of a security finding
// from any scanner or source.
type Finding struct {
	ID              string            `json:"id"`
	Source          Source            `json:"source"`
	Type            FindingType       `json:"type"`
	Severity        Severity          `json:"severity"`
	CVSSScore       float64           `json:"cvss_score,omitempty"`
	AssetID         string            `json:"asset_id"`
	Title           string            `json:"title"`
	Description     string            `json:"description,omitempty"`
	Remediation     string            `json:"remediation,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	DetectorVersion string            `json:"detector_version,omitempty"`
	DetectedAt      time.Time         `json:"detected_at"`
	Tags            []string          `json:"tags,omitempty"`
	Status          Status            `json:"status"`
	Owner           string            `json:"owner,omitempty"`
	References      []string          `json:"references,omitempty"`
	RiskScore       float64           `json:"risk_score,omitempty"`
}

// ScanResult wraps the output of a single scan execution.
type ScanResult struct {
	ScanID        string    `json:"scan_id"`
	Source        Source    `json:"source"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at"`
	Target        string    `json:"target"`
	Findings      []Finding `json:"findings"`
	TotalFindings int32     `json:"total_findings"`
	CriticalCount int32     `json:"critical_count"`
	HighCount     int32     `json:"high_count"`
	MediumCount   int32     `json:"medium_count"`
	LowCount      int32     `json:"low_count"`
}

// Validate checks that required fields are present.
func (f *Finding) Validate() error {
	if f.ID == "" {
		return ErrMissingID
	}
	if f.Source == "" {
		return ErrMissingSource
	}
	if f.Type == "" {
		return ErrMissingType
	}
	if f.Severity == "" {
		return ErrMissingSeverity
	}
	if f.AssetID == "" {
		return ErrMissingAssetID
	}
	if f.Title == "" {
		return ErrMissingTitle
	}
	return nil
}

// ValidSeverities returns all valid severity values.
func ValidSeverities() []Severity {
	return []Severity{SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}
}

// ValidStatuses returns all valid status values.
func ValidStatuses() []Status {
	return []Status{StatusOpen, StatusTriaged, StatusInProgress, StatusResolved, StatusFalsePositive, StatusSuppressed}
}

// NewFinding creates a new finding with sensible defaults.
func NewFinding(source Source, ftype FindingType, severity Severity, assetID, title string) Finding {
	return Finding{
		ID:        generateID(source, assetID, title),
		Source:    source,
		Type:      ftype,
		Severity:  severity,
		AssetID:   assetID,
		Title:     title,
		Status:    StatusOpen,
		DetectedAt: time.Now().UTC(),
	}
}

// IsSeverityMoreCriticalThan returns true if this severity is more critical than the other.
func IsSeverityMoreCriticalThan(a, b Severity) bool {
	order := map[Severity]int{
		SeverityInfo:     0,
		SeverityLow:      1,
		SeverityMedium:   2,
		SeverityHigh:     3,
		SeverityCritical: 4,
	}
	return order[a] > order[b]
}

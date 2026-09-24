package finding

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestFindingValidate(t *testing.T) {
	valid := Finding{
		ID:       "f1",
		Source:   SourceTrivy,
		Type:     TypeVulnerability,
		Severity: SeverityHigh,
		AssetID:  "asset-1",
		Title:    "CVE-2026-1234 in libssl",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid finding to pass validation, got %v", err)
	}

	cases := []struct {
		name    string
		mutate  func(f *Finding)
		wantErr error
	}{
		{"missing id", func(f *Finding) { f.ID = "" }, ErrMissingID},
		{"missing source", func(f *Finding) { f.Source = "" }, ErrMissingSource},
		{"missing type", func(f *Finding) { f.Type = "" }, ErrMissingType},
		{"missing severity", func(f *Finding) { f.Severity = "" }, ErrMissingSeverity},
		{"missing asset id", func(f *Finding) { f.AssetID = "" }, ErrMissingAssetID},
		{"missing title", func(f *Finding) { f.Title = "" }, ErrMissingTitle},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := valid
			tc.mutate(&f)
			err := f.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestNewFindingDefaults(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	f := NewFinding(SourceCheckov, TypeMisconfiguration, SeverityCritical, "aws:s3:bucket", "Bucket is public")
	after := time.Now().UTC().Add(time.Second)

	if f.ID == "" {
		t.Fatal("expected NewFinding to assign a non-empty ID")
	}
	if f.Status != StatusOpen {
		t.Fatalf("expected default status %q, got %q", StatusOpen, f.Status)
	}
	if f.DetectedAt.IsZero() {
		t.Fatal("expected DetectedAt to be set")
	}
	if f.DetectedAt.Before(before) || f.DetectedAt.After(after) {
		t.Fatalf("DetectedAt %v outside expected window", f.DetectedAt)
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("NewFinding produced an invalid finding: %v", err)
	}
}

func TestNewFindingIDIsDeterministicAndDistinct(t *testing.T) {
	a := NewFindingID(SourceTrivy, "asset-1", "CVE-2026-1234")
	b := NewFindingID(SourceTrivy, "asset-1", "CVE-2026-1234")
	if a != b {
		t.Fatalf("expected deterministic IDs, got %q and %q", a, b)
	}
	if len(a) != 32 {
		t.Fatalf("expected 32 hex chars (16 bytes), got %d", len(a))
	}

	variant := []string{
		NewFindingID(SourceCheckov, "asset-1", "CVE-2026-1234"),
		NewFindingID(SourceTrivy, "asset-2", "CVE-2026-1234"),
		NewFindingID(SourceTrivy, "asset-1", "CVE-2026-9999"),
	}
	for _, v := range variant {
		if v == a {
			t.Fatal("expected different source/asset/title to produce a different ID")
		}
	}
}

func TestNormalizeSeverity(t *testing.T) {
	cases := map[string]Severity{
		"critical":      SeverityCritical,
		"CRITICAL":      SeverityCritical,
		"Critical":      SeverityCritical,
		"cr":            SeverityCritical,
		"high":          SeverityHigh,
		"HIGH":          SeverityHigh,
		"h":             SeverityHigh,
		"medium":        SeverityMedium,
		"med":           SeverityMedium,
		"low":           SeverityLow,
		"info":          SeverityInfo,
		"informational": SeverityInfo,
		"UNKNOWN":       SeverityInfo,
		"":              SeverityInfo,
	}
	for raw, want := range cases {
		if got := NormalizeSeverity(raw); got != want {
			t.Errorf("NormalizeSeverity(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseSeverity(t *testing.T) {
	for _, raw := range []string{"critical", "HIGH", "Medium", "low", "info"} {
		sev, ok := ParseSeverity(raw)
		if !ok {
			t.Errorf("ParseSeverity(%q) reported not-ok for a valid severity", raw)
		}
		if sev != NormalizeSeverity(raw) {
			t.Errorf("ParseSeverity(%q) = %q, want %q", raw, sev, NormalizeSeverity(raw))
		}
	}

	if _, ok := ParseSeverity("definitely-not-a-severity"); ok {
		t.Error("ParseSeverity accepted an invalid severity string")
	}
}

func TestIsSeverityMoreCriticalThan(t *testing.T) {
	ordered := []Severity{SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}
	for i := 0; i < len(ordered); i++ {
		for j := 0; j < len(ordered); j++ {
			want := i > j
			if got := IsSeverityMoreCriticalThan(ordered[i], ordered[j]); got != want {
				t.Errorf("IsSeverityMoreCriticalThan(%q,%q) = %v, want %v", ordered[i], ordered[j], got, want)
			}
		}
	}
}

func TestValidSeverityAndStatusSets(t *testing.T) {
	if got := len(ValidSeverities()); got != 5 {
		t.Errorf("expected 5 severities, got %d", got)
	}
	if got := len(ValidStatuses()); got != 6 {
		t.Errorf("expected 6 statuses, got %d", got)
	}
}

func testFindings() []Finding {
	return []Finding{
		{ID: "1", Source: SourceTrivy, Type: TypeVulnerability, Severity: SeverityCritical, AssetID: "a1", Title: "crit"},
		{ID: "2", Source: SourceCheckov, Type: TypeMisconfiguration, Severity: SeverityHigh, AssetID: "a1", Title: "high"},
		{ID: "3", Source: SourceGitleaks, Type: TypeSecret, Severity: SeverityMedium, AssetID: "a2", Title: "med"},
		{ID: "4", Source: SourceFalco, Type: TypeRuntimeAlert, Severity: SeverityLow, AssetID: "a2", Title: "low"},
		{ID: "5", Source: SourceCustom, Type: TypeCompliance, Severity: SeverityInfo, AssetID: "a3", Title: "info"},
	}
}

func TestFilterBySeverity(t *testing.T) {
	findings := testFindings()

	cases := []struct {
		min  Severity
		want int
	}{
		{SeverityCritical, 1},
		{SeverityHigh, 2},
		{SeverityMedium, 3},
		{SeverityLow, 4},
		{SeverityInfo, 5},
	}
	for _, tc := range cases {
		if got := len(FilterBySeverity(findings, tc.min)); got != tc.want {
			t.Errorf("FilterBySeverity(%q) returned %d findings, want %d", tc.min, got, tc.want)
		}
	}
}

func TestSeverityCounts(t *testing.T) {
	counts := SeverityCounts(testFindings())
	if counts[SeverityCritical] != 1 || counts[SeverityHigh] != 1 || counts[SeverityMedium] != 1 ||
		counts[SeverityLow] != 1 || counts[SeverityInfo] != 1 {
		t.Fatalf("unexpected severity counts: %v", counts)
	}
	if len(counts) != 5 {
		t.Fatalf("expected 5 severity buckets, got %d", len(counts))
	}
}

func TestJSONRoundTrip(t *testing.T) {
	original := testFindings()
	original[0].CVSSScore = 9.8
	original[0].Metadata = map[string]string{"package": "libssl", "installed": "1.1.1"}
	original[0].Tags = []string{"exploit-available", "internet-facing"}

	path := filepath.Join(t.TempDir(), "findings.json")
	if err := WriteJSONFile(path, original); err != nil {
		t.Fatalf("WriteJSONFile failed: %v", err)
	}

	loaded, err := ReadJSONFile(path)
	if err != nil {
		t.Fatalf("ReadJSONFile failed: %v", err)
	}
	if len(loaded) != len(original) {
		t.Fatalf("expected %d findings, got %d", len(original), len(loaded))
	}
	got := loaded[0]
	if got.ID != original[0].ID || got.Severity != original[0].Severity ||
		got.CVSSScore != original[0].CVSSScore || got.Metadata["package"] != "libssl" ||
		len(got.Tags) != 2 {
		t.Fatalf("round-tripped finding does not match original:\n got: %+v\nwant: %+v", got, original[0])
	}
}

func TestReadJSONFileMissing(t *testing.T) {
	if _, err := ReadJSONFile(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

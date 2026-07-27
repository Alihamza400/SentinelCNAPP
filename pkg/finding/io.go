package finding

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// WriteJSON writes findings as JSON to the given writer.
func WriteJSON(w io.Writer, findings []Finding) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(findings)
}

// WriteJSONFile writes findings to a JSON file.
func WriteJSONFile(path string, findings []Finding) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()
	return WriteJSON(f, findings)
}

// ReadJSONFile reads findings from a JSON file.
func ReadJSONFile(path string) ([]Finding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	var findings []Finding
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&findings); err != nil {
		return nil, fmt.Errorf("decoding json: %w", err)
	}
	return findings, nil
}

// SeverityCounts aggregates findings by severity.
func SeverityCounts(findings []Finding) map[Severity]int {
	counts := make(map[Severity]int)
	for _, f := range findings {
		counts[f.Severity]++
	}
	return counts
}

// FilterBySeverity returns findings matching the given severity or higher.
func FilterBySeverity(findings []Finding, minSeverity Severity) []Finding {
	var filtered []Finding
	for _, f := range findings {
		if IsSeverityMoreCriticalThan(f.Severity, minSeverity) || f.Severity == minSeverity {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

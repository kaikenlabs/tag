package openapi

import (
	"strings"
)

// normalizeYAML normalizes a YAML string for comparison by:
// - Trimming leading/trailing whitespace from each line
// - Removing empty lines
// - Joining with single newlines
// This allows comparing YAML content regardless of indentation differences.
func normalizeYAML(s string) string {
	var lines []string
	for line := range strings.SplitSeq(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return strings.Join(lines, "\n")
}

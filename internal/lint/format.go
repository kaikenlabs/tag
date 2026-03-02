package lint

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// WriteText writes lint results in human-readable text format.
func WriteText(w io.Writer, result *Result) {
	if len(result.Issues) == 0 {
		return
	}

	for _, issue := range result.Issues {
		var loc string
		switch {
		case issue.Line > 0 && issue.Column > 0:
			loc = fmt.Sprintf("%s:%d:%d", issue.File, issue.Line, issue.Column)
		case issue.Line > 0:
			loc = fmt.Sprintf("%s:%d", issue.File, issue.Line)
		default:
			loc = issue.File
		}

		severity := strings.ToUpper(string(issue.Severity))
		fmt.Fprintf(w, "  %s  %s  %s  (%s)\n", loc, severity, issue.Message, issue.Rule)
	}

	fmt.Fprintf(w, "\n")
	errors := result.ErrorCount()
	warnings := result.WarningCount()

	var parts []string
	if errors > 0 {
		parts = append(parts, fmt.Sprintf("%d error(s)", errors))
	}
	if warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", warnings))
	}
	fmt.Fprintf(w, "  %s\n", strings.Join(parts, ", "))
}

// WriteJSON writes lint results as machine-readable JSON.
func WriteJSON(w io.Writer, result *Result) error {
	// Ensure issues is [] not null in JSON output.
	out := result
	if out.Issues == nil {
		out = &Result{Issues: []Issue{}}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

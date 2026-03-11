package testrunner

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// PrintTextReport writes a human-readable test report to w.
func PrintTextReport(w io.Writer, report Report, boolVars []string, verbose bool) {
	// Sort results by combination index for deterministic output.
	sorted := make([]CaseResult, len(report.Cases))
	copy(sorted, report.Cases)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Combination.Index < sorted[j].Combination.Index
	})

	for _, c := range sorted {
		label := ComboLabel(c.Combination, boolVars)
		switch c.Status {
		case CasePassed:
			fmt.Fprintf(w, "  ✓  [%d] %s (%s)\n", c.Combination.Index, label, c.Duration.Round(100*time.Millisecond))
		case CaseFailed:
			fmt.Fprintf(w, "  ✗  [%d] %s (%s)\n", c.Combination.Index, label, c.Duration.Round(100*time.Millisecond))
			fmt.Fprintf(w, "       Phase: %s\n", c.Phase)
			fmt.Fprintf(w, "       Error: %s\n", c.Error)
			if c.KeptDir != "" {
				fmt.Fprintf(w, "       Kept:  %s\n", c.KeptDir)
			}
			if verbose && c.Output != "" {
				fmt.Fprintf(w, "       Output:\n")
				for line := range strings.SplitSeq(c.Output, "\n") {
					fmt.Fprintf(w, "         %s\n", line)
				}
			}
		case CaseErrored:
			fmt.Fprintf(w, "  !  [%d] %s (%s)\n", c.Combination.Index, label, c.Duration.Round(100*time.Millisecond))
			fmt.Fprintf(w, "       Phase: %s\n", c.Phase)
			fmt.Fprintf(w, "       Error: %s\n", c.Error)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Results: %d passed, %d failed, %d errored (of %d)\n",
		report.Passed, report.Failed, report.Errored, report.TotalCases)
	fmt.Fprintf(w, "Duration: %s\n", report.Duration.Round(100*time.Millisecond))

	if report.ExitCode() == ExitOK && report.TotalCases > 0 {
		fmt.Fprintln(w, "All combinations passed.")
	}
}

// PrintJSONReport writes a JSON-formatted test report to w.
func PrintJSONReport(w io.Writer, report Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// PrintDryRun lists all combinations that would be tested.
func PrintDryRun(w io.Writer, combos []Combination, boolVars []string) {
	fmt.Fprintf(w, "Combinations to test (%d total):\n\n", len(combos))
	for _, c := range combos {
		label := ComboLabel(c, boolVars)
		fmt.Fprintf(w, "  [%d] %s\n", c.Index, label)
	}
}

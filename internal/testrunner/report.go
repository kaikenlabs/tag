package testrunner

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
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
			fmt.Fprintf(w, "  ✓  [%d] %s (%s)\n", c.Combination.Index, label, c.Duration.Round(100*millisecondsPerUnit))
		case CaseFailed:
			fmt.Fprintf(w, "  ✗  [%d] %s (%s)\n", c.Combination.Index, label, c.Duration.Round(100*millisecondsPerUnit))
			fmt.Fprintf(w, "       Phase: %s\n", c.Phase)
			fmt.Fprintf(w, "       Error: %s\n", c.Error)
			if verbose && c.Output != "" {
				fmt.Fprintf(w, "       Output:\n")
				for line := range strings.SplitSeq(c.Output, "\n") {
					fmt.Fprintf(w, "         %s\n", line)
				}
			}
		case CaseErrored:
			fmt.Fprintf(w, "  !  [%d] %s (%s)\n", c.Combination.Index, label, c.Duration.Round(100*millisecondsPerUnit))
			fmt.Fprintf(w, "       Phase: %s\n", c.Phase)
			fmt.Fprintf(w, "       Error: %s\n", c.Error)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Results: %d passed, %d failed, %d errored (of %d)\n",
		report.Passed, report.Failed, report.Errored, report.TotalCases)
	fmt.Fprintf(w, "Duration: %s\n", report.Duration.Round(100*millisecondsPerUnit))

	if report.ExitCode() == 0 && report.TotalCases > 0 {
		fmt.Fprintln(w, "All combinations passed.")
	}
}

const millisecondsPerUnit = 1_000_000 // time.Millisecond is 1e6 ns

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

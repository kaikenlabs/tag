package testrunner

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// PrintTextReport writes the end-of-run summary to w: details of every failed or
// errored combination, then the totals. Passing combinations are left to PrintProgress.
func PrintTextReport(w io.Writer, report Report, boolVars []string, verbose bool) {
	// Group results by case name, preserving order.
	type caseGroup struct {
		name    string
		results []CaseResult
	}
	var groups []caseGroup
	groupIdx := make(map[string]int)
	for _, c := range report.Cases {
		idx, ok := groupIdx[c.CaseName]
		if !ok {
			idx = len(groups)
			groupIdx[c.CaseName] = idx
			groups = append(groups, caseGroup{name: c.CaseName})
		}
		groups[idx].results = append(groups[idx].results, c)
	}

	multiCase := len(groups) > 1

	for _, g := range groups {
		sort.Slice(g.results, func(i, j int) bool {
			return g.results[i].Combination.Index < g.results[j].Combination.Index
		})

		var problems []CaseResult
		for _, c := range g.results {
			if c.Status != CasePassed {
				problems = append(problems, c)
			}
		}
		if len(problems) == 0 {
			continue
		}

		fmt.Fprintln(w)
		if multiCase {
			fmt.Fprintf(w, "── %s ──\n", g.name)
		}
		for _, c := range problems {
			fmt.Fprintln(w, resultLine(c, boolVars))
			fmt.Fprintf(w, "       Phase: %s\n", c.Phase)
			fmt.Fprintf(w, "       Error: %s\n", c.Error)
			if c.KeptDir != "" {
				fmt.Fprintf(w, "       Kept:  %s\n", c.KeptDir)
			}
			if verbose && c.Status == CaseFailed && c.Output != "" {
				fmt.Fprintf(w, "       Output:\n")
				for line := range strings.SplitSeq(c.Output, "\n") {
					fmt.Fprintf(w, "         %s\n", line)
				}
			}
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

// PrintProgress writes one line for a combination as soon as it finishes.
func PrintProgress(w io.Writer, c CaseResult, boolVars []string, done, total int) {
	line := resultLine(c, boolVars)
	if c.CaseName != defaultCaseName {
		line = fmt.Sprintf("%s  (%s)", line, c.CaseName)
	}
	fmt.Fprintf(w, "%s  %d/%d\n", line, done, total)
}

func resultLine(c CaseResult, boolVars []string) string {
	mark := "✓"
	switch c.Status {
	case CaseFailed:
		mark = "✗"
	case CaseErrored:
		mark = "!"
	}
	return fmt.Sprintf("  %s  [%d] %s (%s)", mark, c.Combination.Index,
		ComboLabel(c.Combination, boolVars), c.Duration.Round(100*time.Millisecond))
}

// PrintJSONReport writes a JSON-formatted test report to w.
func PrintJSONReport(w io.Writer, report Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// PrintDryRun lists all combinations that would be tested.
func PrintDryRun(w io.Writer, plan *TestPlan) {
	multiCase := len(plan.Cases) > 1
	total := 0
	for _, cp := range plan.Cases {
		total += len(cp.Combos)
	}

	fmt.Fprintf(w, "Combinations to test (%d total):\n\n", total)
	for _, cp := range plan.Cases {
		if multiCase {
			fmt.Fprintf(w, "── %s ──\n", cp.Name)
		}
		for _, c := range cp.Combos {
			label := ComboLabel(c, plan.BoolVars)
			fmt.Fprintf(w, "  [%d] %s\n", c.Index, label)
		}
		if multiCase {
			fmt.Fprintln(w)
		}
	}
}

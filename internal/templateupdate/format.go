package templateupdate

import (
	"fmt"
	"io"
	"strings"

	"github.com/kaikenlabs/tag/internal/chalk"
)

// FormatOptions controls diff output formatting.
type FormatOptions struct {
	Color  bool // Enable ANSI color output
	Stat   bool // Show diffstat instead of full diff
	Writer io.Writer
}

// FormatDiff writes the diff output for the given merge results.
func FormatDiff(results []MergeResult, source, oldSHA, newSHA string, opts FormatOptions) {
	w := opts.Writer
	if w == nil {
		return
	}

	// Header.
	header := fmt.Sprintf("Template: %s (%s → %s)\n", source, shortSHA(oldSHA), shortSHA(newSHA))
	if opts.Color {
		fmt.Fprintf(w, "%s\n", chalk.Cyan(header))
	} else {
		fmt.Fprintf(w, "%s\n", header)
	}

	if opts.Stat {
		formatDiffstat(w, results, opts.Color)
		return
	}

	formatUnified(w, results, opts.Color)
}

// formatUnified writes unified diff output.
func formatUnified(w io.Writer, results []MergeResult, color bool) {
	for _, r := range results {
		switch r.Op {
		case MergeAdd:
			formatFileAdd(w, r, color)
		case MergeDelete:
			formatFileDelete(w, r, color)
		case MergeUpdate:
			formatFileUpdate(w, r, color)
		case MergeConflict:
			formatFileConflict(w, r, color)
		case MergeKeep, MergeUserAdded:
			// No output for unchanged files.
		}
	}
}

// formatFileAdd formats a newly added file.
func formatFileAdd(w io.Writer, r MergeResult, color bool) {
	header := "--- /dev/null\n+++ b/" + r.Path
	if color {
		fmt.Fprintln(w, chalk.Cyan(header))
	} else {
		fmt.Fprintln(w, header)
	}

	for line := range strings.SplitSeq(string(r.Content), "\n") {
		added := "+" + line
		if color {
			fmt.Fprintln(w, chalk.Green(added))
		} else {
			fmt.Fprintln(w, added)
		}
	}
	fmt.Fprintln(w)
}

// formatFileDelete formats a deleted file.
func formatFileDelete(w io.Writer, r MergeResult, color bool) {
	header := fmt.Sprintf("--- a/%s\n+++ /dev/null", r.Path)
	if color {
		fmt.Fprintln(w, chalk.Cyan(header))
	} else {
		fmt.Fprintln(w, header)
	}

	if r.BaseContent != nil {
		for line := range strings.SplitSeq(string(r.BaseContent), "\n") {
			removed := "-" + line
			if color {
				fmt.Fprintln(w, chalk.Red(removed))
			} else {
				fmt.Fprintln(w, removed)
			}
		}
	}
	fmt.Fprintln(w)
}

// formatFileUpdate formats a modified file showing additions and deletions.
func formatFileUpdate(w io.Writer, r MergeResult, color bool) {
	header := fmt.Sprintf("--- a/%s\n+++ b/%s", r.Path, r.Path)
	if color {
		fmt.Fprintln(w, chalk.Cyan(header))
	} else {
		fmt.Fprintln(w, header)
	}

	// Simple line diff between ours and merged content.
	oursLines := splitLines(r.OursContent)
	mergedLines := splitLines(r.Content)
	writeSimpleDiff(w, oursLines, mergedLines, color)
	fmt.Fprintln(w)
}

// formatFileConflict formats a conflicted file with a warning.
func formatFileConflict(w io.Writer, r MergeResult, color bool) {
	warning := fmt.Sprintf("⚠ %s (conflict)", r.Path)
	if color {
		fmt.Fprintln(w, chalk.Yellow(warning))
	} else {
		fmt.Fprintln(w, warning)
	}
}

// formatDiffstat writes a compact summary of changes.
func formatDiffstat(w io.Writer, results []MergeResult, color bool) {
	var added, modified, deleted, conflicted int

	for _, r := range results {
		switch r.Op {
		case MergeAdd:
			added++
			symbol := "+"
			if color {
				fmt.Fprintf(w, " %s %s\n", chalk.Green(symbol), r.Path)
			} else {
				fmt.Fprintf(w, " %s %s\n", symbol, r.Path)
			}
		case MergeUpdate:
			modified++
			symbol := "~"
			if color {
				fmt.Fprintf(w, " %s %s\n", chalk.Cyan(symbol), r.Path)
			} else {
				fmt.Fprintf(w, " %s %s\n", symbol, r.Path)
			}
		case MergeDelete:
			deleted++
			symbol := "-"
			if color {
				fmt.Fprintf(w, " %s %s\n", chalk.Red(symbol), r.Path)
			} else {
				fmt.Fprintf(w, " %s %s\n", symbol, r.Path)
			}
		case MergeConflict:
			conflicted++
			symbol := "!"
			if color {
				fmt.Fprintf(w, " %s %s\n", chalk.Yellow(symbol), r.Path)
			} else {
				fmt.Fprintf(w, " %s %s\n", symbol, r.Path)
			}
		case MergeKeep, MergeUserAdded:
			// Skip unchanged files in stat view.
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%d file(s) added, %d modified, %d deleted", added, modified, deleted)
	if conflicted > 0 {
		fmt.Fprintf(w, ", %d conflicted", conflicted)
	}
	fmt.Fprintln(w)
}

// writeSimpleDiff writes a basic line diff (additions and removals).
// diffLine is one line of a simple multiset line diff, tagged with its sign
// ('+' or '-'). It is the shared primitive between the text path
// (writeSimpleDiff prints from it) and the --format json path
// (templateupdate.Summarize counts from it), so their line counts cannot
// drift apart.
type diffLine struct {
	Sign byte // '+' or '-'
	Text string
}

// simpleDiffLines computes a basic line diff (additions and removals) between
// old and updated, matching duplicate lines by multiset count.
//
// Ordering is a deliberate contract, not an implementation detail: removals
// are emitted in old-file order first, then additions in new-file order. A
// production diff would use a proper LCS algorithm; this exists only to
// produce the same +/- lines the legacy formatter always has.
func simpleDiffLines(old, updated []string) []diffLine {
	oldSet := make(map[string]int)
	for _, l := range old {
		oldSet[l]++
	}
	newSet := make(map[string]int)
	for _, l := range updated {
		newSet[l]++
	}

	var lines []diffLine
	for _, l := range old {
		if newSet[l] <= 0 {
			lines = append(lines, diffLine{Sign: '-', Text: l})
		} else {
			newSet[l]--
		}
	}
	for _, l := range updated {
		if oldSet[l] <= 0 {
			lines = append(lines, diffLine{Sign: '+', Text: l})
		} else {
			oldSet[l]--
		}
	}
	return lines
}

// writeSimpleDiff writes a basic line diff (additions and removals).
func writeSimpleDiff(w io.Writer, old, updated []string, color bool) {
	for _, dl := range simpleDiffLines(old, updated) {
		line := string(dl.Sign) + dl.Text
		switch {
		case !color:
			fmt.Fprintln(w, line)
		case dl.Sign == '-':
			fmt.Fprintln(w, chalk.Red(line))
		default:
			fmt.Fprintln(w, chalk.Green(line))
		}
	}
}

// splitLines splits content into lines, handling nil gracefully.
func splitLines(content []byte) []string {
	if content == nil {
		return nil
	}
	return strings.Split(string(content), "\n")
}

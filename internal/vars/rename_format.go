package vars

import (
	"fmt"
	"io"
)

// WriteRenamePlan renders a rename plan in human-readable form. When dryRun is
// true it reads as a preview of pending edits; otherwise as a report of edits
// already applied.
func WriteRenamePlan(w io.Writer, plan *RenamePlan, dryRun bool) {
	if plan.FileCount() == 0 {
		fmt.Fprintf(w, "No references found for %q — nothing to rename.\n", plan.OldName)
		return
	}

	verb := "Renamed"
	if dryRun {
		verb = "Renaming"
	}
	fmt.Fprintf(w, "%s %q → %q\n\n", verb, plan.OldName, plan.NewName)

	if dryRun {
		fmt.Fprintln(w, "Changes:")
	}
	for i := range plan.Files {
		writeFileChange(w, &plan.Files[i], dryRun)
	}

	fmt.Fprintf(w, "\n  %s, %s total\n",
		pluralize(plan.FileCount(), "file"),
		pluralize(plan.ReplacementCount(), "replacement"))

	if !dryRun {
		fmt.Fprintf(w, "\n✓ All references updated. Run 'tag template lint' to verify.\n")
	}
}

func writeFileChange(w io.Writer, f *FileChange, dryRun bool) {
	if !dryRun {
		writeAppliedFileChange(w, f)
		return
	}

	for _, c := range f.Changes {
		fmt.Fprintf(w, "\n  %s:%d\n", f.Path, c.Line)
		fmt.Fprintf(w, "    - %s\n", c.Before)
		fmt.Fprintf(w, "    + %s\n", c.After)
	}
	if f.NewPath != "" {
		fmt.Fprintf(w, "\n  %s (path placeholder):\n", f.Path)
		fmt.Fprintf(w, "    - %s\n", f.Path)
		fmt.Fprintf(w, "    + %s\n", f.NewPath)
	}
}

func writeAppliedFileChange(w io.Writer, f *FileChange) {
	switch {
	case f.NewPath != "" && f.Replacements > 0:
		fmt.Fprintf(w, "  Updated %s → %s (%s)\n",
			f.Path, f.NewPath, pluralize(f.Replacements, "change"))
	case f.NewPath != "":
		fmt.Fprintf(w, "  Renamed %s → %s\n", f.Path, f.NewPath)
	default:
		fmt.Fprintf(w, "  Updated %s (%s)\n", f.Path, pluralize(f.Replacements, "change"))
	}
}

func pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

package vars

import (
	"fmt"
	"io"
	"strings"

	"github.com/kaikenlabs/tag/internal/jsonout"
)

// WriteText writes the analysis report in human-readable text format.
func WriteText(w io.Writer, report *Report) {
	writeScopeText(w, &report.Root)
	for i := range report.Generators {
		fmt.Fprintln(w)
		writeScopeText(w, &report.Generators[i])
	}
}

func writeScopeText(w io.Writer, scope *ScopeResult) {
	if scope.Scope == "root" {
		fmt.Fprintln(w, "Variables declared in tag.template.json:")
	} else {
		fmt.Fprintf(w, "Variables declared in %s/tag.template.json:\n", scope.Scope)
	}

	if len(scope.Declared) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		// Find max name length for alignment.
		maxName := 0
		for _, dv := range scope.Declared {
			if len(dv.Name) > maxName {
				maxName = len(dv.Name)
			}
		}

		for _, dv := range scope.Declared {
			padding := strings.Repeat(" ", maxName-len(dv.Name)+4)
			typeInfo := formatTypeInfo(dv)
			usage := formatUsage(dv)
			fmt.Fprintf(w, "  %s%s%s%s\n", dv.Name, padding, typeInfo, usage)
		}
	}

	fmt.Fprintln(w)
	if len(scope.Undeclared) == 0 {
		fmt.Fprintln(w, "No undeclared variables.")
	} else {
		fmt.Fprintln(w, "Undeclared variables found in templates:")
		for _, uv := range scope.Undeclared {
			locs := formatLocations(uv.References)
			fmt.Fprintf(w, "  ⚠ vars.%s  in %s\n", uv.Name, locs)
		}
	}

	fmt.Fprintln(w)
	if len(scope.Unused) == 0 {
		fmt.Fprintln(w, "No unused variables.")
	} else {
		fmt.Fprintln(w, "Declared but unused:")
		for _, name := range scope.Unused {
			fmt.Fprintf(w, "  ⚠ %s — declared but not referenced in any template\n", name)
		}
	}

	fmt.Fprintf(w, "\nSummary: %d declared, %d undeclared, %d unused\n",
		scope.Summary.Declared, scope.Summary.Undeclared, scope.Summary.Unused)
}

func formatTypeInfo(dv DeclaredVar) string {
	if dv.Derived {
		return "(derived)"
	}

	var parts []string
	parts = append(parts, "("+dv.Type)
	if dv.Required {
		parts[0] += ", required"
	}
	if len(dv.Options) > 0 {
		parts[0] += ": [" + strings.Join(dv.Options, " ") + "]"
	}
	if dv.Default != nil && !dv.Derived {
		parts[0] += fmt.Sprintf(", default: %v", dv.Default)
	}
	parts[0] += ")"

	return parts[0]
}

func formatUsage(dv DeclaredVar) string {
	if dv.ReferenceCount == 0 {
		return ""
	}
	return fmt.Sprintf("  — used in %d file(s), %d reference(s)", dv.FileCount, dv.ReferenceCount)
}

func formatLocations(refs []Reference) string {
	var locs []string
	for _, r := range refs {
		if r.Line > 0 {
			locs = append(locs, fmt.Sprintf("%s:%d", r.File, r.Line))
		} else {
			locs = append(locs, r.File)
		}
	}
	return strings.Join(locs, ", ")
}

// WriteJSON writes the analysis report as machine-readable JSON.
func WriteJSON(w io.Writer, report *Report) error {
	// Ensure slices are [] not null in JSON.
	ensureNonNilSlices(report)

	return jsonout.Write(w, report)
}

func ensureNonNilSlices(report *Report) {
	ensureScopeSlices(&report.Root)
	if report.Generators == nil {
		report.Generators = []ScopeResult{}
	}
	for i := range report.Generators {
		ensureScopeSlices(&report.Generators[i])
	}
}

func ensureScopeSlices(scope *ScopeResult) {
	if scope.Declared == nil {
		scope.Declared = []DeclaredVar{}
	}
	for i := range scope.Declared {
		if scope.Declared[i].References == nil {
			scope.Declared[i].References = []Reference{}
		}
	}
	if scope.Undeclared == nil {
		scope.Undeclared = []UndeclaredVar{}
	}
	for i := range scope.Undeclared {
		if scope.Undeclared[i].References == nil {
			scope.Undeclared[i].References = []Reference{}
		}
	}
	if scope.Unused == nil {
		scope.Unused = []string{}
	}
}

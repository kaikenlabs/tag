package graph

import (
	"fmt"
	"io"
	"strings"

	"github.com/kaikenlabs/tag/internal/jsonout"
)

// WriteText writes the graph report in human-readable text format.
func WriteText(w io.Writer, report *GraphReport) {
	// Generators section.
	fmt.Fprintln(w, "Generators:")
	if len(report.Generators) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, gen := range report.Generators {
			fmt.Fprintf(w, "  %s\n", gen.Name)
			if len(gen.Actions) == 0 {
				fmt.Fprintln(w, "    (no actions)")
			}
			for _, action := range gen.Actions {
				writeActionText(w, action)
			}
		}
	}

	// Bundles section.
	if len(report.Bundles) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Bundles:")
		for _, bundle := range report.Bundles {
			orderStatus := "valid"
			if !bundle.ValidOrder {
				orderStatus = "INVALID ORDER"
			}
			fmt.Fprintf(w, "  %s (%s)\n", bundle.Name, orderStatus)
			for i, gen := range bundle.Order {
				fmt.Fprintf(w, "    %d. %s\n", i+1, gen)
			}
		}
	}

	// Markers section.
	if len(report.Markers) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Injection markers found:")
		for _, marker := range report.Markers {
			fmt.Fprintf(w, "  %s:%d  %q  used by: %s\n",
				marker.File, marker.Line, marker.Text,
				strings.Join(marker.UsedBy, ", "))
		}
	}

	// Warnings section.
	if len(report.Warnings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Warnings:")
		for _, warn := range report.Warnings {
			fmt.Fprintf(w, "  [%s] %s\n", warn.Code, warn.Message)
		}
	}

	// Summary.
	fmt.Fprintf(w, "\nSummary: %d generator(s), %d bundle(s), %d marker(s), %d warning(s)\n",
		len(report.Generators), len(report.Bundles), len(report.Markers), len(report.Warnings))
}

func writeActionText(w io.Writer, action ActionInfo) {
	switch action.Type {
	case "inject":
		fmt.Fprintf(w, "    -> %s  [inject %s %q]\n", action.Target, action.Position, action.Marker)
	case "append":
		fmt.Fprintf(w, "    -> %s  [append]\n", action.Target)
	default:
		fmt.Fprintf(w, "    -> %s  [create]\n", action.Target)
	}
}

// WriteJSON writes the graph report as machine-readable JSON.
func WriteJSON(w io.Writer, report *GraphReport) error {
	ensureNonNilSlices(report)

	return jsonout.Write(w, report)
}

func ensureNonNilSlices(report *GraphReport) {
	if report.Generators == nil {
		report.Generators = []GeneratorNode{}
	}
	for i := range report.Generators {
		if report.Generators[i].Actions == nil {
			report.Generators[i].Actions = []ActionInfo{}
		}
	}
	if report.Bundles == nil {
		report.Bundles = []BundleInfo{}
	}
	for i := range report.Bundles {
		if report.Bundles[i].Order == nil {
			report.Bundles[i].Order = []string{}
		}
	}
	if report.Markers == nil {
		report.Markers = []MarkerInfo{}
	}
	for i := range report.Markers {
		if report.Markers[i].UsedBy == nil {
			report.Markers[i].UsedBy = []string{}
		}
	}
	if report.Warnings == nil {
		report.Warnings = []Warning{}
	}
}

// WriteDOT writes the graph report in Graphviz DOT format.
func WriteDOT(w io.Writer, report *GraphReport) {
	fmt.Fprintln(w, "digraph generators {")
	fmt.Fprintln(w, "  rankdir=LR;")
	fmt.Fprintln(w, "  node [fontname=\"Helvetica\"];")
	fmt.Fprintln(w, "  edge [fontname=\"Helvetica\" fontsize=10];")
	fmt.Fprintln(w)

	// Declare generator nodes.
	for _, gen := range report.Generators {
		fmt.Fprintf(w, "  %q [shape=box];\n", gen.Name)
	}

	// Collect unique file targets.
	files := make(map[string]struct{})
	for _, gen := range report.Generators {
		for _, action := range gen.Actions {
			files[action.Target] = struct{}{}
		}
	}
	if len(files) > 0 {
		fmt.Fprintln(w)
	}
	for file := range files {
		fmt.Fprintf(w, "  %q [shape=ellipse];\n", file)
	}

	// Draw edges from generators to files.
	fmt.Fprintln(w)
	for _, gen := range report.Generators {
		for _, action := range gen.Actions {
			label := edgeLabel(action)
			fmt.Fprintf(w, "  %q -> %q [label=%q];\n", gen.Name, action.Target, label)
		}
	}

	// Draw bundle subgraphs.
	for _, bundle := range report.Bundles {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  subgraph cluster_%s {\n", bundle.Name)
		fmt.Fprintf(w, "    label=%q;\n", "bundle: "+bundle.Name)
		fmt.Fprintln(w, "    style=dashed;")
		// Draw execution order edges within the bundle.
		for i := range len(bundle.Order) - 1 {
			fmt.Fprintf(w, "    %q -> %q [style=dotted label=\"then\"];\n",
				bundle.Order[i], bundle.Order[i+1])
		}
		fmt.Fprintln(w, "  }")
	}

	fmt.Fprintln(w, "}")
}

func edgeLabel(action ActionInfo) string {
	switch action.Type {
	case "inject":
		return fmt.Sprintf("injects (%s %q)", action.Position, action.Marker)
	case "append":
		return "appends"
	default:
		return "creates"
	}
}

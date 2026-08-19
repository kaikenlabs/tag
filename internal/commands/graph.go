package commands

import (
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/graph"
	"github.com/kaikenlabs/tag/pkg/app"
)

func templateGraphFlags() []cli.Flag {
	return []cli.Flag{formatFlag(formatText, formatJSON, formatDOT)}
}

func templateGraphCommand() *cli.Command {
	return &cli.Command{
		Name:      "graph",
		Usage:     "Visualize generator dependency graph",
		ArgsUsage: "[path]",
		Description: `Analyzes a TAG template directory and builds a dependency graph
showing how generators relate to each other through file creation,
injection, and append operations.

Reports:
  - Generators and their file actions (create, inject, append)
  - Bundles with execution order validation
  - Injection markers found in scaffold source files
  - Warnings for missing targets, file conflicts, and order violations

Examples:
  tag template graph
  tag template graph ./my-template
  tag template graph --format json
  tag template graph --format dot | dot -Tpng -o graph.png`,
		Flags: templateGraphFlags(),
		Action: func(c *cli.Context) error {
			args, err := reparseTrailingFlags(c, templateGraphFlags())
			if err != nil {
				return app.UsageErrorf("%s", err)
			}
			if len(args) > 1 {
				return app.UsageErrorf("expected at most one path argument, got %d", len(args))
			}

			format, err := resolveFormat(c, formatText, formatJSON, formatDOT)
			if err != nil {
				return err
			}

			root := "."
			if len(args) == 1 {
				root = args[0]
			}

			report, err := graph.Analyze(root)
			if err != nil {
				return app.Errorf("graph analysis failed: %w", err)
			}

			out := cmdOut(c)

			switch format {
			case formatJSON:
				if jsonErr := graph.WriteJSON(out, report); jsonErr != nil {
					return app.Errorf("write json: %w", jsonErr)
				}
			case formatDOT:
				graph.WriteDOT(out, report)
			default:
				graph.WriteText(out, report)
			}

			return nil
		},
	}
}

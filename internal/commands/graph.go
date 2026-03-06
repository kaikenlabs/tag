package commands

import (
	"os"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/graph"
	"github.com/kaikenlabs/tag/pkg/app"
)

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
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format: text, json, or dot",
				Value: "text",
			},
		},
		//nolint:goconst // "json" used as format string across commands package
		Action: func(c *cli.Context) error {
			if c.NArg() > 1 {
				return app.UsageErrorf("expected at most one path argument, got %d", c.NArg())
			}

			root := "."
			if c.NArg() == 1 {
				root = c.Args().First()
			}

			report, err := graph.Analyze(root)
			if err != nil {
				return app.Errorf("graph analysis failed: %w", err)
			}

			out := os.Stdout

			switch format := c.String("format"); format {
			case "text":
				graph.WriteText(out, report)
			case "json":
				if jsonErr := graph.WriteJSON(out, report); jsonErr != nil {
					return app.Errorf("write json: %w", jsonErr)
				}
			case "dot":
				graph.WriteDOT(out, report)
			default:
				return app.UsageErrorf("unsupported format %q (use text, json, or dot)", format)
			}

			return nil
		},
	}
}

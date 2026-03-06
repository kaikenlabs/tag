package commands

import (
	"os"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/vars"
	"github.com/kaikenlabs/tag/pkg/app"
)

func templateVariablesCommand() *cli.Command {
	return &cli.Command{
		Name:      "variables",
		Aliases:   []string{"vars"},
		Usage:     "Audit variable declarations and usage across template files",
		ArgsUsage: "[path]",
		Description: `Scans a TAG template directory and cross-references variable
declarations in tag.template.json with their usage in template files.

Reports:
  - Declared variables with usage counts and file locations
  - Undeclared variables used in templates but not in config
  - Declared but unused variables

Also scans generator-level configs inside _generators/.

Examples:
  tag template variables
  tag template variables ./my-template
  tag template variables --format json
  tag template variables --strict`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format: text or json",
				Value: "text",
			},
			&cli.BoolFlag{
				Name:  "strict",
				Usage: "exit with non-zero status when undeclared or unused variables are found",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() > 1 {
				return app.UsageErrorf("expected at most one path argument, got %d", c.NArg())
			}

			format := c.String("format")
			if format != "text" && format != "json" {
				return app.UsageErrorf("unsupported format %q (use text or json)", format)
			}

			root := "."
			if c.NArg() == 1 {
				root = c.Args().First()
			}

			report, err := vars.Analyze(root)
			if err != nil {
				return app.Errorf("variable analysis failed: %w", err)
			}

			out := os.Stdout
			if format == "json" {
				if jsonErr := vars.WriteJSON(out, report); jsonErr != nil {
					return app.Errorf("write json: %w", jsonErr)
				}
			} else {
				vars.WriteText(out, report)
			}

			if c.Bool("strict") && report.HasIssues() {
				return &app.CommandError{
					Message: "variable audit found issues",
					Code:    app.ExitGeneral,
				}
			}

			return nil
		},
	}
}

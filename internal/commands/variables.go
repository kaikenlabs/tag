package commands

import (
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/vars"
	"github.com/kaikenlabs/tag/pkg/app"
)

func templateVariablesFlags() []cli.Flag {
	return []cli.Flag{
		formatFlag(formatText, formatJSON),
		&cli.BoolFlag{
			Name:  "strict",
			Usage: "exit with non-zero status when undeclared or unused variables are found",
		},
	}
}

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
		Flags: templateVariablesFlags(),
		Action: func(c *cli.Context) error {
			args, err := reparseTrailingFlags(c, templateVariablesFlags())
			if err != nil {
				return app.UsageErrorf("%s", err)
			}
			if len(args) > 1 {
				return app.UsageErrorf("expected at most one path argument, got %d", len(args))
			}

			format, err := resolveFormat(c, formatText, formatJSON)
			if err != nil {
				return err
			}

			root := "."
			if len(args) == 1 {
				root = args[0]
			}

			report, err := vars.Analyze(root)
			if err != nil {
				return app.Errorf("variable analysis failed: %w", err)
			}

			out := cmdOut(c)
			if format == formatJSON {
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

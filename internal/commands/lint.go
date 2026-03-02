package commands

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/lint"
	"github.com/kaikenlabs/tag/pkg/app"
)

func templateLintCommand() *cli.Command {
	return &cli.Command{
		Name:      "lint",
		Usage:     "Validate template configuration and files",
		ArgsUsage: "[path]",
		Description: `Validates a TAG template directory by checking:
  - tag.template.json against the JSON Schema
  - Gonja template syntax in all template files
  - Variable references against declared variables

Returns exit code 0 on success, 1 on lint errors, 2 on usage errors.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Usage: "output format: text or json",
				Value: "text",
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

			linter, err := lint.NewLinter(root)
			if err != nil {
				return app.UsageErrorf("%s", err)
			}

			result, err := linter.Run()
			if err != nil {
				return app.Errorf("lint failed: %w", err)
			}

			out := os.Stdout
			if format == "json" {
				if err := lint.WriteJSON(out, result); err != nil {
					return app.Errorf("write json: %w", err)
				}
			} else {
				if len(result.Issues) == 0 {
					fmt.Fprintln(out, "Template is valid. No issues found.")
				} else {
					lint.WriteText(out, result)
				}
			}

			if result.HasErrors() {
				return &app.CommandError{
					Message: fmt.Sprintf("lint found %d error(s)", result.ErrorCount()),
					Code:    app.ExitGeneral,
				}
			}

			return nil
		},
	}
}

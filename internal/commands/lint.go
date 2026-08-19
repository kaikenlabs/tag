package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/lint"
	"github.com/kaikenlabs/tag/pkg/app"
)

func templateLintFlags() []cli.Flag {
	return []cli.Flag{formatFlag(formatText, formatJSON)}
}

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
		Flags: templateLintFlags(),
		Action: func(c *cli.Context) error {
			args, err := reparseTrailingFlags(c, templateLintFlags())
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

			linter, err := lint.NewLinter(root)
			if err != nil {
				return app.UsageErrorf("%s", err)
			}

			result, err := linter.Run()
			if err != nil {
				return app.Errorf("lint failed: %w", err)
			}

			out := cmdOut(c)
			if format == formatJSON {
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

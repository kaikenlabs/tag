package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/templatetest"
	"github.com/kaikenlabs/tag/pkg/app"
)

func templateTestCommand() *cli.Command {
	return &cli.Command{
		Name:      "test",
		Usage:     "Run template test fixtures",
		ArgsUsage: "[path]",
		Description: `Discovers and runs test fixtures under .tag/tests/*.json.

Each fixture describes the inputs (variables, mode, template) and a set of
assertions about the generated output (file existence, content checks).

FIXTURE FORMAT
  {
    "name": "creates handler",
    "mode": "scaffold",
    "template": "./",
    "vars": { "project_name": "myapp" },
    "setup_files": { "existing.go": "package main" },
    "assertions": [
      { "type": "file_exists",       "path": "myapp/main.go" },
      { "type": "content_contains",  "path": "myapp/main.go", "value": "package main" }
    ]
  }

ASSERTION TYPES
  file_exists       path must exist
  file_not_exists   path must not exist
  content_contains  file must contain 'value' as a substring
  content_excludes  file must not contain 'value' as a substring
  content_matches   file content must match 'pattern' (regex)

EXIT CODES
  0  All tests passed
  1  One or more assertion failures
  2  One or more test errors (fixture load or runner failure)`,
		Action: func(c *cli.Context) error {
			root := "."
			if c.NArg() > 0 {
				root = c.Args().First()
			}
			return templateTestAction(c.Context, c.App.Writer, root)
		},
	}
}

func templateTestAction(ctx context.Context, w io.Writer, templateRoot string) error {
	report, err := templatetest.Run(ctx, templatetest.RunOptions{
		TemplateRoot: templateRoot,
	})
	if err != nil {
		return app.Errorf("test runner failed: %w", err)
	}

	printTestReport(w, report)

	switch report.ExitCode() {
	case 1:
		return &app.CommandError{
			Message: fmt.Sprintf("template test: %d failure(s)", report.Failed),
			Code:    app.ExitGeneral,
		}
	case 2:
		return &app.CommandError{
			Message: fmt.Sprintf("template test: %d error(s)", report.Errored),
			Code:    app.ExitUsage,
		}
	}
	return nil
}

func printTestReport(w io.Writer, report templatetest.Report) {
	for _, c := range report.Cases {
		switch c.Status {
		case templatetest.CasePassed:
			fmt.Fprintf(w, "  ✓  %s\n", c.Name)
		case templatetest.CaseFailed:
			fmt.Fprintf(w, "  ✗  %s\n", c.Name)
			for _, ar := range c.Assertions {
				if !ar.Passed {
					fmt.Fprintf(w, "       FAIL: %s\n", ar.Detail)
				}
			}
		case templatetest.CaseErrored:
			fmt.Fprintf(w, "  !  %s — %s\n", c.Name, c.Error)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Results: %d passed, %d failed, %d errored\n",
		report.Passed, report.Failed, report.Errored)

	if len(report.Cases) == 0 {
		fmt.Fprintln(w, "No test fixtures found.")
		return
	}

	if report.ExitCode() == 0 {
		fmt.Fprintln(w, "All tests passed.")
	}
}

package commands

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/testrunner"
	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/pkg/app"
)

// TestCommand returns the top-level "test" command for matrix testing.
func TestCommand() *cli.Command {
	return &cli.Command{
		Name:      "test",
		Usage:     "Test all boolean variable combinations produce valid output",
		ArgsUsage: "[template-dir]",
		Description: `Discovers boolean variables in tag.template.json, generates all 2^N
combinations, scaffolds each one, and runs validation commands.

Validation commands can be defined in tag.template.json:

  {
    "test": {
      "commands": ["go build ./...", "go vet ./..."],
      "env": { "CGO_ENABLED": "0" }
    }
  }

Or overridden via --run flags. If no commands are configured, only
scaffold success is verified.

EXAMPLES
  tag test                                  # test current directory
  tag test ./my-template -m module=foo      # with required string vars
  tag test . --pin use_s3=false             # fix a var, skip permutation
  tag test . --dry-run                      # list combinations only
  tag test . --filter use_postgres=true     # only combos with postgres
  tag test . --fail-fast --verbose          # stop early, show output

EXIT CODES
  0  All combinations passed
  1  One or more combinations failed
  2  One or more errors (config, setup)`,
		Flags: testFlags(),
		Action: func(c *cli.Context) error {
			root := "."
			if c.NArg() > 0 {
				root = c.Args().First()
			}
			return testAction(c, c.App.Writer, root)
		},
	}
}

func testFlags() []cli.Flag {
	return []cli.Flag{
		&cli.IntFlag{
			Name:    "parallel",
			Aliases: []string{"p"},
			Value:   4,
			Usage:   "Max concurrent test runs",
		},
		&cli.StringSliceFlag{
			Name:    "meta",
			Aliases: []string{"m"},
			Usage:   "Required variable override in key=value format (can be repeated)",
		},
		&cli.StringFlag{
			Name:  "values",
			Usage: "JSON file with variable values",
		},
		&cli.StringSliceFlag{
			Name:  "skip-var",
			Usage: "Exclude boolean var from permutation (use its default)",
		},
		&cli.StringSliceFlag{
			Name:  "pin",
			Usage: "Fix a variable to a specific value, e.g. --pin use_s3=false",
		},
		&cli.StringSliceFlag{
			Name:  "run",
			Usage: "Validation command (overrides template config, can be repeated)",
		},
		&cli.StringFlag{
			Name:  "filter",
			Usage: "Filter combinations by index or key=value pairs (comma-separated)",
		},
		&cli.BoolFlag{
			Name:  "fail-fast",
			Usage: "Stop on first failure",
		},
		&cli.BoolFlag{
			Name:  "dry-run",
			Usage: "List combinations without running tests",
		},
		&cli.BoolFlag{
			Name:  "keep-failed",
			Usage: "Keep scaffolded directories on failure for debugging",
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Value: 5 * time.Minute,
			Usage: "Per-command timeout",
		},
		&cli.IntFlag{
			Name:  "max-cases",
			Value: 64,
			Usage: "Safety limit for total combinations (0 = unlimited)",
		},
		&cli.StringFlag{
			Name:  "format",
			Value: "text",
			Usage: "Output format: text or json",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Show full command output on failures",
		},
		&cli.BoolFlag{
			Name:  "accept-hooks",
			Usage: "Accept and run pre/post scaffold hooks during testing",
		},
	}
}

func testAction(c *cli.Context, w io.Writer, templateDir string) error {
	pinVars, err := parseKeyValueSlice(c.StringSlice("pin"))
	if err != nil {
		return app.Errorf("invalid --pin value: %w", err)
	}

	meta, err := parseKeyValueSlice(c.StringSlice("meta"))
	if err != nil {
		return app.Errorf("invalid --meta value: %w", err)
	}

	cfg := testrunner.Config{
		TemplateDir: templateDir,
		Meta:        meta,
		ValuesFile:  c.String("values"),
		SkipVars:    c.StringSlice("skip-var"),
		PinVars:     pinVars,
		RunCommands: c.StringSlice("run"),
		Filter:      c.String("filter"),
		Parallel:    c.Int("parallel"),
		FailFast:    c.Bool("fail-fast"),
		DryRun:      c.Bool("dry-run"),
		KeepFailed:  c.Bool("keep-failed"),
		Timeout:     c.Duration("timeout"),
		MaxCases:    c.Int("max-cases"),
		Verbose:     c.Bool("verbose"),
		AcceptHooks: c.Bool("accept-hooks"),
		Format:      c.String("format"),
	}

	// Load template config to extract boolean var names for display.
	tmplCfg, err := loadTestTemplateConfig(templateDir)
	if err != nil {
		return err
	}
	boolVars := testrunner.ExtractBooleanVars(tmplCfg, cfg.SkipVars)

	// Header.
	combos := testrunner.GenerateCombinations(boolVars, pinVars)
	combos = testrunner.FilterCombinations(combos, cfg.Filter)
	fmt.Fprintf(w, "Template: %s\n", templateDir)
	fmt.Fprintf(w, "Boolean variables: %v\n", boolVars)
	fmt.Fprintf(w, "Combinations: %d | Parallel: %d\n\n", len(combos), cfg.Parallel)

	if cfg.DryRun {
		testrunner.PrintDryRun(w, combos, boolVars)
		return nil
	}

	report, err := testrunner.Run(c.Context, cfg)
	if err != nil {
		return app.Errorf("test runner: %w", err)
	}

	switch cfg.Format {
	case "json":
		if jsonErr := testrunner.PrintJSONReport(w, report); jsonErr != nil {
			return app.Errorf("write JSON report: %w", jsonErr)
		}
	default:
		testrunner.PrintTextReport(w, report, boolVars, cfg.Verbose)
	}

	switch report.ExitCode() {
	case 1:
		return &app.CommandError{
			Message: fmt.Sprintf("matrix test: %d failure(s)", report.Failed),
			Code:    app.ExitGeneral,
		}
	case 2:
		return &app.CommandError{
			Message: fmt.Sprintf("matrix test: %d error(s)", report.Errored),
			Code:    app.ExitUsage,
		}
	}
	return nil
}

func loadTestTemplateConfig(templateDir string) (*tmplconfig.TemplateConfig, error) {
	configPath := templateDir + "/tag.template.json"
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, app.Errorf("read template config: %w", err)
	}
	cfg, err := tmplconfig.ParseTemplateConfig(data)
	if err != nil {
		return nil, app.Errorf("parse template config: %w", err)
	}
	return cfg, nil
}

func parseKeyValueSlice(pairs []string) (map[string]string, error) {
	result := make(map[string]string, len(pairs))
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return nil, fmt.Errorf("expected key=value format, got %q", p)
		}
		result[k] = v
	}
	return result, nil
}

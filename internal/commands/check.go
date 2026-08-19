package commands

import (
	"fmt"
	"io"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/jsonout"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/templateupdate"
	"github.com/kaikenlabs/tag/pkg/app"
)

// CheckCommand returns the check command definition.
func CheckCommand() *cli.Command {
	return &cli.Command{
		Name:  "check",
		Usage: "Check if upstream template has changed since last update",
		Description: `Checks whether the upstream template has newer commits than the
version recorded in .tagconfig.json. Useful in CI pipelines to detect template drift.

Exit codes:
  0  Project is up to date
  1  Updates are available (or error occurred)

Examples:
  # Check current project
  tag check

  # Check with no output (CI mode)
  tag check --quiet

  # Check against a specific branch
  tag check --ref main`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "dir",
				Value: ".",
				Usage: "Project directory to check",
			},
			&cli.StringFlag{
				Name:  "ref",
				Usage: "Override the template ref to check against",
			},
			&cli.BoolFlag{
				Name:  "quiet",
				Usage: "Suppress output (including JSON), only set exit code",
			},
			formatFlag(formatText, formatJSON),
		},
		Action: checkAction,
	}
}

func checkAction(c *cli.Context) error {
	format, err := resolveFormat(c, formatText, formatJSON)
	if err != nil {
		return err
	}

	resolver := newGitResolver()
	checker := templateupdate.NewChecker(resolver)
	result, err := checker.Check(c.Context, templateupdate.CheckOptions{
		ProjectDir: c.String("dir"),
		Ref:        c.String("ref"),
	})
	if err != nil {
		return app.Errorf("check: %w", err)
	}

	if !c.Bool("quiet") {
		if writeErr := writeCheckResult(cmdOut(c), format, result); writeErr != nil {
			return app.Errorf("write check result: %w", writeErr)
		}
	}

	if result.UpToDate {
		return nil
	}
	return cli.Exit("", 1)
}

// writeCheckResult renders result in the requested format to w.
func writeCheckResult(w io.Writer, format string, result *templateupdate.CheckResult) error {
	if format == formatJSON {
		return jsonout.Write(w, result)
	}

	if result.UpToDate {
		fmt.Fprintf(w, "✓ Project is up to date with template (commit %s)\n", shortCommitSHA(result.CurrentSHA))
		return nil
	}

	fmt.Fprintf(w, "✗ Template updates available\n")
	fmt.Fprintf(w, "  Template: %s\n", result.Source)
	fmt.Fprintf(w, "  Current:  %s\n", shortCommitSHA(result.CurrentSHA))
	fmt.Fprintf(w, "  Latest:   %s\n", shortCommitSHA(result.LatestSHA))
	fmt.Fprintf(w, "  Run 'tag diff' to see changes, 'tag update' to apply.\n")
	return nil
}

// newGitResolver creates a GitFetcher suitable for commit resolution.
// Package-level variable so tests can substitute a stub resolver.
var newGitResolver = defaultGitResolver

func defaultGitResolver() remote.LatestCommitResolver {
	auth := remote.NewEnvAuthProvider()
	return remote.NewGitFetcher(auth)
}

// shortCommitSHA returns first 7 chars of a SHA.
func shortCommitSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

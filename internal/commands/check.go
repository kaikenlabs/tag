package commands

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

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
				Usage: "Suppress output, only set exit code",
			},
		},
		Action: checkAction,
	}
}

func checkAction(c *cli.Context) error {
	resolver := newGitResolver()
	checker := templateupdate.NewChecker(resolver)
	result, err := checker.Check(c.Context, templateupdate.CheckOptions{
		ProjectDir: c.String("dir"),
		Ref:        c.String("ref"),
	})
	if err != nil {
		return app.Errorf("check: %w", err)
	}

	quiet := c.Bool("quiet")

	if result.UpToDate {
		if !quiet {
			fmt.Fprintf(os.Stdout, "✓ Project is up to date with template (commit %s)\n", shortCommitSHA(result.CurrentSHA))
		}
		return nil
	}

	// Updates available — exit 1.
	if !quiet {
		fmt.Fprintf(os.Stdout, "✗ Template updates available\n")
		fmt.Fprintf(os.Stdout, "  Template: %s\n", result.Source)
		fmt.Fprintf(os.Stdout, "  Current:  %s\n", shortCommitSHA(result.CurrentSHA))
		fmt.Fprintf(os.Stdout, "  Latest:   %s\n", shortCommitSHA(result.LatestSHA))
		fmt.Fprintf(os.Stdout, "  Run 'tag diff' to see changes, 'tag update' to apply.\n")
	}

	return cli.Exit("", 1)
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

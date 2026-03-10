package commands

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/templateupdate"
	"github.com/kaikenlabs/tag/pkg/app"
)

// DiffCommand returns the diff command definition.
func DiffCommand() *cli.Command {
	return &cli.Command{
		Name:  "diff",
		Usage: "Show what would change if you ran 'tag update'",
		Description: `Performs a dry-run 3-way merge and displays a diff of proposed changes.
This is a read-only operation with no side effects.

Examples:
  # Show full diff
  tag diff

  # Show compact summary
  tag diff --stat

  # Pipe-friendly output (no colors)
  tag diff --no-color

  # Check against a specific ref
  tag diff --ref v2.0.0`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "dir",
				Value: ".",
				Usage: "Project directory",
			},
			&cli.StringFlag{
				Name:  "ref",
				Usage: "Override template ref",
			},
			&cli.BoolFlag{
				Name:  "stat",
				Usage: "Show diffstat summary instead of full diff",
			},
			&cli.BoolFlag{
				Name:  "no-color",
				Usage: "Disable color output",
			},
		},
		Action: diffAction,
	}
}

func diffAction(c *cli.Context) error {
	resolver := newGitResolver()
	auth := remote.NewEnvAuthProvider()
	fetcher := remote.NewGitFetcher(auth)
	renderer := templateupdate.NewHistoricalRenderer(fetcher)

	differ := templateupdate.NewDiffer(renderer, resolver)
	result, err := differ.Diff(c.Context, templateupdate.DiffOptions{
		ProjectDir: c.String("dir"),
		Ref:        c.String("ref"),
	})
	if err != nil {
		return app.Errorf("diff: %w", err)
	}

	// No changes.
	if result.OldSHA == result.NewSHA {
		fmt.Fprintln(os.Stdout, "Already up to date.")
		return nil
	}

	// Determine color output.
	useColor := !c.Bool("no-color") && isStdoutTTY()

	templateupdate.FormatDiff(result.Results, result.Source, result.OldSHA, result.NewSHA, templateupdate.FormatOptions{
		Color:  useColor,
		Stat:   c.Bool("stat"),
		Writer: os.Stdout,
	})

	return nil
}

// isStdoutTTY checks if stdout is a terminal.
func isStdoutTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd())) //nolint:gosec // Fd() returns a valid file descriptor
}

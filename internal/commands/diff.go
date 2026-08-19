package commands

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	"github.com/kaikenlabs/tag/internal/jsonout"
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
			formatFlag(formatText, formatJSON),
		},
		Action: diffAction,
	}
}

func diffAction(c *cli.Context) error {
	// diff takes no positionals. It cannot use reparseTrailingFlags (there is
	// nothing to reparse INTO), so a stray token must be rejected directly —
	// otherwise `tag diff stray --format json` silently prints text, because
	// urfave/cli stops parsing at the first non-flag token ("stray") and
	// --format never reaches the flag parser at all.
	if c.NArg() > 0 {
		return app.UsageErrorf("tag diff does not accept positional arguments (got %q)", c.Args().First())
	}

	format, err := resolveFormat(c, formatText, formatJSON)
	if err != nil {
		return err
	}

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

	out := cmdOut(c)

	// JSON is written unconditionally, even when up to date: a script must
	// never receive an empty body with exit 0. Presentation flags (--stat,
	// --no-color) are accepted but ignored here, since they are read only in
	// the text branch below.
	if format == formatJSON {
		if writeErr := jsonout.Write(out, templateupdate.Summarize(result)); writeErr != nil {
			return app.Errorf("write json: %w", writeErr)
		}
		return nil
	}

	// No changes — text keeps this exact sentinel unchanged.
	if result.OldSHA == result.NewSHA {
		fmt.Fprintln(out, "Already up to date.")
		return nil
	}

	// Determine color output.
	useColor := !c.Bool("no-color") && isStdoutTTY()

	templateupdate.FormatDiff(result.Results, result.Source, result.OldSHA, result.NewSHA, templateupdate.FormatOptions{
		Color:  useColor,
		Stat:   c.Bool("stat"),
		Writer: out,
	})

	return nil
}

// isStdoutTTY checks if stdout is a terminal.
func isStdoutTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd())) //nolint:gosec // Fd() returns a valid file descriptor
}

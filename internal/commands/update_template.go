package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/templateupdate"
	"github.com/kaikenlabs/tag/pkg/app"
)

// UpdateTemplateCommand returns the template update command definition.
func UpdateTemplateCommand() *cli.Command {
	return &cli.Command{
		Name:    "update",
		Aliases: []string{"up"},
		Usage:   "Apply upstream template changes to your project",
		Description: `Applies upstream template changes using 3-way merge. Detects what
changed in the template since you scaffolded (or last updated), compares with
your local modifications, and merges them together.

Examples:
  # Apply latest template changes
  tag update

  # Preview without applying
  tag update --dry-run

  # Auto-resolve conflicts with your version
  tag update --accept-ours

  # Auto-resolve conflicts with template version
  tag update --accept-theirs

  # Override variables during update
  tag update --set author="Jane Doe" --set license=MIT

  # Resume after resolving conflicts manually
  tag update --continue

  # Abort and restore from backup
  tag update --abort`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "dir",
				Value: ".",
				Usage: "Project directory",
			},
			&cli.StringFlag{
				Name:  "ref",
				Usage: "Override template ref to update to",
			},
			&cli.StringSliceFlag{
				Name:  "set",
				Usage: "Override/add variable values (key=value)",
			},
			&cli.BoolFlag{
				Name:  "accept-ours",
				Usage: "Auto-resolve conflicts with your version",
			},
			&cli.BoolFlag{
				Name:  "accept-theirs",
				Usage: "Auto-resolve conflicts with template version",
			},
			&cli.StringSliceFlag{
				Name:  "skip",
				Usage: "Additional skip patterns for this run",
			},
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Show what would change without applying",
			},
			&cli.BoolFlag{
				Name:  "backup",
				Value: true,
				Usage: "Create backup before applying changes",
			},
			&cli.BoolFlag{
				Name:  "continue",
				Usage: "Resume after manual conflict resolution",
			},
			&cli.BoolFlag{
				Name:  "abort",
				Usage: "Abort in-progress update, restore backup",
			},
		},
		Action: updateTemplateAction,
	}
}

func updateTemplateAction(c *cli.Context) error {
	// Validate mutually exclusive flags.
	if c.Bool("continue") && c.Bool("abort") {
		return app.Errorf("cannot use --continue and --abort together")
	}
	if c.Bool("accept-ours") && c.Bool("accept-theirs") {
		return app.Errorf("cannot use --accept-ours and --accept-theirs together")
	}

	// Determine update mode.
	mode := templateupdate.UpdateModeApply
	if c.Bool("continue") {
		mode = templateupdate.UpdateModeContinue
	} else if c.Bool("abort") {
		mode = templateupdate.UpdateModeAbort
	}

	// Determine resolve mode.
	resolveMode := templateupdate.ResolveNone
	if c.Bool("accept-ours") {
		resolveMode = templateupdate.ResolveOurs
	} else if c.Bool("accept-theirs") {
		resolveMode = templateupdate.ResolveTheirs
	}

	// Parse --set overrides using the same key=value parser as --meta.
	varOverrides, err := parseSetFlags(c.StringSlice("set"))
	if err != nil {
		return err
	}

	resolver := newGitResolver()
	auth := remote.NewEnvAuthProvider()
	fetcher := remote.NewGitFetcher(auth)
	renderer := templateupdate.NewHistoricalRenderer(fetcher)

	updater := templateupdate.NewUpdater(renderer, resolver)
	result, err := updater.Update(c.Context, templateupdate.UpdateOptions{
		ProjectDir:   c.String("dir"),
		Ref:          c.String("ref"),
		VarOverrides: varOverrides,
		ResolveMode:  resolveMode,
		SkipPatterns: c.StringSlice("skip"),
		DryRun:       c.Bool("dry-run"),
		Backup:       c.Bool("backup"),
		Mode:         mode,
	})
	if err != nil {
		return app.Errorf("update: %w", err)
	}

	// Handle abort.
	if mode == templateupdate.UpdateModeAbort {
		fmt.Fprintln(os.Stdout, "Update aborted. Backup restored.")
		return nil
	}

	// Handle continue.
	if mode == templateupdate.UpdateModeContinue {
		fmt.Fprintln(os.Stdout, "All conflicts resolved.")
		fmt.Fprintf(os.Stdout, "  ✓ .tagconfig.json updated (commit: %s)\n", shortCommitSHA(result.NewSHA))
		return nil
	}

	// No changes needed.
	if result.OldSHA == result.NewSHA {
		fmt.Fprintln(os.Stdout, "Already up to date.")
		return nil
	}

	// Print results.
	if c.Bool("dry-run") {
		fmt.Fprintf(os.Stdout, "Would update from %s → %s\n", shortCommitSHA(result.OldSHA), shortCommitSHA(result.NewSHA))
	} else {
		fmt.Fprintf(os.Stdout, "Updating from template (%s → %s)\n", shortCommitSHA(result.OldSHA), shortCommitSHA(result.NewSHA))
	}

	printUpdateSummary(result)

	if result.Conflicts != nil && result.Conflicts.HasConflicts() {
		fmt.Fprintf(os.Stdout, "\n⚠ %d conflict(s) found. Resolve manually, then run: tag update --continue\n", len(result.Conflicts.Conflicts))
		fmt.Fprintln(os.Stdout, "  Or: tag update --accept-ours | --accept-theirs")
		return cli.Exit("", 1)
	}

	if !c.Bool("dry-run") {
		fmt.Fprintf(os.Stdout, "\nTemplate update complete. %d file(s) updated, %d added, %d deleted.\n",
			result.UpdatedFiles, result.NewFiles, result.DeletedFiles)
	}

	return nil
}

// parseSetFlags parses key=value pairs from --set flags.
func parseSetFlags(args []string) (map[string]string, error) {
	result := make(map[string]string, len(args))
	for _, kv := range args {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 { //nolint:mnd // key=value requires exactly 2 parts
			return nil, app.Errorf("invalid --set value %q: expected key=value format", kv)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

// printUpdateSummary prints the file-by-file status.
func printUpdateSummary(result *templateupdate.UpdateResult) {
	for _, r := range result.Applied {
		switch r.Op {
		case templateupdate.MergeAdd:
			fmt.Fprintf(os.Stdout, "  + %s (added)\n", r.Path)
		case templateupdate.MergeUpdate:
			fmt.Fprintf(os.Stdout, "  ✓ %s (updated)\n", r.Path)
		case templateupdate.MergeDelete:
			fmt.Fprintf(os.Stdout, "  - %s (deleted)\n", r.Path)
		case templateupdate.MergeConflict:
			fmt.Fprintf(os.Stdout, "  ⚠ %s (conflict)\n", r.Path)
		}
	}
}

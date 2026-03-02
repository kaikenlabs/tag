package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/history"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/pkg/app"
)

// UndoCommand returns the `tag undo` command definition.
func UndoCommand() *cli.Command {
	return &cli.Command{
		Name:  "undo",
		Usage: "Revert a previous generation",
		Description: `Revert files created or modified by a previous tag generate or tag scaffold.

By default, reverts the most recent generation. Use --id to target a specific
generation and --list to view the full history.

Conflict detection: if a file was modified after the generation was recorded,
undo refuses to overwrite it. Use --force to override, or --partial to revert
only the unmodified files.

EXAMPLES
  tag undo                           Undo the last generation
  tag undo --list                    Show generation history
  tag undo --id gen_1741000000_a3f2bc  Undo a specific generation
  tag undo --yes                     Skip confirmation prompt
  tag undo --force                   Override conflict detection
  tag undo --partial                 Revert unmodified files, skip conflicts`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "id",
				Usage:   "Generation ID to undo (default: last generation)",
				Aliases: []string{"i"},
			},
			&cli.BoolFlag{
				Name:  "list",
				Usage: "List all recorded generations and exit",
			},
			&cli.BoolFlag{
				Name:    "yes",
				Aliases: []string{"y"},
				Usage:   "Skip confirmation prompt",
			},
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Override conflict detection and revert even if files were modified",
			},
			&cli.BoolFlag{
				Name:  "partial",
				Usage: "Revert unmodified files and skip conflicting ones instead of aborting",
			},
		},
		Action: undoAction,
	}
}

func undoAction(c *cli.Context) error {
	tagDir, err := resolveTagDir()
	if err != nil {
		return err
	}

	if c.Bool("list") {
		return listGenerations(tagDir, c)
	}

	return runUndo(tagDir, c)
}

func listGenerations(tagDir string, c *cli.Context) error {
	gens, err := history.ListGenerations(tagDir)
	if err != nil {
		return app.Errorf("cannot load history: %w", err)
	}

	if len(gens) == 0 {
		fmt.Fprintln(c.App.Writer, "No generations recorded.")
		return nil
	}

	fmt.Fprintln(c.App.Writer, "Generation history (newest first):")
	for _, g := range gens {
		name := g.Template
		if name == "" {
			name = g.Command
		}
		fmt.Fprintf(c.App.Writer, "  %s  %s  %-20s  %d file(s)\n",
			g.ID,
			g.Timestamp.Format("2006-01-02 15:04:05"),
			name,
			len(g.Files),
		)
	}
	return nil
}

func runUndo(tagDir string, c *cli.Context) error {
	genID := c.String("id")
	force := c.Bool("force")
	partial := c.Bool("partial")
	yes := c.Bool("yes")

	// Load manifest to determine what will be reverted so we can show a preview.
	m, err := history.Load(tagDir)
	if err != nil {
		return app.Errorf("cannot load history: %w", err)
	}

	// Find the target generation.
	var targetGen *history.Generation
	if genID == "" {
		if len(m.Generations) == 0 {
			return app.Errorf("no generations to undo")
		}
		g := m.Generations[len(m.Generations)-1]
		targetGen = &g
	} else {
		for i := range m.Generations {
			if m.Generations[i].ID == genID {
				g := m.Generations[i]
				targetGen = &g
				break
			}
		}
		if targetGen == nil {
			return app.Errorf("generation %q not found", genID)
		}
	}

	if !yes {
		name := targetGen.Template
		if name == "" {
			name = targetGen.Command
		}
		fmt.Fprintf(c.App.Writer, "About to undo generation %s (%s) — %d file(s)\n",
			targetGen.ID, name, len(targetGen.Files))
		for _, f := range targetGen.Files {
			fmt.Fprintf(c.App.Writer, "  [%s] %s\n", f.Action, f.Path)
		}
		fmt.Fprintln(c.App.Writer)

		if !promptConfirm(c) {
			fmt.Fprintln(c.App.Writer, "Undo cancelled.")
			return nil
		}
	}

	// Always pass the resolved ID so history.Undo targets exactly the
	// generation shown in the preview, eliminating any TOCTOU window.
	opts := history.UndoOptions{
		GenID:   targetGen.ID,
		Force:   force,
		Partial: partial,
		Out:     c.App.Writer,
	}

	if err := history.Undo(tagDir, opts); err != nil {
		var conflictErr *history.ConflictError
		if errors.As(err, &conflictErr) {
			fmt.Fprintln(c.App.Writer, "Conflict: the following files were modified after generation:")
			for _, p := range conflictErr.Paths {
				fmt.Fprintf(c.App.Writer, "  - %s\n", p)
			}
			fmt.Fprintln(c.App.Writer, "\nUse --force to override, or --partial to revert unmodified files only.")
			return app.Errorf("undo aborted due to conflicts")
		}
		return app.Errorf("undo failed: %w", err)
	}

	return nil
}

// resolveTagDir returns the absolute path to the .tag/ directory for the
// current project by walking up from the current working directory.
func resolveTagDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", app.Errorf("cannot get working directory: %w", err)
	}

	dir := cwd
	for {
		candidate := filepath.Join(dir, types.TemplatesDir)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fallback: use the .tag/ directory relative to cwd (will be created if needed).
	return filepath.Join(cwd, types.TemplatesDir), nil
}

// promptConfirm asks the user for a yes/no answer on stdin. Falls back to
// false if not a TTY (non-interactive mode).
func promptConfirm(c *cli.Context) bool {
	if !isTerminal() {
		fmt.Fprintln(c.App.Writer, "Non-interactive mode: use --yes to confirm undo automatically.")
		return false
	}

	fmt.Fprint(c.App.Writer, "Proceed? [y/N] ")
	var response string
	if _, err := fmt.Fscan(os.Stdin, &response); err != nil {
		return false
	}
	return strings.ToLower(strings.TrimSpace(response)) == "y"
}

// isTerminal returns true if stdin is connected to a terminal (interactive mode).
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/fileaction"
	"github.com/kaikenlabs/tag/internal/history"
	"github.com/kaikenlabs/tag/internal/jsonout"
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
		Flags:  undoFlags(),
		Action: undoAction,
	}
}

// undoFlags is shared between the command definition and
// reparseTrailingFlags, so a trailing --format is recognised.
func undoFlags() []cli.Flag {
	return []cli.Flag{
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
		formatFlag(formatText, formatJSON),
	}
}

func undoAction(c *cli.Context) error {
	if _, err := reparseTrailingFlags(c, undoFlags()); err != nil {
		return err
	}

	format, err := resolveFormat(c, formatText, formatJSON)
	if err != nil {
		return err
	}
	jsonMode := format == formatJSON

	tagDir, err := resolveTagDir()
	if err != nil {
		return err
	}

	if c.Bool("list") {
		return listGenerations(tagDir, c, jsonMode)
	}

	return runUndo(tagDir, c, jsonMode)
}

func listGenerations(tagDir string, c *cli.Context, jsonMode bool) error {
	gens, err := history.ListGenerations(tagDir)
	if err != nil {
		return app.Errorf("cannot load history: %w", err)
	}

	if jsonMode {
		return jsonout.Write(cmdOut(c), newUndoListDoc(gens))
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

func runUndo(tagDir string, c *cli.Context, jsonMode bool) error {
	genID := c.String("id")
	force := c.Bool("force")
	partial := c.Bool("partial")
	yes := c.Bool("yes")

	// D2: JSON mode must never imply consent for a destructive operation, and
	// auto-confirming would be a silent behaviour change relative to text
	// mode — so --yes is mandatory rather than assumed.
	if jsonMode && !yes {
		return app.UsageErrorf("--format json requires --yes")
	}

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

	// undoOut receives the human-readable text history.Undo writes through
	// (the conflict warning under --partial, and the success summary). In
	// JSON mode it is rerouted to c.App.ErrWriter so it stays visible to a
	// human without corrupting the document on stdout.
	undoOut := c.App.Writer
	if jsonMode {
		undoOut = cmdErr(c)
	}

	// Always pass the resolved ID so history.Undo targets exactly the
	// generation shown in the preview, eliminating any TOCTOU window.
	opts := history.UndoOptions{
		GenID:   targetGen.ID,
		Force:   force,
		Partial: partial,
		Out:     undoOut,
	}

	result, err := history.Undo(tagDir, opts)
	if err != nil {
		var conflictErr *history.ConflictError
		if errors.As(err, &conflictErr) {
			return reportUndoConflict(c, jsonMode, targetGen.ID, conflictErr)
		}
		return app.Errorf("undo failed: %w", err)
	}

	if jsonMode {
		return jsonout.Write(cmdOut(c), newUndoDoc(result.GenID, result.Files, result.Reverted, result.Skipped, result.Conflicts))
	}

	return nil
}

// reportUndoConflict renders a history.ConflictError in the requested
// format. In JSON mode it writes a document with conflicts populated (D5),
// then returns the same error text as the text branch so the exit code
// matches either way.
func reportUndoConflict(c *cli.Context, jsonMode bool, genID string, conflictErr *history.ConflictError) error {
	if jsonMode {
		if writeErr := jsonout.Write(cmdOut(c), newUndoDoc(genID, nil, 0, 0, conflictErr.Paths)); writeErr != nil {
			return writeErr
		}
		return app.Errorf("undo aborted due to conflicts")
	}

	fmt.Fprintln(c.App.Writer, "Conflict: the following files were modified after generation:")
	for _, p := range conflictErr.Paths {
		fmt.Fprintf(c.App.Writer, "  - %s\n", p)
	}
	fmt.Fprintln(c.App.Writer, "\nUse --force to override, or --partial to revert unmodified files only.")
	return app.Errorf("undo aborted due to conflicts")
}

// undoGenerationJSON is one entry of `undo --list --format json`.
type undoGenerationJSON struct {
	ID       string `json:"id"`
	Template string `json:"template,omitempty"`
	Command  string `json:"command"`
	Files    int    `json:"files"`
}

// undoListDoc is the JSON shape for `undo --list --format json` (D7): a
// --format flag that silently ignores a sibling flag of the same command is
// a bug, so --list is not left out of the JSON surface.
type undoListDoc struct {
	Generations []undoGenerationJSON `json:"generations"`
}

func newUndoListDoc(gens []history.Generation) undoListDoc {
	out := make([]undoGenerationJSON, 0, len(gens))
	for _, g := range gens {
		out = append(out, undoGenerationJSON{
			ID:       g.ID,
			Template: g.Template,
			Command:  g.Command,
			Files:    len(g.Files),
		})
	}
	return undoListDoc{Generations: out}
}

// undoFileJSON is one entry of an undoDoc's "files" list.
type undoFileJSON struct {
	Path     string            `json:"path"`
	Action   fileaction.Action `json:"action"`
	Reverted bool              `json:"reverted"`
}

// undoDoc is the JSON shape for `undo --format json`. On a hard conflict
// abort (no --partial/--force) files is empty and reverted/skipped are zero:
// nothing was touched, only conflicts is populated.
type undoDoc struct {
	GenID     string         `json:"gen_id"`
	Files     []undoFileJSON `json:"files"`
	Reverted  int            `json:"reverted"`
	Skipped   int            `json:"skipped"`
	Conflicts []string       `json:"conflicts,omitempty"`
}

func newUndoDoc(genID string, files []history.UndoFileResult, reverted, skipped int, conflicts []string) undoDoc {
	out := make([]undoFileJSON, 0, len(files))
	for _, f := range files {
		out = append(out, undoFileJSON{Path: f.Path, Action: f.Action, Reverted: f.Reverted})
	}

	var conflictsOut []string
	if len(conflicts) > 0 {
		conflictsOut = make([]string, len(conflicts))
		copy(conflictsOut, conflicts)
	}

	return undoDoc{
		GenID:     genID,
		Files:     out,
		Reverted:  reverted,
		Skipped:   skipped,
		Conflicts: conflictsOut,
	}
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

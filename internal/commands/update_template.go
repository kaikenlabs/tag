package commands

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/jsonout"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/templateupdate"
	"github.com/kaikenlabs/tag/pkg/app"
)

// templateUpdater is the minimal surface updateTemplateAction needs from
// *templateupdate.Updater. It is substitutable in tests (precedent:
// newGitResolver, newLocalLibrary): the concrete constructor chain
// (env auth provider -> GitFetcher -> HistoricalRenderer -> Updater) needs a
// real git remote, so without this seam the entire success path of `update`
// is untestable.
type templateUpdater interface {
	Update(ctx context.Context, opts templateupdate.UpdateOptions) (*templateupdate.UpdateResult, error)
}

// newTemplateUpdater builds the production updater chain. Tests substitute
// this package-level var to inject a fake without a real git remote.
var newTemplateUpdater = func() templateUpdater {
	resolver := newGitResolver()
	auth := remote.NewEnvAuthProvider()
	fetcher := remote.NewGitFetcher(auth)
	renderer := templateupdate.NewHistoricalRenderer(fetcher)
	return templateupdate.NewUpdater(renderer, resolver)
}

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
			&cli.BoolFlag{
				Name:  "skip-hooks",
				Usage: "Skip all hook execution during update",
			},
			&cli.BoolFlag{
				Name:  "accept-hooks",
				Usage: "Run changed hooks without prompting",
			},
			formatFlag(formatText, formatJSON),
		},
		Action: updateTemplateAction,
	}
}

func updateTemplateAction(c *cli.Context) error {
	// update takes no positionals. It cannot use reparseTrailingFlags (there
	// is nothing to reparse INTO), so a stray token must be rejected
	// directly — otherwise `tag update stray --format json` would silently
	// print text, since urfave/cli stops parsing at the first non-flag token
	// and --format never reaches the flag parser at all. (Same pattern as
	// diffAction.)
	if c.NArg() > 0 {
		return app.UsageErrorf("tag update does not accept positional arguments (got %q)", c.Args().First())
	}

	format, err := resolveFormat(c, formatText, formatJSON)
	if err != nil {
		return err
	}
	jsonMode := format == formatJSON

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

	dryRun := c.Bool("dry-run")

	updater := newTemplateUpdater()
	result, err := updater.Update(c.Context, templateupdate.UpdateOptions{
		ProjectDir:   c.String("dir"),
		Ref:          c.String("ref"),
		VarOverrides: varOverrides,
		ResolveMode:  resolveMode,
		SkipPatterns: c.StringSlice("skip"),
		DryRun:       dryRun,
		Backup:       c.Bool("backup"),
		SkipHooks:    c.Bool("skip-hooks"),
		AcceptHooks:  c.Bool("accept-hooks"),
		Mode:         mode,
	})
	if err != nil {
		return app.Errorf("update: %w", err)
	}

	out := cmdOut(c)

	if jsonMode {
		return writeUpdateJSON(out, mode, dryRun, result)
	}

	// Handle abort.
	if mode == templateupdate.UpdateModeAbort {
		fmt.Fprintln(out, "Update aborted. Backup restored.")
		return nil
	}

	// Handle continue.
	if mode == templateupdate.UpdateModeContinue {
		fmt.Fprintln(out, "All conflicts resolved.")
		fmt.Fprintf(out, "  ✓ .tagconfig.json updated (commit: %s)\n", shortCommitSHA(result.NewSHA))
		return nil
	}

	// No changes needed.
	if result.OldSHA == result.NewSHA {
		fmt.Fprintln(out, "Already up to date.")
		return nil
	}

	// Print results.
	if dryRun {
		fmt.Fprintf(out, "Would update from %s → %s\n", shortCommitSHA(result.OldSHA), shortCommitSHA(result.NewSHA))
	} else {
		fmt.Fprintf(out, "Updating from template (%s → %s)\n", shortCommitSHA(result.OldSHA), shortCommitSHA(result.NewSHA))
	}

	if len(result.VarChanges) > 0 {
		fmt.Fprintln(out, "\nVariable changes:")
		lines := templateupdate.FormatVarChanges(result.VarChanges, nil)
		for _, line := range lines {
			fmt.Fprintln(out, line)
		}
		fmt.Fprintln(out)
	}

	if len(result.HookChanges) > 0 {
		fmt.Fprintln(out, "\nHook changes:")
		for _, line := range templateupdate.FormatHookChanges(result.HookChanges) {
			fmt.Fprintln(out, line)
		}
		fmt.Fprintln(out)
	}

	printUpdateSummary(out, result)

	if result.Conflicts != nil && result.Conflicts.HasConflicts() {
		fmt.Fprintf(out, "\n⚠ %d conflict(s) found. Resolve manually, then run: tag update --continue\n", len(result.Conflicts.Conflicts))
		fmt.Fprintln(out, "  Or: tag update --accept-ours | --accept-theirs")
		return cli.Exit("", 1)
	}

	if !dryRun {
		fmt.Fprintf(out, "\nTemplate update complete. %d file(s) updated, %d added, %d deleted.\n",
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

// printUpdateSummary prints the file-by-file status to w.
func printUpdateSummary(w io.Writer, result *templateupdate.UpdateResult) {
	for _, r := range result.Applied {
		switch r.Op {
		case templateupdate.MergeAdd:
			fmt.Fprintf(w, "  + %s (added)\n", r.Path)
		case templateupdate.MergeUpdate:
			fmt.Fprintf(w, "  ✓ %s (updated)\n", r.Path)
		case templateupdate.MergeDelete:
			fmt.Fprintf(w, "  - %s (deleted)\n", r.Path)
		case templateupdate.MergeConflict:
			fmt.Fprintf(w, "  ⚠ %s (conflict)\n", r.Path)
		}
	}
}

// writeUpdateJSON renders the JSON result of an update run. A conflict
// writes a document (with "conflicts" populated) AND the existing non-zero
// exit code, mirroring generate's D5 treatment of engine.ConflictError.
func writeUpdateJSON(w io.Writer, mode templateupdate.UpdateMode, dryRun bool, result *templateupdate.UpdateResult) error {
	doc := newUpdateDoc(mode, dryRun, result)
	if writeErr := jsonout.Write(w, doc); writeErr != nil {
		return writeErr
	}
	if result.Conflicts != nil && result.Conflicts.HasConflicts() {
		return cli.Exit("", 1)
	}
	return nil
}

// updateFileJSON is one entry of an updateDoc's "files" list. Op is
// MergeOp.String() verbatim (epic decision: update keeps templateupdate's own
// MergeOp vocabulary rather than mapping onto fileaction.Action, since a
// 3-way merge decision is a different concept from "TAG wrote this file this
// way" — see internal/fileaction's package doc).
type updateFileJSON struct {
	Path string `json:"path"`
	Op   string `json:"op"`
	// Conflicted is not redundant with Op: NewConflictReport classifies a file
	// as conflicted when `mr.Op == MergeConflict || mr.Conflicted`, so the flag
	// carries information Op alone does not.
	Conflicted bool `json:"conflicted"`
	// IsBinary tells a consumer the file cannot be diffed or merged as text.
	IsBinary bool `json:"is_binary"`
	// PromptReason is only set for MergePrompt, hence omitempty.
	//
	// These are the only MergeResult fields that are safe to serialise:
	// Content, BaseContent, OursContent and TheirsContent are unbounded copies
	// of the user's own source and must never reach stdout.
	PromptReason string `json:"prompt_reason,omitempty"`
}

// updateConflictJSON reuses the ConflictStatus tag names (conflicted_files,
// prompt_files) rather than embedding ConflictStatus itself, which would drag
// in schema_version and an unrelated started_at timestamp (D4). skipped is
// NOT a ConflictStatus tag — it is new to this DTO.
//
// Deliberately excluded: every ConflictedFile content byte slice
// (Base/Ours/Theirs/MergedContent) — the user's own source code, unbounded.
type updateConflictJSON struct {
	ConflictedFiles []string `json:"conflicted_files"`
	PromptFiles     []string `json:"prompt_files,omitempty"`
	Skipped         []string `json:"skipped,omitempty"`
}

// updateDoc is the JSON shape for `update --format json`. mode distinguishes
// apply/continue/abort, since abort and continue produce no file list: abort
// carries nothing beyond mode, continue carries only the SHAs.
type updateDoc struct {
	Mode   string `json:"mode"`
	DryRun bool   `json:"dry_run"`
	OldSHA string `json:"old_sha,omitempty"`
	NewSHA string `json:"new_sha,omitempty"`
	// No omitempty on the apply-mode payload below. These are only ever set
	// in apply mode (abort and continue return early with the minimal shape),
	// and within that mode a consumer must be able to read doc.files.length
	// and doc.updated_files unconditionally. With omitempty an apply run that
	// merged nothing dropped every one of these keys, which is strictly worse
	// than the `null` that #354's empty-array criterion exists to prevent:
	// absent reads as a TypeError, and "updated_files": 0 becomes
	// indistinguishable from "this field does not exist".
	UpToDate     bool                `json:"up_to_date"`
	Files        []updateFileJSON    `json:"files"`
	NewFiles     int                 `json:"new_files"`
	UpdatedFiles int                 `json:"updated_files"`
	DeletedFiles int                 `json:"deleted_files"`
	Conflicts    *updateConflictJSON `json:"conflicts,omitempty"`
}

func newUpdateDoc(mode templateupdate.UpdateMode, dryRun bool, result *templateupdate.UpdateResult) updateDoc {
	doc := updateDoc{
		Mode:   updateModeString(mode),
		DryRun: dryRun,
		OldSHA: result.OldSHA,
		NewSHA: result.NewSHA,
	}

	// Every mode emits the same key set. abort and continue genuinely apply no
	// files, so they report files: [] and zero counters rather than dropping
	// the keys — one stable shape means a consumer never has to branch on
	// "mode" just to know whether a field will be there.
	doc.UpToDate = result.OldSHA == result.NewSHA

	files := make([]updateFileJSON, 0, len(result.Applied))
	for _, r := range result.Applied {
		files = append(files, updateFileJSON{
			Path:         r.Path,
			Op:           r.Op.String(),
			Conflicted:   r.Conflicted,
			IsBinary:     r.IsBinary,
			PromptReason: r.PromptReason,
		})
	}
	doc.Files = files
	doc.NewFiles = result.NewFiles
	doc.UpdatedFiles = result.UpdatedFiles
	doc.DeletedFiles = result.DeletedFiles

	if result.Conflicts != nil && result.Conflicts.HasConflicts() {
		conflicted := make([]string, 0, len(result.Conflicts.Conflicts))
		for _, cf := range result.Conflicts.Conflicts {
			conflicted = append(conflicted, cf.Path)
		}
		prompts := make([]string, 0, len(result.Conflicts.Prompts))
		for _, p := range result.Conflicts.Prompts {
			prompts = append(prompts, p.Path)
		}
		skipped := make([]string, len(result.Conflicts.Skipped))
		copy(skipped, result.Conflicts.Skipped)

		doc.Conflicts = &updateConflictJSON{
			ConflictedFiles: conflicted,
			PromptFiles:     prompts,
			Skipped:         skipped,
		}
	}

	return doc
}

func updateModeString(mode templateupdate.UpdateMode) string {
	switch mode {
	case templateupdate.UpdateModeContinue:
		return "continue"
	case templateupdate.UpdateModeAbort:
		return "abort"
	case templateupdate.UpdateModeApply:
		return "apply"
	default:
		return "apply"
	}
}

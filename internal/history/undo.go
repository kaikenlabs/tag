package history

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/kaikenlabs/tag/internal/types"
)

// UndoOptions controls the behaviour of the undo engine.
type UndoOptions struct {
	// GenID identifies the generation to undo. When empty, the last generation
	// in the manifest is used.
	GenID string
	// Force skips conflict detection and proceeds even when files have been
	// modified after the generation was recorded.
	Force bool
	// Partial reverts files that have not been modified and silently skips
	// those that conflict, instead of aborting.
	Partial bool
	// Out receives the human-readable summary written after a successful undo.
	Out io.Writer
}

// Undo reverts a generation recorded in tagDir's manifest.
// On success, the generation entry is removed from the manifest.
func Undo(tagDir string, opts UndoOptions) error {
	m, err := Load(tagDir)
	if err != nil {
		return err
	}

	// Resolve target generation.
	var gen Generation
	if opts.GenID == "" {
		if len(m.Generations) == 0 {
			return errors.New("no generations to undo")
		}
		gen = m.Generations[len(m.Generations)-1]
	} else {
		idx := indexByID(m, opts.GenID)
		if idx < 0 {
			return ErrNotFound
		}
		gen = m.Generations[idx]
	}

	// Conflict detection: compare current file hash vs recorded hash_after.
	conflicted := checkConflicts(gen)

	if len(conflicted) > 0 {
		if !opts.Force && !opts.Partial {
			return &ConflictError{Paths: conflicted}
		}
		if opts.Partial {
			printConflictWarning(opts.Out, gen.ID, conflicted)
		}
	}

	// Revert files in reverse order.
	backupDir := filepath.Join(tagDir, types.HistoryBackupsDir, gen.ID)
	var reverted, skipped int
	for i := len(gen.Files) - 1; i >= 0; i-- {
		entry := gen.Files[i]

		// In partial mode, skip conflicted files.
		if opts.Partial && !opts.Force && isConflicted(entry, conflicted) {
			skipped++
			continue
		}

		if err := revertFile(entry, backupDir); err != nil {
			return fmt.Errorf("undo file %s: %w", entry.Path, err)
		}
		reverted++
	}

	// Clean up empty directories left by deleted files.
	cleanEmptyDirs(gen.Files)

	// Remove backup directory for this generation.
	_ = os.RemoveAll(backupDir)

	// Remove generation from manifest.
	if err := Remove(tagDir, gen.ID); err != nil {
		return fmt.Errorf("update manifest: %w", err)
	}

	printSummary(opts.Out, gen, reverted, skipped)
	return nil
}

// ListGenerations returns all generations in the manifest, newest first.
func ListGenerations(tagDir string) ([]Generation, error) {
	m, err := Load(tagDir)
	if err != nil {
		return nil, err
	}
	// Return a copy reversed so newest is first.
	gs := make([]Generation, len(m.Generations))
	for i, g := range m.Generations {
		gs[len(m.Generations)-1-i] = g
	}
	return gs, nil
}

// checkConflicts returns the paths of files whose current hash differs from
// the recorded hash_after (i.e., they have been modified after generation).
func checkConflicts(gen Generation) []string {
	var conflicts []string
	for _, entry := range gen.Files {
		current, err := HashFile(entry.Path)
		if err != nil {
			// File no longer exists or can't be read; for "create" actions this
			// means the file was already deleted — not a conflict.
			if entry.Action == ActionCreate {
				continue
			}
			// For inject/append, missing backup target is a conflict.
			conflicts = append(conflicts, entry.Path)
			continue
		}
		if current != entry.HashAfter {
			conflicts = append(conflicts, entry.Path)
		}
	}
	return conflicts
}

func isConflicted(entry FileEntry, conflicted []string) bool {
	return slices.Contains(conflicted, entry.Path)
}

// revertFile undoes a single FileEntry.
func revertFile(entry FileEntry, backupDir string) error {
	switch entry.Action {
	case ActionCreate:
		// Delete the file. If it no longer exists, treat as already reverted.
		if _, statErr := os.Stat(entry.Path); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return nil // already gone — desired state
			}
			return fmt.Errorf("check file %s: %w", entry.Path, statErr)
		}
		return os.Remove(entry.Path)

	case ActionInject, ActionAppend, ActionOverwrite, ActionOpenAPIMerge:
		// Restore from backup. RecordingFileWriter.snapshotBefore backs up any
		// pre-existing file before WriteFile (overwrite) and before
		// MergeOpenAPIFile (openapi-merge), same as it does for inject/append,
		// so restore-from-backup is correct for all four.
		backupPath := filepath.Join(backupDir, entry.Path)
		data, err := os.ReadFile(backupPath)
		if err != nil {
			return fmt.Errorf("read backup for %s: %w", entry.Path, err)
		}
		info, err := os.Stat(backupPath)
		if err != nil {
			return fmt.Errorf("stat backup for %s: %w", entry.Path, err)
		}
		if err := os.MkdirAll(filepath.Dir(entry.Path), types.DirMode); err != nil {
			return fmt.Errorf("create parent dir for %s: %w", entry.Path, err)
		}
		return os.WriteFile(entry.Path, data, info.Mode())
	}
	// An unrecognised action (e.g. written by a future TAG version) is
	// intentionally a no-op rather than an error, so undoing a manifest
	// produced by a newer TAG version does not hard-fail.
	return nil
}

// cleanEmptyDirs removes empty directories left behind after deleting created files.
func cleanEmptyDirs(files []FileEntry) {
	seen := make(map[string]bool)
	for _, entry := range files {
		if entry.Action != ActionCreate {
			continue
		}
		dir := filepath.Dir(entry.Path)
		for dir != "." && dir != "/" && !seen[dir] {
			seen[dir] = true
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) > 0 {
				break
			}
			if err := os.Remove(dir); err != nil {
				break
			}
			dir = filepath.Dir(dir)
		}
	}
}

func printConflictWarning(out io.Writer, genID string, paths []string) {
	if out == nil {
		return
	}
	fmt.Fprintf(out, "Warning: the following files were modified after generation %s and will be skipped:\n", genID)
	for _, p := range paths {
		fmt.Fprintf(out, "  - %s\n", p)
	}
}

func printSummary(out io.Writer, gen Generation, reverted, skipped int) {
	if out == nil {
		return
	}
	fmt.Fprintf(out, "Undid generation %s (%s)\n", gen.ID, gen.Template)
	fmt.Fprintf(out, "  %d file(s) reverted", reverted)
	if skipped > 0 {
		fmt.Fprintf(out, ", %d file(s) skipped (conflict)", skipped)
	}
	fmt.Fprintln(out)
}

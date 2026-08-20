package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/history"
)

// seedUndoProjectWithGeneration chdirs into a fresh temp project (as
// seedUndoProject does) with a single recorded generation that created
// target, and returns the project dir and target's relative path.
func seedUndoProjectWithGeneration(t *testing.T, genID string) (dir, target string) {
	t.Helper()
	dir = seedUndoProject(t)
	target = "handler.go"
	require.NoError(t, os.WriteFile(filepath.Join(dir, target), []byte("package main\n"), 0o600))
	hash, err := history.HashFile(target)
	require.NoError(t, err)
	require.NoError(t, history.Append(filepath.Join(dir, ".tag"), history.Generation{
		ID:        genID,
		Timestamp: time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
		Template:  "model",
		Command:   "generate",
		Files:     []history.FileEntry{{Path: target, Action: history.ActionCreate, HashAfter: hash}},
	}))
	return dir, target
}

// TestUT_UndoJSON_RequiresYes exercises D2: JSON mode must never imply
// consent for a destructive operation. Without --yes, --format json is a
// usage error and nothing is reverted or written.
func TestUT_UndoJSON_RequiresYes(t *testing.T) {
	_, target := seedUndoProjectWithGeneration(t, "gen_1_aaa")

	run := runCLICapturingAll(t, UndoCommand(), "undo", "--format", "json")
	require.Error(t, run.Err)
	assert.Contains(t, run.Err.Error(), "--yes")
	assert.Empty(t, run.Writer)

	// Nothing was reverted.
	_, statErr := os.Stat(target)
	assert.NoError(t, statErr)
}

// TestUT_UndoJSON_RevertsAndWritesDocument exercises the success path end to
// end: the document decodes to exactly one value, nothing bypasses
// c.App.Writer, and the human-readable summary (written through
// history.Undo's opts.Out) lands on ErrOut rather than corrupting stdout.
func TestUT_UndoJSON_RevertsAndWritesDocument(t *testing.T) {
	_, target := seedUndoProjectWithGeneration(t, "gen_1_aaa")

	run := runCLICapturingAll(t, UndoCommand(), "undo", "--yes", "--format", "json")
	require.NoError(t, run.Err)
	require.Empty(t, run.Stdout, "nothing should bypass c.App.Writer to the real os.Stdout")

	var doc undoDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	assert.Equal(t, "gen_1_aaa", doc.GenID)
	assert.Equal(t, 1, doc.Reverted)
	assert.Equal(t, 0, doc.Skipped)
	assert.Nil(t, doc.Conflicts)
	assert.Equal(t, []undoFileJSON{{Path: target, Action: "create", Reverted: true}}, doc.Files)

	assert.Contains(t, run.ErrOut, "gen_1_aaa")
	assert.NotContains(t, run.Writer, "Undid generation")

	_, statErr := os.Stat(target)
	assert.True(t, os.IsNotExist(statErr), "reverted create should delete the file")
}

// TestUT_UndoJSON_ConflictWritesDocumentAndExitCode exercises a real
// (non-mocked) history.ConflictError: a document with "conflicts" populated
// is written AND the command still exits non-zero.
func TestUT_UndoJSON_ConflictWritesDocumentAndExitCode(t *testing.T) {
	dir, target := seedUndoProjectWithGeneration(t, "gen_1_aaa")
	// Simulate a post-generation edit so the recorded hash no longer matches.
	require.NoError(t, os.WriteFile(filepath.Join(dir, target), []byte("user modified\n"), 0o600))

	run := runCLICapturingAll(t, UndoCommand(), "undo", "--yes", "--format", "json")
	require.Error(t, run.Err)

	type exitCoder interface{ ExitCode() int }
	ec, ok := run.Err.(exitCoder)
	require.True(t, ok)
	assert.Equal(t, 1, ec.ExitCode())

	var doc undoDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc), "a document must still be written on conflict")
	assert.Equal(t, "gen_1_aaa", doc.GenID)
	assert.Equal(t, []string{target}, doc.Conflicts)
	assert.Equal(t, 0, doc.Reverted)
	assert.Empty(t, doc.Files)

	// The file was left untouched.
	content, err := os.ReadFile(filepath.Join(dir, target))
	require.NoError(t, err)
	assert.Equal(t, "user modified\n", string(content))
}

// TestUT_UndoJSON_PartialReportsSkippedAndReverted drives --partial through
// the command layer for the first time (strategy.md notes --partial has no
// command-level test at all): one clean file is reverted, one conflicting
// file is skipped, and both counts and the conflicts list come from the real
// history.UndoResult rather than being recomputed by the command.
func TestUT_UndoJSON_PartialReportsSkippedAndReverted(t *testing.T) {
	dir := seedUndoProject(t)

	clean := "clean.go"
	dirty := "dirty.go"
	require.NoError(t, os.WriteFile(filepath.Join(dir, clean), []byte("clean\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, dirty), []byte("dirty-original\n"), 0o600))

	cleanHash, err := history.HashFile(clean)
	require.NoError(t, err)
	dirtyHash, err := history.HashFile(dirty)
	require.NoError(t, err)

	require.NoError(t, history.Append(filepath.Join(dir, ".tag"), history.Generation{
		ID:      "gen_1_aaa",
		Command: "generate",
		Files: []history.FileEntry{
			{Path: clean, Action: history.ActionCreate, HashAfter: cleanHash},
			{Path: dirty, Action: history.ActionCreate, HashAfter: dirtyHash},
		},
	}))

	// Modify only the dirty file after generation.
	require.NoError(t, os.WriteFile(filepath.Join(dir, dirty), []byte("user modified\n"), 0o600))

	run := runCLICapturingAll(t, UndoCommand(), "undo", "--yes", "--partial", "--format", "json")
	require.NoError(t, run.Err)

	var doc undoDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	assert.Equal(t, 1, doc.Reverted)
	assert.Equal(t, 1, doc.Skipped)
	assert.Equal(t, []string{dirty}, doc.Conflicts)

	_, cleanStatErr := os.Stat(filepath.Join(dir, clean))
	assert.True(t, os.IsNotExist(cleanStatErr), "clean file should be reverted")
	_, dirtyStatErr := os.Stat(filepath.Join(dir, dirty))
	assert.NoError(t, dirtyStatErr, "dirty file should be left in place (skipped)")
}

// TestUT_UndoJSON_ListEmitsGenerationsKey exercises D7: `undo --list
// --format json` must also emit JSON, wrapped under "generations" — a
// --format flag that silently ignores a sibling flag on the same command is
// a bug.
func TestUT_UndoJSON_ListEmitsGenerationsKey(t *testing.T) {
	dir := seedUndoProject(t)
	require.NoError(t, history.Append(filepath.Join(dir, ".tag"), history.Generation{
		ID:        "gen_1_aaa",
		Timestamp: time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
		Template:  "model",
		Command:   "generate",
		Files:     []history.FileEntry{{Path: "handler.go", Action: history.ActionCreate, HashAfter: "sha256:abc"}},
	}))

	run := runCLICapturingAll(t, UndoCommand(), "undo", "--list", "--format", "json")
	require.NoError(t, run.Err)

	var doc undoListDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	require.Len(t, doc.Generations, 1)
	assert.Equal(t, "gen_1_aaa", doc.Generations[0].ID)
	assert.Equal(t, "model", doc.Generations[0].Template)
	assert.Equal(t, 1, doc.Generations[0].Files)
}

// TestUT_UndoJSON_ListEmptyIsEmptyArrayNotNull pins the []T,0,n convention
// for the list surface.
func TestUT_UndoJSON_ListEmptyIsEmptyArrayNotNull(t *testing.T) {
	seedUndoProject(t)

	run := runCLICapturingAll(t, UndoCommand(), "undo", "--list", "--format", "json")
	require.NoError(t, run.Err)
	assert.Contains(t, run.Writer, `"generations": []`)
}

// TestUT_Undo_StrayPositionalIsUsageError closes a silent-and-destructive
// footgun. `undo` selects a generation with --id and has never taken a
// positional, but it also never rejected one: `tag undo gen_1787_566c0e`
// discarded the token and undid the LAST generation instead of the named one.
// Adding reparseTrailingFlags (needed for a trailing --format) does not change
// that on its own, since the returned positionals were simply dropped.
//
// update got this guard in the same change; undo needs it more, because here
// the silent fallback reverts the wrong files.
func TestUT_Undo_StrayPositionalIsUsageError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag"), 0o750))
	t.Chdir(dir)

	for _, argv := range [][]string{
		{"undo", "gen_1_aaa", "--yes", "--format", "json"},
		{"undo", "gen_1_aaa", "--yes"},
	} {
		run := runCLICapturingAll(t, UndoCommand(), argv...)
		require.Error(t, run.Err, "argv %v", argv)
		assert.Contains(t, run.Err.Error(), "does not accept positional arguments")
		assert.Empty(t, run.Stdout)
	}
}

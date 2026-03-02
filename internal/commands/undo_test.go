package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/history"
)

// runUndoApp is a test helper that runs the undo command within a temp project
// directory (containing a .tag/ directory) and returns output + error.
func runUndoApp(t *testing.T, projectDir string, args ...string) (string, error) {
	t.Helper()
	// Change into the project dir so resolveTagDir finds .tag/.
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projectDir))
	t.Cleanup(func() { _ = os.Chdir(orig) })

	var out bytes.Buffer
	app := &cli.App{
		Writer:         &out,
		Commands:       []*cli.Command{UndoCommand()},
		ExitErrHandler: func(_ *cli.Context, _ error) {},
	}

	argv := append([]string{"tag", "undo"}, args...)
	err = app.Run(argv)
	return out.String(), err
}

func TestUT_UndoCommand_List_EmptyHistory(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")
	require.NoError(t, os.MkdirAll(tagDir, 0o755))

	out, err := runUndoApp(t, dir, "--list")
	require.NoError(t, err)
	assert.Contains(t, out, "No generations recorded")
}

func TestUT_UndoCommand_List_ShowsGenerations(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")
	require.NoError(t, os.MkdirAll(tagDir, 0o755))

	g := history.Generation{
		ID:        "gen_1_aaa",
		Timestamp: time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
		Template:  "model",
		Command:   "generate",
		Files:     []history.FileEntry{{Path: "handler.go", Action: history.ActionCreate, HashAfter: "sha256:abc"}},
	}
	require.NoError(t, history.Append(tagDir, g))

	out, err := runUndoApp(t, dir, "--list")
	require.NoError(t, err)
	assert.Contains(t, out, "gen_1_aaa")
	assert.Contains(t, out, "model")
}

func TestUT_UndoCommand_UndoLastGeneration_WithYes(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")
	require.NoError(t, os.MkdirAll(tagDir, 0o755))

	// Create a file to be undone.
	target := filepath.Join(dir, "handler.go")
	require.NoError(t, os.WriteFile(target, []byte("package main"), 0o644))
	hash, err := history.HashFile(target)
	require.NoError(t, err)

	g := history.Generation{
		ID:      "gen_1_aaa",
		Command: "generate", Template: "model",
		Files: []history.FileEntry{{Path: target, Action: history.ActionCreate, HashAfter: hash}},
	}
	require.NoError(t, history.Append(tagDir, g))

	_, err = runUndoApp(t, dir, "--yes")
	require.NoError(t, err)

	_, statErr := os.Stat(target)
	assert.True(t, os.IsNotExist(statErr))
}

func TestUT_UndoCommand_ConflictError_DisplaysHelp(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")
	require.NoError(t, os.MkdirAll(tagDir, 0o755))

	target := filepath.Join(dir, "handler.go")
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))
	hashAfter := history.HashBytes([]byte("original"))

	g := history.Generation{
		ID:      "gen_1_aaa",
		Command: "generate",
		Files:   []history.FileEntry{{Path: target, Action: history.ActionCreate, HashAfter: hashAfter}},
	}
	require.NoError(t, history.Append(tagDir, g))

	// User modifies the file.
	require.NoError(t, os.WriteFile(target, []byte("user modified"), 0o644))

	out, err := runUndoApp(t, dir, "--yes")
	assert.Error(t, err)
	assert.Contains(t, out, "--force")
}

func TestUT_UndoCommand_UnknownTagDir_StillWorks(t *testing.T) {
	// Even without a .tag/ dir, undo --list should show "No generations".
	dir := t.TempDir()
	// No .tag/ created — resolveTagDir falls back to cwd/.tag

	out, err := runUndoApp(t, dir, "--list")
	require.NoError(t, err)
	assert.Contains(t, out, "No generations recorded")
}

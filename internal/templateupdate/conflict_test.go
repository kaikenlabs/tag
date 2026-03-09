package templateupdate

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_ConflictReport_FromMergeResults(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "clean.go", Op: MergeUpdate, Content: []byte("ok")},
		{Path: "conflict.go", Op: MergeConflict, Conflicted: true, Content: []byte("<<<<<<< x")},
		{Path: "prompt.go", Op: MergePrompt, PromptReason: "user deleted"},
		{Path: "keep.go", Op: MergeKeep},
	}
	skipped := []string{"ignored.log"}

	report := NewConflictReport(results, skipped)

	assert.Len(t, report.Clean, 2) // update + keep
	assert.Len(t, report.Conflicts, 1)
	assert.Len(t, report.Prompts, 1)
	assert.Equal(t, skipped, report.Skipped)
	assert.True(t, report.HasConflicts())
}

func TestUT_ConflictReport_NoConflicts(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "a.go", Op: MergeUpdate},
		{Path: "b.go", Op: MergeKeep},
	}

	report := NewConflictReport(results, nil)
	assert.False(t, report.HasConflicts())
}

func TestUT_ConflictReport_CountRegions(t *testing.T) {
	t.Parallel()

	content := []byte("<<<<<<< LOCAL\nours\n||||||| BASE\nbase\n=======\ntheirs\n>>>>>>> TEMPLATE\n" +
		"middle\n<<<<<<< LOCAL\nours2\n=======\ntheirs2\n>>>>>>> TEMPLATE\n")

	count := countConflictMarkers(content)
	assert.Equal(t, 2, count)
}

func TestUT_AcceptOurs_ResolvesAllConflicts(t *testing.T) {
	t.Parallel()

	report := &ConflictReport{
		Conflicts: []ConflictedFile{
			{Path: "a.go", OursContent: []byte("ours-a"), TheirsContent: []byte("theirs-a"), Mode: 0o644},
			{Path: "b.go", OursContent: []byte("ours-b"), TheirsContent: []byte("theirs-b"), Mode: 0o755},
		},
	}

	resolved := ResolveConflicts(report, ResolveOurs)
	require.Len(t, resolved, 2)

	assert.Equal(t, "a.go", resolved[0].Path)
	assert.Equal(t, []byte("ours-a"), resolved[0].Content)
	assert.Equal(t, MergeUpdate, resolved[0].Op)

	assert.Equal(t, "b.go", resolved[1].Path)
	assert.Equal(t, []byte("ours-b"), resolved[1].Content)
}

func TestUT_AcceptTheirs_ResolvesAllConflicts(t *testing.T) {
	t.Parallel()

	report := &ConflictReport{
		Conflicts: []ConflictedFile{
			{Path: "a.go", OursContent: []byte("ours"), TheirsContent: []byte("theirs"), Mode: 0o644},
		},
	}

	resolved := ResolveConflicts(report, ResolveTheirs)
	require.Len(t, resolved, 1)
	assert.Equal(t, []byte("theirs"), resolved[0].Content)
}

func TestUT_ResolveNone_ReturnsNil(t *testing.T) {
	t.Parallel()

	report := &ConflictReport{
		Conflicts: []ConflictedFile{{Path: "a.go"}},
	}

	resolved := ResolveConflicts(report, ResolveNone)
	assert.Nil(t, resolved)
}

func TestUT_ConflictStatusFile_WriteAndRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create .tag directory.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, types.TemplatesDir), 0o755))

	report := &ConflictReport{
		Conflicts: []ConflictedFile{{Path: "a.go"}, {Path: "b.go"}},
		Prompts:   []MergeResult{{Path: "c.go", Op: MergePrompt}},
	}

	status := NewConflictStatus(report, "abc123")
	require.NoError(t, WriteConflictStatus(dir, status))

	loaded, err := ReadConflictStatus(dir)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, conflictStatusVersion, loaded.SchemaVersion)
	assert.Equal(t, "abc123", loaded.UpdateCommit)
	assert.Equal(t, []string{"a.go", "b.go"}, loaded.ConflictedFiles)
	assert.Equal(t, []string{"c.go"}, loaded.PromptFiles)
	assert.Empty(t, loaded.ResolvedFiles)
	assert.False(t, loaded.StartedAt.IsZero())
}

func TestUT_ConflictStatusFile_ReadMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	status, err := ReadConflictStatus(dir)
	require.NoError(t, err)
	assert.Nil(t, status)
}

func TestUT_ConflictStatusFile_Clear(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tagDir := filepath.Join(dir, types.TemplatesDir)
	require.NoError(t, os.MkdirAll(tagDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "conflicts.json"), []byte("{}"), 0o644))

	require.NoError(t, ClearConflictStatus(dir))

	_, err := os.Stat(filepath.Join(tagDir, "conflicts.json"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_ConflictStatusFile_ClearMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Should not error when file doesn't exist.
	require.NoError(t, ClearConflictStatus(dir))
}

func TestUT_ConflictSummary_Format(t *testing.T) {
	t.Parallel()

	report := &ConflictReport{
		Conflicts: []ConflictedFile{
			{Path: "src/config.go", MarkerCount: 3},
			{Path: "Makefile", MarkerCount: 1},
		},
		Prompts: []MergeResult{
			{Path: "README.md", Op: MergePrompt, PromptReason: "user deleted"},
		},
	}

	var buf bytes.Buffer
	FormatConflictSummary(&buf, report)
	output := buf.String()

	assert.Contains(t, output, "3 conflict(s)")
	assert.Contains(t, output, "CONFLICT  src/config.go (3 region(s))")
	assert.Contains(t, output, "CONFLICT  Makefile (1 region(s))")
	assert.Contains(t, output, "PROMPT    README.md")
	assert.Contains(t, output, "tag update --continue")
	assert.Contains(t, output, "--accept-ours")
}

func TestUT_ConflictSummary_NoConflicts(t *testing.T) {
	t.Parallel()

	report := &ConflictReport{}
	var buf bytes.Buffer
	FormatConflictSummary(&buf, report)
	assert.Empty(t, buf.String())
}

func TestUT_CleanSummary_Format(t *testing.T) {
	t.Parallel()

	report := &ConflictReport{
		Clean: []MergeResult{
			{Op: MergeAdd},
			{Op: MergeAdd},
			{Op: MergeUpdate},
			{Op: MergeDelete},
		},
		Skipped: []string{"a.log"},
	}

	var buf bytes.Buffer
	FormatCleanSummary(&buf, report)
	output := buf.String()

	assert.Contains(t, output, "2 added")
	assert.Contains(t, output, "1 updated")
	assert.Contains(t, output, "1 deleted")
	assert.Contains(t, output, "1 skipped")
}

func TestUT_CleanSummary_NoChanges(t *testing.T) {
	t.Parallel()

	report := &ConflictReport{
		Clean: []MergeResult{
			{Op: MergeKeep},
		},
	}

	var buf bytes.Buffer
	FormatCleanSummary(&buf, report)
	assert.Contains(t, buf.String(), "up to date")
}

func TestUT_MergeOp_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		op   MergeOp
		want string
	}{
		{MergeKeep, "keep"},
		{MergeAdd, "add"},
		{MergeDelete, "delete"},
		{MergeUpdate, "update"},
		{MergeConflict, "conflict"},
		{MergeUserAdded, "user-added"},
		{MergePrompt, "prompt"},
		{MergeOp(99), "unknown"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, tc.op.String())
	}
}

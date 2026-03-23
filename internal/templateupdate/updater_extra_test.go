package templateupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_BuildUpdateResult_CountsOps(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "a.go", Op: MergeAdd},
		{Path: "b.go", Op: MergeAdd},
		{Path: "c.go", Op: MergeUpdate},
		{Path: "d.go", Op: MergeDelete},
		{Path: "e.go", Op: MergeKeep},
	}

	got := buildUpdateResult("old-sha", "new-sha", results, nil)

	assert.Equal(t, "old-sha", got.OldSHA)
	assert.Equal(t, "new-sha", got.NewSHA)
	assert.Equal(t, 2, got.NewFiles)
	assert.Equal(t, 1, got.UpdatedFiles)
	assert.Equal(t, 1, got.DeletedFiles)
	assert.Len(t, got.Applied, 5)
}

func TestUT_BuildUpdateResult_EmptyResults(t *testing.T) {
	t.Parallel()

	got := buildUpdateResult("a", "b", nil, nil)

	assert.Equal(t, 0, got.NewFiles)
	assert.Equal(t, 0, got.UpdatedFiles)
	assert.Equal(t, 0, got.DeletedFiles)
	assert.Nil(t, got.Applied)
	assert.Nil(t, got.Conflicts)
}

func TestUT_BuildUpdateResult_WithConflictReport(t *testing.T) {
	t.Parallel()

	report := &ConflictReport{
		Conflicts: []ConflictedFile{{Path: "x.go"}},
	}
	got := buildUpdateResult("a", "b", nil, report)

	require.NotNil(t, got.Conflicts)
	assert.Len(t, got.Conflicts.Conflicts, 1)
}

func TestUT_ReplaceConflicts_ReplacesMatchingPaths(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "a.go", Op: MergeConflict, Content: []byte("conflict")},
		{Path: "b.go", Op: MergeUpdate, Content: []byte("ok")},
		{Path: "c.go", Op: MergePrompt, Content: []byte("prompt")},
	}
	resolved := []MergeResult{
		{Path: "a.go", Op: MergeUpdate, Content: []byte("resolved-a")},
		{Path: "c.go", Op: MergeUpdate, Content: []byte("resolved-c")},
	}

	out := replaceConflicts(results, resolved)

	require.Len(t, out, 3)
	assert.Equal(t, MergeUpdate, out[0].Op)
	assert.Equal(t, []byte("resolved-a"), out[0].Content)
	assert.Equal(t, MergeUpdate, out[1].Op) // b.go unchanged
	assert.Equal(t, MergeUpdate, out[2].Op)
	assert.Equal(t, []byte("resolved-c"), out[2].Content)
}

func TestUT_ReplaceConflicts_NoReplacements(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "a.go", Op: MergeKeep},
	}
	out := replaceConflicts(results, nil)

	require.Len(t, out, 1)
	assert.Equal(t, MergeKeep, out[0].Op)
}

func TestUT_ApplyResults_WritesAndDeletesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	results := []MergeResult{
		{Path: "new.txt", Op: MergeAdd, Content: []byte("hello"), Mode: types.FileMode},
		{Path: "updated.txt", Op: MergeUpdate, Content: []byte("updated"), Mode: types.FileMode},
		{Path: "keep.txt", Op: MergeKeep},
		{Path: "user.txt", Op: MergeUserAdded},
	}

	err := applyResults(dir, results)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "new.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(content))

	content, err = os.ReadFile(filepath.Join(dir, "updated.txt"))
	require.NoError(t, err)
	assert.Equal(t, "updated", string(content))
}

func TestUT_ApplyResults_DeleteNonExistentIgnored(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	results := []MergeResult{
		{Path: "gone.txt", Op: MergeDelete},
	}

	err := applyResults(dir, results)
	assert.NoError(t, err)
}

func TestUT_ApplyResults_CreatesSubdirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	results := []MergeResult{
		{Path: "sub/dir/file.txt", Op: MergeAdd, Content: []byte("nested"), Mode: types.FileMode},
	}

	err := applyResults(dir, results)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "sub", "dir", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested", string(content))
}

func TestUT_UpdateTagConfig_WritesJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg := &scaffold.TagConfig{
		Template: &scaffold.TagTemplate{
			Source:    "github.com/example/repo",
			CommitSHA: "old-sha",
		},
		Variables: map[string]any{"key": "val"},
	}

	err := updateTagConfig(dir, cfg, "new-sha", "v2.0", map[string]any{"key": "newval"})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, types.TagConfigFile))
	require.NoError(t, err)

	var loaded scaffold.TagConfig
	require.NoError(t, json.Unmarshal(data, &loaded))

	assert.Equal(t, "new-sha", loaded.Template.CommitSHA)
	assert.Equal(t, "v2.0", loaded.Template.Ref)
	assert.Equal(t, "newval", loaded.Variables["key"])
}

func TestUT_UpdateTagConfig_EmptyRefNotOverwritten(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg := &scaffold.TagConfig{
		Template: &scaffold.TagTemplate{
			Ref: "v1.0",
		},
		Variables: map[string]any{},
	}

	err := updateTagConfig(dir, cfg, "sha", "", nil)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, types.TagConfigFile))
	require.NoError(t, err)

	var loaded scaffold.TagConfig
	require.NoError(t, json.Unmarshal(data, &loaded))

	assert.Equal(t, "v1.0", loaded.Template.Ref)
}

func TestUT_MergeVars_OverridesApplied(t *testing.T) {
	t.Parallel()

	stored := map[string]any{"a": "1", "b": "2"}
	overrides := map[string]string{"b": "99", "c": "3"}

	merged := mergeVars(stored, overrides)

	assert.Equal(t, "1", merged["a"])
	assert.Equal(t, "99", merged["b"])
	assert.Equal(t, "3", merged["c"])
	// Verify original map is not mutated.
	assert.Equal(t, "2", stored["b"])
}

func TestUT_MergeVars_NilOverrides(t *testing.T) {
	t.Parallel()

	stored := map[string]any{"x": "y"}
	merged := mergeVars(stored, nil)

	assert.Equal(t, "y", merged["x"])
	assert.Len(t, merged, 1)
}

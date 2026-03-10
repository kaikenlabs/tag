package templateupdate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
)

// mockRendererForUpdate is a test double for HistoricalRenderer.
// We test the Update orchestration by mocking at the resolver level.

func setupUpdateProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeTagConfig(t, dir, &scaffold.TagConfig{
		SchemaVersion: 1,
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: "abc123def456789012345678901234567890abcd",
		},
		Variables: map[string]any{
			"project_name": "myproject",
		},
	})

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# myproject"), 0o644))
	return dir
}

func TestUT_Updater_Update_AlreadyUpToDate(t *testing.T) {
	dir := setupUpdateProject(t)

	resolver := &mockResolver{sha: "abc123def456789012345678901234567890abcd"}
	renderer := NewHistoricalRenderer(&mockFetcher{})
	updater := NewUpdater(renderer, resolver)

	result, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: dir,
	})
	require.NoError(t, err)
	assert.Equal(t, result.OldSHA, result.NewSHA)
}

func TestUT_Updater_Update_ResolverError(t *testing.T) {
	dir := setupUpdateProject(t)

	resolver := &mockResolver{err: assert.AnError}
	renderer := NewHistoricalRenderer(&mockFetcher{})
	updater := NewUpdater(renderer, resolver)

	_, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: dir,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve latest commit")
}

func TestUT_Updater_Update_MissingConfig(t *testing.T) {
	dir := t.TempDir()

	resolver := &mockResolver{sha: "abc123"}
	renderer := NewHistoricalRenderer(&mockFetcher{})
	updater := NewUpdater(renderer, resolver)

	_, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: dir,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load project config")
}

func TestUT_Updater_Continue_NoPendingUpdate(t *testing.T) {
	dir := setupUpdateProject(t)

	resolver := &mockResolver{}
	renderer := NewHistoricalRenderer(&mockFetcher{})
	updater := NewUpdater(renderer, resolver)

	_, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: dir,
		Mode:       UpdateModeContinue,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pending update")
}

func TestUT_Updater_Abort_NoBackup(t *testing.T) {
	dir := setupUpdateProject(t)

	resolver := &mockResolver{}
	renderer := NewHistoricalRenderer(&mockFetcher{})
	updater := NewUpdater(renderer, resolver)

	_, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: dir,
		Mode:       UpdateModeAbort,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no backup found")
}

func TestUT_Updater_PendingConflictsBlock(t *testing.T) {
	dir := setupUpdateProject(t)

	// Write fake conflict status.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, types.TemplatesDir), 0o755))
	status := map[string]any{
		"schema_version":   1,
		"update_commit":    "newcommit",
		"conflicted_files": []string{"file.txt"},
	}
	data, marshalErr := json.Marshal(status)
	require.NoError(t, marshalErr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, types.TemplatesDir, "conflicts.json"), data, 0o644))

	resolver := &mockResolver{sha: "different_sha_12345678901234567890abcd"}
	renderer := NewHistoricalRenderer(&mockFetcher{})
	updater := NewUpdater(renderer, resolver)

	_, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: dir,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pending conflicts")
}

func TestUT_MergeVars(t *testing.T) {
	stored := map[string]any{"a": "1", "b": "2"}
	overrides := map[string]string{"b": "3", "c": "4"}

	merged := mergeVars(stored, overrides)
	assert.Equal(t, "1", merged["a"])
	assert.Equal(t, "3", merged["b"])
	assert.Equal(t, "4", merged["c"])
}

func TestUT_MergeVars_NoOverrides(t *testing.T) {
	stored := map[string]any{"a": "1"}
	merged := mergeVars(stored, nil)
	assert.Equal(t, stored, merged)
}

func TestUT_CollectAffectedPaths(t *testing.T) {
	results := []MergeResult{
		{Path: "new.txt", Op: MergeAdd},
		{Path: "keep.txt", Op: MergeKeep},
		{Path: "mod.txt", Op: MergeUpdate},
		{Path: "del.txt", Op: MergeDelete},
		{Path: "user.txt", Op: MergeUserAdded},
	}

	paths := collectAffectedPaths(results)
	assert.ElementsMatch(t, []string{"new.txt", "mod.txt", "del.txt"}, paths)
}

func TestUT_ReplaceConflicts(t *testing.T) {
	results := []MergeResult{
		{Path: "ok.txt", Op: MergeUpdate, Content: []byte("ok")},
		{Path: "conflict.txt", Op: MergeConflict, Content: []byte("<<<")},
	}

	resolved := []MergeResult{
		{Path: "conflict.txt", Op: MergeUpdate, Content: []byte("resolved")},
	}

	replaced := replaceConflicts(results, resolved)
	assert.Len(t, replaced, 2)
	assert.Equal(t, MergeUpdate, replaced[1].Op)
	assert.Equal(t, []byte("resolved"), replaced[1].Content)
}

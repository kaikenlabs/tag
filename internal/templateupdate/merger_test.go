package templateupdate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMerger is a TextMerger for testing that returns predictable results.
type mockMerger struct {
	merged     []byte
	conflicted bool
	err        error
}

func (m *mockMerger) Merge3(_ context.Context, _ string, _, _, _ []byte) ([]byte, bool, error) {
	return m.merged, m.conflicted, m.err
}

func rf(content string, binary bool) *RenderedFile {
	return &RenderedFile{
		Content:  []byte(content),
		Mode:     0o644,
		IsBinary: binary,
	}
}

func rfMode(content string, mode os.FileMode) *RenderedFile {
	return &RenderedFile{
		Content: []byte(content),
		Mode:    mode,
	}
}

var ctx = context.Background()

// --- Decision matrix tests ---

func TestUT_MergeFile_NewTemplateFile(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "new.go", nil, nil, rf("new content", false))
	require.NoError(t, err)
	assert.Equal(t, MergeAdd, result.Op)
	assert.Equal(t, []byte("new content"), result.Content)
}

func TestUT_MergeFile_UserAddedFile(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "user.txt", nil, rf("user stuff", false), nil)
	require.NoError(t, err)
	assert.Equal(t, MergeUserAdded, result.Op)
}

func TestUT_MergeFile_BothAddedSameContent(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "same.go", nil, rf("same", false), rf("same", false))
	require.NoError(t, err)
	assert.Equal(t, MergeKeep, result.Op)
}

func TestUT_MergeFile_BothAddedDifferentContent(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{merged: []byte("merged"), conflicted: false}, nil)
	result, err := engine.MergeFile(ctx, "diff.go", nil, rf("ours", false), rf("theirs", false))
	require.NoError(t, err)
	assert.Equal(t, MergeUpdate, result.Op)
	assert.Equal(t, []byte("merged"), result.Content)
}

func TestUT_MergeFile_BothAddedDifferentContentConflict(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{merged: []byte("conflict"), conflicted: true}, nil)
	result, err := engine.MergeFile(ctx, "diff.go", nil, rf("ours", false), rf("theirs", false))
	require.NoError(t, err)
	assert.Equal(t, MergeConflict, result.Op)
	assert.True(t, result.Conflicted)
}

func TestUT_MergeFile_BothAddedBinaryDifferent(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "img.png", nil, rf("bin1", true), rf("bin2", true))
	require.NoError(t, err)
	assert.Equal(t, MergePrompt, result.Op)
	assert.False(t, result.Conflicted, "binary prompts should not set Conflicted")
	assert.Contains(t, result.PromptReason, "binary")
}

func TestUT_MergeFile_BothDeleted(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "old.go", rf("base", false), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, MergeDelete, result.Op)
}

func TestUT_MergeFile_UserDeletedTemplateChanged(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "changed.go", rf("base", false), nil, rf("new template", false))
	require.NoError(t, err)
	assert.Equal(t, MergePrompt, result.Op)
	assert.Contains(t, result.PromptReason, "deleted")
}

func TestUT_MergeFile_UserDeletedTemplateUnchanged(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "same.go", rf("base", false), nil, rf("base", false))
	require.NoError(t, err)
	assert.Equal(t, MergeDelete, result.Op)
}

func TestUT_MergeFile_TemplateDeletedUserModified(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "kept.go", rf("base", false), rf("modified", false), nil)
	require.NoError(t, err)
	assert.Equal(t, MergePrompt, result.Op)
	assert.Contains(t, result.PromptReason, "removed")
}

func TestUT_MergeFile_TemplateDeletedUserUnmodified(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "gone.go", rf("base", false), rf("base", false), nil)
	require.NoError(t, err)
	assert.Equal(t, MergeDelete, result.Op)
}

func TestUT_MergeFile_UserUntouchedTemplateChanged(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "updated.go", rf("base", false), rf("base", false), rf("new", false))
	require.NoError(t, err)
	assert.Equal(t, MergeUpdate, result.Op)
	assert.Equal(t, []byte("new"), result.Content)
}

func TestUT_MergeFile_TemplateUnchangedUserModified(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "custom.go", rf("base", false), rf("custom", false), rf("base", false))
	require.NoError(t, err)
	assert.Equal(t, MergeKeep, result.Op)
}

func TestUT_MergeFile_Converged(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "same.go", rf("base", false), rf("new", false), rf("new", false))
	require.NoError(t, err)
	assert.Equal(t, MergeKeep, result.Op)
}

func TestUT_MergeFile_BothModifiedCleanMerge(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{merged: []byte("clean merge"), conflicted: false}, nil)
	result, err := engine.MergeFile(ctx, "merged.go",
		rf("base", false), rf("ours-mod", false), rf("theirs-mod", false))
	require.NoError(t, err)
	assert.Equal(t, MergeUpdate, result.Op)
	assert.Equal(t, []byte("clean merge"), result.Content)
	assert.False(t, result.Conflicted)
}

func TestUT_MergeFile_BothModifiedConflict(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{merged: []byte("<<<<<<< conflict"), conflicted: true}, nil)
	result, err := engine.MergeFile(ctx, "conflict.go",
		rf("base", false), rf("ours-mod", false), rf("theirs-mod", false))
	require.NoError(t, err)
	assert.Equal(t, MergeConflict, result.Op)
	assert.True(t, result.Conflicted)
}

func TestUT_MergeFile_BinaryFileConflict(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "image.png",
		rf("base-bin", true), rf("ours-bin", true), rf("theirs-bin", true))
	require.NoError(t, err)
	assert.Equal(t, MergePrompt, result.Op)
	assert.Contains(t, result.PromptReason, "binary")
	assert.True(t, result.IsBinary)
	assert.NotEmpty(t, result.OursSHA256, "should have SHA256 for ours")
	assert.NotEmpty(t, result.TheirsSHA256, "should have SHA256 for theirs")
	assert.NotEqual(t, result.OursSHA256, result.TheirsSHA256, "different content = different hashes")
}

func TestUT_MergeFile_TextMergerError(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{err: errors.New("merge failed")}, nil)
	_, err := engine.MergeFile(ctx, "error.go",
		rf("base", false), rf("ours", false), rf("theirs", false))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge failed")
}

func TestUT_MergeFile_FileModeChange(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	base := rf("content", false)
	ours := rfMode("content", 0o755) // User made executable.
	theirs := rf("content", false)   // Template unchanged.

	result, err := engine.MergeFile(ctx, "script.sh", base, ours, theirs)
	require.NoError(t, err)
	assert.Equal(t, MergeKeep, result.Op)
	assert.Equal(t, os.FileMode(0o755), result.Mode, "should preserve user's mode")
}

func TestUT_MergeFile_AllNil(t *testing.T) {
	t.Parallel()
	engine := NewMergeEngine(&mockMerger{}, nil)
	result, err := engine.MergeFile(ctx, "ghost.go", nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, MergeKeep, result.Op)
}

// --- MergeTrees tests ---

func TestUT_MergeTrees_CombinesAllFiles(t *testing.T) {
	t.Parallel()

	base := map[string]*RenderedFile{"a.go": rf("a", false)}
	ours := map[string]*RenderedFile{"a.go": rf("a", false), "b.go": rf("b", false)}
	theirs := map[string]*RenderedFile{"a.go": rf("a-new", false), "c.go": rf("c", false)}

	engine := NewMergeEngine(&mockMerger{}, nil)
	results, skipped, err := engine.MergeTrees(ctx, base, ours, theirs)
	require.NoError(t, err)
	assert.Empty(t, skipped)

	paths := make([]string, len(results))
	for i, r := range results {
		paths[i] = r.Path
	}
	assert.Equal(t, []string{"a.go", "b.go", "c.go"}, paths, "should be sorted")
}

func TestUT_MergeTrees_SkippedFilesExcluded(t *testing.T) {
	t.Parallel()

	theirs := map[string]*RenderedFile{
		"main.go": rf("main", false),
		"skip.me": rf("skip", false),
	}

	ignoreFn := func(path string) bool { return path == "skip.me" }
	engine := NewMergeEngine(&mockMerger{}, ignoreFn)
	results, skipped, err := engine.MergeTrees(ctx, nil, nil, theirs)
	require.NoError(t, err)

	assert.Equal(t, []string{"skip.me"}, skipped)
	require.Len(t, results, 1)
	assert.Equal(t, "main.go", results[0].Path)
}

func TestUT_MergeTrees_EmptyTrees(t *testing.T) {
	t.Parallel()

	engine := NewMergeEngine(&mockMerger{}, nil)
	results, skipped, err := engine.MergeTrees(ctx, nil, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.Empty(t, skipped)
}

func TestUT_MergeTrees_DeterministicOrder(t *testing.T) {
	t.Parallel()

	theirs := map[string]*RenderedFile{
		"z.go": rf("z", false),
		"a.go": rf("a", false),
		"m.go": rf("m", false),
	}

	engine := NewMergeEngine(&mockMerger{}, nil)
	results, _, err := engine.MergeTrees(ctx, nil, nil, theirs)
	require.NoError(t, err)

	paths := make([]string, len(results))
	for i, r := range results {
		paths[i] = r.Path
	}
	assert.Equal(t, []string{"a.go", "m.go", "z.go"}, paths)
}

func TestUT_MergeTrees_ErrorPropagation(t *testing.T) {
	t.Parallel()

	base := map[string]*RenderedFile{"a.go": rf("base", false)}
	ours := map[string]*RenderedFile{"a.go": rf("ours", false)}
	theirs := map[string]*RenderedFile{"a.go": rf("theirs", false)}

	engine := NewMergeEngine(&mockMerger{err: errors.New("boom")}, nil)
	_, _, err := engine.MergeTrees(ctx, base, ours, theirs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge a.go")
	assert.Contains(t, err.Error(), "boom")
}

// --- GitMerger integration tests (require git) ---

func TestIT_GitMerger_CleanMerge(t *testing.T) {
	t.Parallel()
	skipWithoutGit(t)

	// Edits must be separated by enough context lines for git merge-file
	// to consider them non-overlapping.
	base := []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\n")
	ours := []byte("line1\nline2-modified\nline3\nline4\nline5\nline6\nline7\n")
	theirs := []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7-modified\n")

	gm := &GitMerger{}
	merged, conflicted, err := gm.Merge3(ctx, "test.txt", base, ours, theirs)
	require.NoError(t, err)
	assert.False(t, conflicted)
	assert.Contains(t, string(merged), "line2-modified")
	assert.Contains(t, string(merged), "line7-modified")
}

func TestIT_GitMerger_ConflictMerge(t *testing.T) {
	t.Parallel()
	skipWithoutGit(t)

	base := []byte("line1\noriginal\nline3\n")
	ours := []byte("line1\nours-change\nline3\n")
	theirs := []byte("line1\ntheirs-change\nline3\n")

	gm := &GitMerger{}
	merged, conflicted, err := gm.Merge3(ctx, "test.txt", base, ours, theirs)
	require.NoError(t, err)
	assert.True(t, conflicted)
	assert.Contains(t, string(merged), "<<<<<<< ")
	assert.Contains(t, string(merged), ">>>>>>> ")
}

func TestIT_GitMerger_IdenticalInputs(t *testing.T) {
	t.Parallel()
	skipWithoutGit(t)

	content := []byte("same content\n")

	gm := &GitMerger{}
	merged, conflicted, err := gm.Merge3(ctx, "test.txt", content, content, content)
	require.NoError(t, err)
	assert.False(t, conflicted)
	assert.Equal(t, content, merged)
}

func TestIT_GitMerger_EmptyBase(t *testing.T) {
	t.Parallel()
	skipWithoutGit(t)

	gm := &GitMerger{}
	merged, conflicted, err := gm.Merge3(ctx, "test.txt", nil, []byte("ours\n"), []byte("theirs\n"))
	require.NoError(t, err)
	// With empty base, both additions conflict.
	assert.True(t, conflicted)
	assert.Contains(t, string(merged), "<<<<<<< ")
}

func skipWithoutGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

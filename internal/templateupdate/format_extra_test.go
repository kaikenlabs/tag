package templateupdate

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// splitLines
// ---------------------------------------------------------------------------

func TestUT_SplitLines_NilInput(t *testing.T) {
	t.Parallel()
	result := splitLines(nil)
	assert.Nil(t, result)
}

func TestUT_SplitLines_EmptyString(t *testing.T) {
	t.Parallel()
	result := splitLines([]byte(""))
	require.Len(t, result, 1)
	assert.Equal(t, "", result[0])
}

func TestUT_SplitLines_SingleLine(t *testing.T) {
	t.Parallel()
	result := splitLines([]byte("hello"))
	require.Len(t, result, 1)
	assert.Equal(t, "hello", result[0])
}

func TestUT_SplitLines_MultipleLines(t *testing.T) {
	t.Parallel()
	result := splitLines([]byte("line1\nline2\nline3"))
	require.Len(t, result, 3)
	assert.Equal(t, []string{"line1", "line2", "line3"}, result)
}

func TestUT_SplitLines_TrailingNewline(t *testing.T) {
	t.Parallel()
	result := splitLines([]byte("alpha\nbeta\n"))
	require.Len(t, result, 3)
	assert.Equal(t, []string{"alpha", "beta", ""}, result)
}

// ---------------------------------------------------------------------------
// formatDiffstat — all operations with conflict count
// ---------------------------------------------------------------------------

func TestUT_FormatDiffstat_AllOps(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "added1.go", Op: MergeAdd},
		{Path: "added2.go", Op: MergeAdd},
		{Path: "modified.go", Op: MergeUpdate},
		{Path: "removed.go", Op: MergeDelete},
		{Path: "clash.go", Op: MergeConflict},
		{Path: "kept.go", Op: MergeKeep},
		{Path: "user.go", Op: MergeUserAdded},
	}

	var buf bytes.Buffer
	formatDiffstat(&buf, results, false)

	output := buf.String()

	// Verify per-file symbols.
	assert.Contains(t, output, "+ added1.go")
	assert.Contains(t, output, "+ added2.go")
	assert.Contains(t, output, "~ modified.go")
	assert.Contains(t, output, "- removed.go")
	assert.Contains(t, output, "! clash.go")

	// Kept and user-added files must not appear.
	assert.NotContains(t, output, "kept.go")
	assert.NotContains(t, output, "user.go")

	// Summary line includes conflict count.
	assert.Contains(t, output, "2 file(s) added, 1 modified, 1 deleted, 1 conflicted")
}

func TestUT_FormatDiffstat_NoConflicts_OmitsConflictCount(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "a.txt", Op: MergeAdd},
	}

	var buf bytes.Buffer
	formatDiffstat(&buf, results, false)

	output := buf.String()
	assert.Contains(t, output, "1 file(s) added, 0 modified, 0 deleted")
	assert.NotContains(t, output, "conflicted")
}

func TestUT_FormatDiffstat_WithColor(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "new.go", Op: MergeAdd},
		{Path: "mod.go", Op: MergeUpdate},
		{Path: "del.go", Op: MergeDelete},
		{Path: "bad.go", Op: MergeConflict},
	}

	var buf bytes.Buffer
	formatDiffstat(&buf, results, true)

	output := buf.String()

	// chalk is a no-op in non-TTY, but the color=true branch is exercised.
	// All file paths are still present.
	assert.Contains(t, output, "new.go")
	assert.Contains(t, output, "mod.go")
	assert.Contains(t, output, "del.go")
	assert.Contains(t, output, "bad.go")
	assert.Contains(t, output, "1 conflicted")
}

func TestUT_FormatDiffstat_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	formatDiffstat(&buf, nil, false)

	output := buf.String()
	assert.Contains(t, output, "0 file(s) added, 0 modified, 0 deleted")
	assert.NotContains(t, output, "conflicted")
}

// ---------------------------------------------------------------------------
// formatFileDelete — nil BaseContent (no deleted lines shown)
// ---------------------------------------------------------------------------

func TestUT_FormatFileDelete_NilBaseContent(t *testing.T) {
	t.Parallel()

	r := MergeResult{Path: "gone.txt", Op: MergeDelete, BaseContent: nil}

	var buf bytes.Buffer
	formatFileDelete(&buf, r, false)

	output := buf.String()
	assert.Contains(t, output, "--- a/gone.txt")
	assert.Contains(t, output, "+++ /dev/null")

	// No line should be a removal marker (BaseContent is nil).
	for line := range strings.SplitSeq(buf.String(), "\n") {
		if line == "--- a/gone.txt" || line == "+++ /dev/null" {
			continue
		}
		if line != "" && line[0] == '-' {
			t.Errorf("unexpected removal line: %q", line)
		}
	}
}

func TestUT_FormatFileDelete_WithColor(t *testing.T) {
	t.Parallel()

	r := MergeResult{Path: "old.css", Op: MergeDelete, BaseContent: []byte("body {\n}\n")}

	var buf bytes.Buffer
	formatFileDelete(&buf, r, true)

	output := buf.String()
	// chalk is a no-op in non-TTY, but the color=true branch is exercised.
	assert.Contains(t, output, "old.css")
	assert.Contains(t, output, "body {")
}

// ---------------------------------------------------------------------------
// formatFileConflict — with color=false and color=true
// ---------------------------------------------------------------------------

func TestUT_FormatFileConflict_NoColor(t *testing.T) {
	t.Parallel()

	r := MergeResult{Path: "merge.go", Op: MergeConflict}

	var buf bytes.Buffer
	formatFileConflict(&buf, r, false)

	output := buf.String()
	assert.Contains(t, output, "merge.go")
	assert.Contains(t, output, "conflict")
	assert.NotContains(t, output, "\033[")
}

func TestUT_FormatFileConflict_WithColor(t *testing.T) {
	t.Parallel()

	r := MergeResult{Path: "merge.go", Op: MergeConflict}

	var buf bytes.Buffer
	formatFileConflict(&buf, r, true)

	output := buf.String()
	// chalk.Yellow is a no-op in non-TTY, but the color branch is still exercised.
	assert.Contains(t, output, "merge.go")
	assert.Contains(t, output, "conflict")
}

// ---------------------------------------------------------------------------
// FormatDiff — color=true header branch
// ---------------------------------------------------------------------------

func TestUT_FormatDiff_ColorHeader(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "f.txt", Op: MergeAdd, Content: []byte("x")},
	}

	var buf bytes.Buffer
	FormatDiff(results, "gh:org/repo", "aaa1111bbb", "ccc2222ddd", FormatOptions{
		Writer: &buf,
		Color:  true,
	})

	output := buf.String()
	// chalk is a no-op in non-TTY, but the Color=true branch is exercised.
	// Short SHAs must appear.
	assert.Contains(t, output, "aaa1111")
	assert.Contains(t, output, "ccc2222")
	assert.Contains(t, output, "gh:org/repo")
}

func TestUT_FormatDiff_ColorStat(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "a.go", Op: MergeAdd},
		{Path: "b.go", Op: MergeConflict},
	}

	var buf bytes.Buffer
	FormatDiff(results, "src", "abc1234", "def5678", FormatOptions{
		Writer: &buf,
		Color:  true,
		Stat:   true,
	})

	output := buf.String()
	// chalk is a no-op in non-TTY, but Color=true branches are exercised.
	assert.Contains(t, output, "1 file(s) added")
	assert.Contains(t, output, "1 conflicted")
	assert.Contains(t, output, "a.go")
	assert.Contains(t, output, "b.go")
}

// ---------------------------------------------------------------------------
// writeSimpleDiff
// ---------------------------------------------------------------------------
// simpleDiffLines — the pure primitive writeSimpleDiff prints from, and that
// Summarize (ticket #351) counts from, so "the diff is computed once" is an
// observable property rather than a claim.
// ---------------------------------------------------------------------------

func TestUT_SimpleDiffLines_BothEmpty(t *testing.T) {
	t.Parallel()
	assert.Empty(t, simpleDiffLines(nil, nil))
}

func TestUT_SimpleDiffLines_AddedLines(t *testing.T) {
	t.Parallel()

	got := simpleDiffLines(nil, []string{"alpha", "beta"})
	assert.Equal(t, []diffLine{
		{Sign: '+', Text: "alpha"},
		{Sign: '+', Text: "beta"},
	}, got)
}

func TestUT_SimpleDiffLines_RemovedLines(t *testing.T) {
	t.Parallel()

	got := simpleDiffLines([]string{"old1", "old2"}, nil)
	assert.Equal(t, []diffLine{
		{Sign: '-', Text: "old1"},
		{Sign: '-', Text: "old2"},
	}, got)
}

func TestUT_SimpleDiffLines_MixedChanges(t *testing.T) {
	t.Parallel()

	got := simpleDiffLines([]string{"keep", "remove"}, []string{"keep", "add"})
	assert.Equal(t, []diffLine{
		{Sign: '-', Text: "remove"},
		{Sign: '+', Text: "add"},
	}, got)
}

func TestUT_SimpleDiffLines_DuplicateLines(t *testing.T) {
	t.Parallel()

	// One "dup" is matched; the other plus "only_old" are removed.
	got := simpleDiffLines(
		[]string{"dup", "dup", "only_old"},
		[]string{"dup", "only_new"},
	)
	assert.Equal(t, []diffLine{
		{Sign: '-', Text: "dup"},
		{Sign: '-', Text: "only_old"},
		{Sign: '+', Text: "only_new"},
	}, got)
}

// TestUT_SimpleDiffLines_OrderingContract pins the exact ordering contract:
// removals in old-file order first, then additions in new-file order,
// duplicates matched by multiset count. This is the highest drift-risk
// behavior in the whole ticket — a refactor that sorts, dedupes, or
// interleaves the two passes would still pass every other test here.
func TestUT_SimpleDiffLines_OrderingContract(t *testing.T) {
	t.Parallel()

	old := []string{"z-remove", "a-remove", "dup", "dup"}
	updated := []string{"z-add", "a-add", "dup"}

	got := simpleDiffLines(old, updated)
	assert.Equal(t, []diffLine{
		{Sign: '-', Text: "z-remove"},
		{Sign: '-', Text: "a-remove"},
		{Sign: '-', Text: "dup"},
		{Sign: '+', Text: "z-add"},
		{Sign: '+', Text: "a-add"},
	}, got)
}

// ---------------------------------------------------------------------------

func TestUT_WriteSimpleDiff_BothEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writeSimpleDiff(&buf, nil, nil, false)
	assert.Empty(t, buf.String())
}

func TestUT_WriteSimpleDiff_AddedLines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writeSimpleDiff(&buf, nil, []string{"alpha", "beta"}, false)

	output := buf.String()
	assert.Contains(t, output, "+alpha")
	assert.Contains(t, output, "+beta")
	assert.NotContains(t, output, "-")
}

func TestUT_WriteSimpleDiff_RemovedLines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writeSimpleDiff(&buf, []string{"old1", "old2"}, nil, false)

	output := buf.String()
	assert.Contains(t, output, "-old1")
	assert.Contains(t, output, "-old2")
	assert.NotContains(t, output, "+")
}

func TestUT_WriteSimpleDiff_MixedChanges(t *testing.T) {
	t.Parallel()

	old := []string{"keep", "remove"}
	updated := []string{"keep", "add"}

	var buf bytes.Buffer
	writeSimpleDiff(&buf, old, updated, false)

	output := buf.String()
	assert.Contains(t, output, "-remove")
	assert.Contains(t, output, "+add")
	// "keep" appears in both sets, so it should not show up as +/-.
	assert.NotContains(t, output, "-keep")
	assert.NotContains(t, output, "+keep")
}

func TestUT_WriteSimpleDiff_WithColor(t *testing.T) {
	t.Parallel()

	old := []string{"gone"}
	updated := []string{"here"}

	var buf bytes.Buffer
	writeSimpleDiff(&buf, old, updated, true)

	output := buf.String()
	// chalk is a no-op in non-TTY, but the color=true branch is exercised.
	assert.Contains(t, output, "gone")
	assert.Contains(t, output, "here")
}

func TestUT_WriteSimpleDiff_DuplicateLines(t *testing.T) {
	t.Parallel()

	old := []string{"dup", "dup", "only_old"}
	updated := []string{"dup", "only_new"}

	var buf bytes.Buffer
	writeSimpleDiff(&buf, old, updated, false)

	output := buf.String()
	// One "dup" is matched; the other plus "only_old" are removed.
	assert.Contains(t, output, "-dup")
	assert.Contains(t, output, "-only_old")
	assert.Contains(t, output, "+only_new")
}

// ---------------------------------------------------------------------------
// formatFileUpdate — exercises writeSimpleDiff via the public path
// ---------------------------------------------------------------------------

func TestUT_FormatFileUpdate_NoColor(t *testing.T) {
	t.Parallel()

	r := MergeResult{
		Path:        "app.go",
		Op:          MergeUpdate,
		OursContent: []byte("old line\nshared"),
		Content:     []byte("new line\nshared"),
	}

	var buf bytes.Buffer
	formatFileUpdate(&buf, r, false)

	output := buf.String()
	assert.Contains(t, output, "--- a/app.go")
	assert.Contains(t, output, "+++ b/app.go")
	assert.Contains(t, output, "-old line")
	assert.Contains(t, output, "+new line")
	assert.NotContains(t, output, "\033[")
}

func TestUT_FormatFileUpdate_WithColor(t *testing.T) {
	t.Parallel()

	r := MergeResult{
		Path:        "app.go",
		Op:          MergeUpdate,
		OursContent: []byte("before"),
		Content:     []byte("after"),
	}

	var buf bytes.Buffer
	formatFileUpdate(&buf, r, true)

	output := buf.String()
	// chalk is a no-op in non-TTY, but the color=true branch is exercised.
	assert.Contains(t, output, "app.go")
	assert.Contains(t, output, "before")
	assert.Contains(t, output, "after")
}

// ---------------------------------------------------------------------------
// formatFileAdd — with color
// ---------------------------------------------------------------------------

func TestUT_FormatFileAdd_WithColor(t *testing.T) {
	t.Parallel()

	r := MergeResult{Path: "new.go", Op: MergeAdd, Content: []byte("package main\n")}

	var buf bytes.Buffer
	formatFileAdd(&buf, r, true)

	output := buf.String()
	// chalk is a no-op in non-TTY, but the color=true branch is exercised.
	assert.Contains(t, output, "new.go")
	assert.Contains(t, output, "package main")
}

// ---------------------------------------------------------------------------
// FormatDiff — unified with MergeUpdate (end-to-end through formatFileUpdate)
// ---------------------------------------------------------------------------

func TestUT_FormatDiff_Unified_UpdatedFile(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{
			Path:        "config.yaml",
			Op:          MergeUpdate,
			OursContent: []byte("port: 8080"),
			Content:     []byte("port: 9090"),
		},
	}

	var buf bytes.Buffer
	FormatDiff(results, "gh:acme/tmpl", "aaa", "bbb", FormatOptions{
		Writer: &buf,
	})

	output := buf.String()
	assert.Contains(t, output, "--- a/config.yaml")
	assert.Contains(t, output, "+++ b/config.yaml")
	assert.Contains(t, output, "-port: 8080")
	assert.Contains(t, output, "+port: 9090")
}

// ---------------------------------------------------------------------------
// FormatDiff — MergePrompt is skipped like MergeKeep
// ---------------------------------------------------------------------------

func TestUT_FormatDiff_SkipsMergePrompt(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "prompt.txt", Op: MergePrompt},
	}

	var buf bytes.Buffer
	FormatDiff(results, "src", "aaa", "bbb", FormatOptions{
		Writer: &buf,
	})

	output := buf.String()
	// The header is always printed, but no file diff should appear.
	assert.NotContains(t, output, "prompt.txt")
}

package templateupdate

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adversarialResult is a SEPARATE fixture from goldenResults() (format_extra_test.go's
// sibling in format_golden_test.go), deliberately containing real non-UTF8
// binary bytes with embedded newlines and real conflict markers in Content.
// The golden fixture leaves both nil, so a naive Summarize implementation
// coincidentally reports 0/0 there and hides a bug that would dump raw bytes
// or count conflict-marker lines as +/- diff lines. It is kept out of the
// text golden fixture (format_extra_test.go) so that file stays readable in
// review.
func adversarialResult() *DiffResult {
	binaryContent := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0xff, 0xfe, 0x0a, 0x00, 0x0a, 0xde, 0xad}

	return &DiffResult{
		OldSHA: "abc1234567890",
		NewSHA: "def0987654321",
		Source: "gh:acme/go-api",
		Results: []MergeResult{
			{Path: "new.go", Op: MergeAdd, Content: []byte("package main\n\nfunc main() {}\n")},
			{Path: "old.go", Op: MergeDelete, BaseContent: []byte("package main\n\n// gone\n")},
			{
				Path:        "svc.go",
				Op:          MergeUpdate,
				OursContent: []byte("const A = 1\nconst B = 2\ndup\ndup\n"),
				Content:     []byte("const A = 1\nconst C = 3\ndup\n"),
			},
			{
				Path:          "config.yaml",
				Op:            MergeConflict,
				Content:       []byte("<<<<<<< ours\nours: true\n=======\ntheirs: true\n>>>>>>> theirs\n"),
				Conflicted:    true,
				OursContent:   []byte("ours: true\n"),
				TheirsContent: []byte("theirs: true\n"),
			},
			{Path: "asset.bin", Op: MergeUpdate, IsBinary: true, Content: binaryContent, OursContent: binaryContent},
			{Path: "removed.bin", Op: MergeDelete, IsBinary: true, BaseContent: binaryContent},
			{Path: "added.bin", Op: MergeAdd, IsBinary: true, Content: binaryContent},
			{Path: "prompted.go", Op: MergePrompt, PromptReason: "you deleted this but template changed it"},
			{Path: "README.md", Op: MergeKeep},
			{Path: "notes.txt", Op: MergeUserAdded},
		},
		Skipped: []string{"vendor/ignored.go"},
	}
}

func TestUT_Summarize_Shape(t *testing.T) {
	t.Parallel()

	summary := Summarize(adversarialResult())
	assert.Equal(t, "abc1234567890", summary.OldSHA)
	assert.Equal(t, "def0987654321", summary.NewSHA)
	assert.Equal(t, "gh:acme/go-api", summary.Source)
	assert.NotEmpty(t, summary.Files)
}

func TestUT_Summarize_OpIsStringNotInteger(t *testing.T) {
	t.Parallel()

	// MergeOp is an int, so a Go-struct field-level assertion on a typed
	// field would compare ints and pass even if the wire carried a bare "3".
	// Assert on the encoded bytes instead.
	result := &DiffResult{Results: []MergeResult{{Path: "f.go", Op: MergeUpdate}}}
	data, err := json.Marshal(Summarize(result))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"op":"update"`)
	assert.NotContains(t, string(data), `"op":3`)
}

func TestUT_Summarize_AllMergeOpsSerialise(t *testing.T) {
	t.Parallel()

	for op := MergeKeep; op <= MergePrompt; op++ {
		result := &DiffResult{Results: []MergeResult{{Path: "f", Op: op}}}
		data, err := json.Marshal(Summarize(result))
		require.NoError(t, err)
		assert.NotContains(t, string(data), `"unknown"`, "MergeOp %d serialised as unknown", op)
	}
}

// TestUT_Summarize_OpInclusionPolicy asserts files includes add/delete/update/
// conflict/prompt and EXCLUDES keep/user-added — deliberately diverging from
// the text formatter, which additionally omits prompt (a pre-existing text
// bug filed separately, see formatUnified).
func TestUT_Summarize_OpInclusionPolicy(t *testing.T) {
	t.Parallel()

	summary := Summarize(adversarialResult())

	ops := make(map[string]bool)
	for _, f := range summary.Files {
		ops[f.Op] = true
	}

	for _, want := range []string{"add", "delete", "update", "conflict", "prompt"} {
		assert.True(t, ops[want], "expected op %q to be included", want)
	}
	for _, unwanted := range []string{"keep", "user-added"} {
		assert.False(t, ops[unwanted], "op %q must be excluded from JSON", unwanted)
	}
}

func TestUT_Summarize_EmptyFilesIsArrayNotNull(t *testing.T) {
	t.Parallel()

	result := &DiffResult{OldSHA: "a", NewSHA: "a", Source: "gh:acme/go-api"}
	data, err := json.Marshal(Summarize(result))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"files":[]`)
	assert.NotContains(t, string(data), `"files":null`)
}

// TestUT_Summarize_CountsMatchTextDiffLines counts the +/- lines FormatDiff
// actually writes for the same MergeResult and requires equality — this is
// what turns "the diff is computed once" from a claim into an observable
// property.
func TestUT_Summarize_CountsMatchTextDiffLines(t *testing.T) {
	t.Parallel()

	result := adversarialResult()
	summary := Summarize(result)

	for _, r := range result.Results {
		if r.Op != MergeUpdate || r.IsBinary {
			continue
		}

		// writeSimpleDiff is the exact call formatFileUpdate makes for the
		// diff body (excluding its "--- a/x / +++ b/x" unified-diff header,
		// which is structural framing, not a counted diff line).
		var buf bytes.Buffer
		writeSimpleDiff(&buf, splitLines(r.OursContent), splitLines(r.Content), false)
		text := buf.String()

		wantAdded := 0
		wantDeleted := 0
		for _, line := range bytesSplitLines(text) {
			switch {
			case line != "" && line[0] == '+':
				wantAdded++
			case line != "" && line[0] == '-':
				wantDeleted++
			}
		}

		var got *DiffFileSummary
		for i := range summary.Files {
			if summary.Files[i].Path == r.Path {
				got = &summary.Files[i]
				break
			}
		}
		require.NotNil(t, got, "path %s missing from summary", r.Path)
		assert.Equal(t, wantAdded, got.Added, "path %s: added count drifted from text formatter", r.Path)
		assert.Equal(t, wantDeleted, got.Deleted, "path %s: deleted count drifted from text formatter", r.Path)
	}
}

// bytesSplitLines splits text formatter output into lines for counting,
// without pulling in the formatter's own splitLines (which operates on raw
// file content, not already-rendered "+"/"-" prefixed output lines).
func bytesSplitLines(s string) []string {
	var lines []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func TestUT_Summarize_BinaryFileNotDumped(t *testing.T) {
	t.Parallel()

	summary := Summarize(adversarialResult())

	for _, path := range []string{"asset.bin", "removed.bin", "added.bin"} {
		var got *DiffFileSummary
		for i := range summary.Files {
			if summary.Files[i].Path == path {
				got = &summary.Files[i]
			}
		}
		require.NotNil(t, got, "path %s missing", path)
		assert.True(t, got.IsBinary)
		assert.Zero(t, got.Added, "binary file %s must report 0 added", path)
		assert.Zero(t, got.Deleted, "binary file %s must report 0 deleted", path)
	}

	data, err := json.Marshal(summary)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "\xde\xad", "raw binary bytes must never reach the JSON output")
}

func TestUT_Summarize_ConflictCountsExcludeMarkers(t *testing.T) {
	t.Parallel()

	summary := Summarize(adversarialResult())

	var got *DiffFileSummary
	for i := range summary.Files {
		if summary.Files[i].Path == "config.yaml" {
			got = &summary.Files[i]
		}
	}
	require.NotNil(t, got)
	assert.True(t, got.Conflicted)
	assert.Zero(t, got.Added, "a conflict must report 0 added, not a count of marker lines")
	assert.Zero(t, got.Deleted, "a conflict must report 0 deleted, not a count of marker lines")
}

// TestUT_Summarize_OmitsFileContents is a sentinel: the JSON shape has no
// field carrying file content at all, so this simply documents that
// expectation by checking the marker bytes any content field would leak.
func TestUT_Summarize_OmitsFileContents(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(Summarize(adversarialResult()))
	require.NoError(t, err)
	s := string(data)
	assert.NotContains(t, s, "package main")
	assert.NotContains(t, s, "<<<<<<<")
	assert.NotContains(t, s, "ours: true")
}

// TestUT_Summarize_CountsMatchTextDiffLinesAllOps is the all-operations
// counterpart to TestUT_Summarize_CountsMatchTextDiffLines above, which walks
// only MergeUpdate. That gap was not theoretical: Summarize originally counted
// a MergeAdd with splitLines, whose nil guard returns zero, while
// formatFileAdd splits r.Content unguarded and therefore prints one bare "+"
// for nil content. Text said 1, JSON said 0, and no test could see it.
//
// Each result is formatted alone so the +/- lines can be attributed to it, and
// the unified-diff header lines ("--- a/x", "+++ b/x", "--- /dev/null") are
// excluded — they are structural framing, not counted diff lines.
func TestUT_Summarize_CountsMatchTextDiffLinesAllOps(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		result MergeResult
	}{
		{"add nil content", MergeResult{Path: "a.go", Op: MergeAdd}},
		{"add empty content", MergeResult{Path: "b.go", Op: MergeAdd, Content: []byte{}}},
		{"add no trailing newline", MergeResult{Path: "c.go", Op: MergeAdd, Content: []byte("one")}},
		{"add trailing newline", MergeResult{Path: "d.go", Op: MergeAdd, Content: []byte("one\ntwo\n")}},
		{"delete nil base", MergeResult{Path: "e.go", Op: MergeDelete}},
		{"delete with base", MergeResult{Path: "f.go", Op: MergeDelete, BaseContent: []byte("gone\n")}},
		{
			"update with duplicates",
			MergeResult{
				Path:        "g.go",
				Op:          MergeUpdate,
				OursContent: []byte("a\ndup\ndup\nb\n"),
				Content:     []byte("a\ndup\nc\n"),
			},
		},
		{"update nil sides", MergeResult{Path: "h.go", Op: MergeUpdate}},
		{"conflict", MergeResult{Path: "i.go", Op: MergeConflict, Conflicted: true}},
		{"prompt", MergeResult{Path: "j.go", Op: MergePrompt}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			FormatDiff([]MergeResult{tc.result}, "src", "old", "new", FormatOptions{Writer: &buf})

			wantAdded, wantDeleted := 0, 0
			for _, line := range bytesSplitLines(buf.String()) {
				switch {
				case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
					// unified-diff header framing, not a diff line
				case strings.HasPrefix(line, "+"):
					wantAdded++
				case strings.HasPrefix(line, "-"):
					wantDeleted++
				}
			}

			files := Summarize(&DiffResult{Results: []MergeResult{tc.result}}).Files
			require.Len(t, files, 1)
			assert.Equal(t, wantAdded, files[0].Added, "added drifted from the text formatter")
			assert.Equal(t, wantDeleted, files[0].Deleted, "deleted drifted from the text formatter")
		})
	}
}

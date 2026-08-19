package templateupdate

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// updateGoldenDiff rewrites the diff golden fixtures instead of asserting
// against them. Run with:
//
//	go test ./internal/templateupdate -run TestUT_FormatDiffGolden -update-golden
//
// Provenance: these fixtures were captured in the baseline commit of the #351
// branch, BEFORE FormatDiff was touched, so they record main's bytes rather
// than the refactor's own output. Regenerating one to make a failing test pass
// launders the exact regression they exist to catch — recapture from the
// previous release instead, and say so in the commit message.
var updateGoldenDiff = flag.Bool("update-golden", false, "rewrite diff golden fixtures")

// goldenResults is the fixture merge set: one file per interesting MergeOp,
// including the two the text formatter deliberately prints nothing for.
func goldenResults() []MergeResult {
	return []MergeResult{
		{Path: "cmd/main.go", Op: MergeAdd, Content: []byte("package main\n\nfunc main() {}\n")},
		{Path: "docs/old.md", Op: MergeDelete, BaseContent: []byte("# Old\ngone\n")},
		{
			// Duplicated, reordered, blank and trailing-newline lines are all
			// present deliberately: writeSimpleDiff matches lines as a
			// multiset (removals in old-file order, then additions in
			// new-file order), and a refactor that dedupes, reorders or
			// interleaves them is exactly the drift this fixture exists to
			// catch.
			Path:        "internal/svc.go",
			Op:          MergeUpdate,
			OursContent: []byte("package svc\n\nconst A = 1\nconst B = 2\ndup\ndup\ndup\ntail\n"),
			Content:     []byte("tail\npackage svc\n\nconst C = 3\nconst A = 1\ndup\nconst D = 4\n"),
		},
		{Path: "config.yaml", Op: MergeConflict, Conflicted: true},
		{Path: "logo.png", Op: MergeUpdate, IsBinary: true},
		{Path: "README.md", Op: MergeKeep},
		{Path: "notes.txt", Op: MergeUserAdded},
	}
}

func TestUT_FormatDiffGolden(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		opts FormatOptions
	}{
		{"diff-unified-plain", FormatOptions{}},
		{"diff-unified-color", FormatOptions{Color: true}},
		{"diff-stat-plain", FormatOptions{Stat: true}},
		{"diff-stat-color", FormatOptions{Stat: true, Color: true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			opts := tc.opts
			opts.Writer = &buf
			FormatDiff(goldenResults(), "gh:acme/go-api", "abc1234567890", "def0987654321", opts)
			assertDiffGolden(t, tc.name, buf.String())
		})
	}
}

func assertDiffGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", "golden", name+".txt")
	if *updateGoldenDiff {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
		return
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden fixture %s — regenerate with -update-golden", path)
	require.Equal(t, string(want), got, "diff text output drifted from the golden fixture")
}

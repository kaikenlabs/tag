package templateupdate

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/jsonout"
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

// TestUT_FormatDiffGolden pins the text output of FormatDiff, including the
// two Color:true cases. What those two DO prove: the Color:true code branch
// runs without panicking or otherwise diverging in content/structure from
// its plain twin. What they do NOT prove: that anything is actually
// colorized — chalk.isTerminal() reads the real process os.Stdout, which is
// never a TTY under `go test`, so chalk.Red/Green/Cyan/Yellow are no-ops here
// and the "-color" fixtures are byte-identical to their plain counterparts.
// Do not cite these as color-output coverage; there isn't any in this suite.
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

// TestUT_SummarizeJSONGolden pins the ENCODED bytes of the --format json diff
// contract against a static fixture.
//
// The hand-written assertions in summary_test.go check the fields they know to
// look for; this one catches the fields nobody thought to check. Concretely it
// is what would fail if MergeResult's Content/BaseContent leaked in as base64
// bodies, or if Mode (an os.FileMode) started serialising as a bare integer —
// neither of which any field-by-field assertion would notice, and neither of
// which a command-level test can reach today, because no test can drive a
// non-empty diff through diffAction without network.
//
// It reuses goldenResults() so the JSON contract and the text contract are
// pinned against the same merge set, which is what makes them comparable.
//
// PROVENANCE, which differs from every other fixture in this directory: the
// diff-*.txt text fixtures were captured from unmodified source and are a
// regression control against main. diff-json.txt cannot be — the JSON path did
// not exist on main. It is a forward schema lock: a change to it is a change to
// a published contract, and must be reviewed as one rather than regenerated.
func TestUT_SummarizeJSONGolden(t *testing.T) {
	t.Parallel()

	summary := Summarize(&DiffResult{
		OldSHA:  "abc1234567890",
		NewSHA:  "def0987654321",
		Source:  "gh:acme/go-api",
		Results: goldenResults(),
		Skipped: []string{"ignored/by-rule.txt"},
	})

	// Encoded through jsonout.Write, not a hand-rolled encoder: that package
	// exists so the wire policy (indent, trailing newline, HTML escaping) is
	// decided in exactly one place. A golden built with its own encoder would
	// keep passing if that policy changed, and would then no longer describe
	// what `tag diff --format json` actually emits.
	var buf bytes.Buffer
	require.NoError(t, jsonout.Write(&buf, summary))

	assertDiffGolden(t, "diff-json", buf.String())
}

package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/templateupdate"
)

// fakeTemplateUpdater is a test double for templateUpdater (P2): the
// production chain (env auth -> GitFetcher -> HistoricalRenderer -> Updater)
// needs a real git remote, so this seam is what makes update's success path
// reachable from a unit test at all.
type fakeTemplateUpdater struct {
	result *templateupdate.UpdateResult
	err    error
}

func (f fakeTemplateUpdater) Update(context.Context, templateupdate.UpdateOptions) (*templateupdate.UpdateResult, error) {
	return f.result, f.err
}

// stubTemplateUpdater substitutes the package-level newTemplateUpdater var.
// Mutates package state: callers must not use t.Parallel.
func stubTemplateUpdater(t *testing.T, result *templateupdate.UpdateResult, err error) {
	t.Helper()
	orig := newTemplateUpdater
	newTemplateUpdater = func() templateUpdater { return fakeTemplateUpdater{result: result, err: err} }
	t.Cleanup(func() { newTemplateUpdater = orig })
}

// TestUT_UpdateJSON_UpToDate exercises the "no changes" branch of apply mode:
// JSON is written unconditionally (unlike text's "Already up to date."
// sentinel line), with up_to_date true and an empty file list.
func TestUT_UpdateJSON_UpToDate(t *testing.T) {
	stubTemplateUpdater(t, &templateupdate.UpdateResult{OldSHA: "abc1234", NewSHA: "abc1234"}, nil)

	run := runCLICapturingAll(t, UpdateTemplateCommand(), "update", "--format", "json")
	require.NoError(t, run.Err)

	var doc updateDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	assert.Equal(t, "apply", doc.Mode)
	assert.True(t, doc.UpToDate)
	assert.Equal(t, "abc1234", doc.OldSHA)
	assert.Equal(t, "abc1234", doc.NewSHA)
	assert.Empty(t, doc.Files)
	assert.Nil(t, doc.Conflicts)
}

// TestUT_UpdateJSON_AppliedFilesAndCounts exercises the success path with
// real changes applied: Op is MergeOp.String() verbatim (the epic decision
// to keep MergeOp rather than mapping onto fileaction.Action), and the whole
// []updateFileJSON is asserted by equality.
func TestUT_UpdateJSON_AppliedFilesAndCounts(t *testing.T) {
	stubTemplateUpdater(t, &templateupdate.UpdateResult{
		OldSHA: "aaa1111",
		NewSHA: "bbb2222",
		Applied: []templateupdate.MergeResult{
			{Path: "a.txt", Op: templateupdate.MergeAdd},
			{Path: "b.txt", Op: templateupdate.MergeUpdate},
			{Path: "c.txt", Op: templateupdate.MergeDelete},
			{Path: "d.txt", Op: templateupdate.MergeKeep},
		},
		NewFiles:     1,
		UpdatedFiles: 1,
		DeletedFiles: 1,
	}, nil)

	run := runCLICapturingAll(t, UpdateTemplateCommand(), "update", "--format", "json")
	require.NoError(t, run.Err)

	var doc updateDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	assert.Equal(t, "apply", doc.Mode)
	assert.False(t, doc.UpToDate)
	want := []updateFileJSON{
		{Path: "a.txt", Op: "add"},
		{Path: "b.txt", Op: "update"},
		{Path: "c.txt", Op: "delete"},
		{Path: "d.txt", Op: "keep"},
	}
	assert.Equal(t, want, doc.Files)
	assert.Equal(t, 1, doc.NewFiles)
	assert.Equal(t, 1, doc.UpdatedFiles)
	assert.Equal(t, 1, doc.DeletedFiles)
	assert.Nil(t, doc.Conflicts)
}

// TestUT_UpdateJSON_ConflictWritesDocumentAndExitCode exercises D5 for
// update: a conflict writes the document (conflicts populated) AND still
// exits non-zero.
func TestUT_UpdateJSON_ConflictWritesDocumentAndExitCode(t *testing.T) {
	report := templateupdate.NewConflictReport([]templateupdate.MergeResult{
		{Path: "x.go", Op: templateupdate.MergeConflict, Content: []byte("<<<<<<<\n")},
		{Path: "y.go", Op: templateupdate.MergePrompt, PromptReason: "binary conflict"},
	}, []string{"z.go"})

	stubTemplateUpdater(t, &templateupdate.UpdateResult{
		OldSHA:    "aaa1111",
		NewSHA:    "bbb2222",
		Applied:   []templateupdate.MergeResult{{Path: "x.go", Op: templateupdate.MergeConflict}},
		Conflicts: report,
	}, nil)

	run := runCLICapturingAll(t, UpdateTemplateCommand(), "update", "--format", "json")
	require.Error(t, run.Err)

	type exitCoder interface{ ExitCode() int }
	ec, ok := run.Err.(exitCoder)
	require.True(t, ok)
	assert.Equal(t, 1, ec.ExitCode())

	var doc updateDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc), "a document must still be written on conflict")
	require.NotNil(t, doc.Conflicts)
	assert.Equal(t, []string{"x.go"}, doc.Conflicts.ConflictedFiles)
	assert.Equal(t, []string{"y.go"}, doc.Conflicts.PromptFiles)
	assert.Equal(t, []string{"z.go"}, doc.Conflicts.Skipped)
}

// TestUT_UpdateJSON_NeverLeaksFileContent is the mandated redaction test:
// UpdateResult.Applied[].Content and every ConflictedFile content field
// (Base/Ours/Theirs/MergedContent) carry the user's own source code and must
// never reach jsonout.Write. Sentinels are planted in all five places and the
// raw document bytes are asserted not to contain any of them.
func TestUT_UpdateJSON_NeverLeaksFileContent(t *testing.T) {
	const sentinel = "SUPER_SECRET_FILE_BODY_MUST_NOT_LEAK"

	report := &templateupdate.ConflictReport{
		Conflicts: []templateupdate.ConflictedFile{
			{
				Path:          "conflicted.go",
				BaseContent:   []byte(sentinel + "-base"),
				OursContent:   []byte(sentinel + "-ours"),
				TheirsContent: []byte(sentinel + "-theirs"),
				MergedContent: []byte(sentinel + "-merged"),
			},
		},
	}

	stubTemplateUpdater(t, &templateupdate.UpdateResult{
		OldSHA: "aaa1111",
		NewSHA: "bbb2222",
		Applied: []templateupdate.MergeResult{
			{Path: "applied.go", Op: templateupdate.MergeUpdate, Content: []byte(sentinel + "-applied")},
		},
		Conflicts: report,
	}, nil)

	run := runCLICapturingAll(t, UpdateTemplateCommand(), "update", "--format", "json")
	require.Error(t, run.Err) // conflict present -> non-zero exit, still wrote a document

	require.NotEmpty(t, run.Writer)
	assert.NotContains(t, run.Writer, sentinel)
}

// TestUT_UpdateJSON_AbortMode asserts abort emits the SAME key set as every
// other mode, with an empty file list and zero counters — abort really does
// apply no files. One stable shape means a consumer never branches on "mode"
// to know whether a key exists. Whole-key-set comparison so a drift either way
// is caught.
func TestUT_UpdateJSON_AbortMode(t *testing.T) {
	stubTemplateUpdater(t, &templateupdate.UpdateResult{}, nil)

	run := runCLICapturingAll(t, UpdateTemplateCommand(), "update", "--abort", "--format", "json")
	require.NoError(t, run.Err)

	var raw map[string]any
	dec := json.NewDecoder(bytes.NewReader([]byte(run.Writer)))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(&raw))

	got := slices.Sorted(maps.Keys(raw))
	assert.Equal(t, []string{
		"deleted_files", "dry_run", "files", "mode",
		"new_files", "up_to_date", "updated_files",
	}, got)
	assert.Equal(t, "abort", raw["mode"])
	assert.Equal(t, []any{}, raw["files"])
}

// TestUT_UpdateJSON_ContinueMode asserts continue's document carries the
// SHAs but no file list — continueUpdate never populates Applied.
func TestUT_UpdateJSON_ContinueMode(t *testing.T) {
	stubTemplateUpdater(t, &templateupdate.UpdateResult{OldSHA: "aaa1111", NewSHA: "bbb2222"}, nil)

	run := runCLICapturingAll(t, UpdateTemplateCommand(), "update", "--continue", "--format", "json")
	require.NoError(t, run.Err)

	var raw map[string]any
	dec := json.NewDecoder(bytes.NewReader([]byte(run.Writer)))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(&raw))

	got := slices.Sorted(maps.Keys(raw))
	assert.Equal(t, []string{
		"deleted_files", "dry_run", "files", "mode",
		"new_files", "new_sha", "old_sha", "up_to_date", "updated_files",
	}, got)
	assert.Equal(t, "continue", raw["mode"])
	assert.Equal(t, []any{}, raw["files"])
	assert.Equal(t, "bbb2222", raw["new_sha"])
}

// TestUT_UpdateJSON_StrayPositionalIsUsageError mirrors diffAction's guard:
// update cannot use reparseTrailingFlags (nothing to reparse into), so a
// stray positional must be rejected directly rather than silently falling
// back to text because urfave/cli stopped parsing at that token.
func TestUT_UpdateJSON_StrayPositionalIsUsageError(t *testing.T) {
	run := runCLICapturingAll(t, UpdateTemplateCommand(), "update", "stray", "--format", "json")
	require.Error(t, run.Err)
	assert.Contains(t, run.Err.Error(), "does not accept positional arguments")
}

// TestUT_UpdateJSON_DryRunReflectsFlag verifies dry_run in the document
// tracks the --dry-run flag rather than being hardcoded.
func TestUT_UpdateJSON_DryRunReflectsFlag(t *testing.T) {
	stubTemplateUpdater(t, &templateupdate.UpdateResult{OldSHA: "aaa1111", NewSHA: "bbb2222"}, nil)

	run := runCLICapturingAll(t, UpdateTemplateCommand(), "update", "--dry-run", "--format", "json")
	require.NoError(t, run.Err)

	var doc updateDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	assert.True(t, doc.DryRun)
}

// TestUT_UpdateJSON_ExactlyOneDocumentAndNothingOnBypassedSinks decodes
// stdout then requires io.EOF (Contains("{") cannot see a prepended summary
// line or two concatenated documents), and asserts nothing bypassed
// c.App.Writer to the real os.Stdout.
func TestUT_UpdateJSON_ExactlyOneDocumentAndNothingOnBypassedSinks(t *testing.T) {
	stubTemplateUpdater(t, &templateupdate.UpdateResult{
		OldSHA: "aaa1111",
		NewSHA: "bbb2222",
		Applied: []templateupdate.MergeResult{
			{Path: "a.txt", Op: templateupdate.MergeAdd},
		},
		NewFiles: 1,
	}, nil)

	run := runCLICapturingAll(t, UpdateTemplateCommand(), "update", "--format", "json")
	require.NoError(t, run.Err)
	require.Empty(t, run.Stdout, "nothing should bypass c.App.Writer to the real os.Stdout")

	dec := json.NewDecoder(bytes.NewReader([]byte(run.Writer)))
	var doc updateDoc
	require.NoError(t, dec.Decode(&doc))
	_, eofErr := dec.Token()
	require.ErrorIs(t, eofErr, io.EOF, "exactly one JSON document must be on the wire")
}

// TestUT_UpdateJSON_ApplyKeysAreStableWhenNothingChanged pins the one shape a
// consumer actually has to survive: an apply run that merged no files.
//
// With `omitempty` on files and the counters, such a run emitted only
// {"mode","dry_run","old_sha","new_sha"} — no "files" key at all, so
// `doc.files.length` is a TypeError rather than 0, and "updated_files": 0 is
// indistinguishable from "the field does not exist". #354 asks for empty
// slices to serialise as [], and a key that vanishes entirely is strictly
// worse than the `null` that criterion was written to prevent.
//
// abort and continue keep their minimal shape deliberately (no file work
// happens in those modes); this is about apply, which is what a script runs.
func TestUT_UpdateJSON_ApplyKeysAreStableWhenNothingChanged(t *testing.T) {
	stubTemplateUpdater(t, &templateupdate.UpdateResult{
		OldSHA: "aaa1111",
		NewSHA: "bbb2222",
	}, nil)

	run := runCLICapturingAll(t, UpdateTemplateCommand(), "update", "--format", "json")
	require.NoError(t, run.Err)

	var raw map[string]any
	dec := json.NewDecoder(bytes.NewReader([]byte(run.Writer)))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(&raw))

	got := slices.Sorted(maps.Keys(raw))
	assert.Equal(t, []string{
		"deleted_files", "dry_run", "files", "mode",
		"new_files", "new_sha", "old_sha", "up_to_date", "updated_files",
	}, got, "an apply document must carry a stable key set")

	assert.Equal(t, []any{}, raw["files"], "an empty file list must be [], not absent")
	assert.InDelta(t, 0.0, raw["updated_files"], 0.0)
	assert.Equal(t, false, raw["up_to_date"])
}

// TestUT_UpdateJSON_FileEntryCarriesConflictedAndBinary pins the three
// per-file fields #354's shape names beyond path/op.
//
// "conflicted" is NOT derivable from "op": NewConflictReport classifies a file
// as conflicted when `mr.Op == MergeConflict || mr.Conflicted`, so a result can
// carry Conflicted=true under some other op and a consumer reading only "op"
// would miss it. "is_binary" tells a consumer the file cannot be diffed or
// merged as text. Both are bounded scalars — unlike MergeResult.Content, which
// must never be serialised.
func TestUT_UpdateJSON_FileEntryCarriesConflictedAndBinary(t *testing.T) {
	stubTemplateUpdater(t, &templateupdate.UpdateResult{
		OldSHA: "aaa1111",
		NewSHA: "bbb2222",
		Applied: []templateupdate.MergeResult{
			{Path: "clean.go", Op: templateupdate.MergeUpdate},
			{Path: "logo.png", Op: templateupdate.MergeUpdate, IsBinary: true},
			{Path: "odd.go", Op: templateupdate.MergeUpdate, Conflicted: true},
			{Path: "ask.go", Op: templateupdate.MergePrompt, PromptReason: "user deleted, template changed"},
		},
	}, nil)

	run := runCLICapturingAll(t, UpdateTemplateCommand(), "update", "--format", "json")
	require.NoError(t, run.Err)

	var doc struct {
		Files []struct {
			Path         string `json:"path"`
			Op           string `json:"op"`
			Conflicted   bool   `json:"conflicted"`
			IsBinary     bool   `json:"is_binary"`
			PromptReason string `json:"prompt_reason"`
		} `json:"files"`
	}
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	require.Len(t, doc.Files, 4)

	assert.False(t, doc.Files[0].Conflicted)
	assert.False(t, doc.Files[0].IsBinary)
	assert.True(t, doc.Files[1].IsBinary, "a binary merge must be flagged")
	assert.True(t, doc.Files[2].Conflicted, "Conflicted must survive independently of op")
	assert.Equal(t, "prompt", doc.Files[3].Op)
	assert.Equal(t, "user deleted, template changed", doc.Files[3].PromptReason)
}

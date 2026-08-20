package commands

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/templateupdate"
	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

// TestUT_UpdateTextGolden pins `tag update`'s human output byte-for-byte.
//
// PROVENANCE: these six fixtures were captured by running the UNMODIFIED `main`
// in a git worktree. Only the updater construction was swapped there (for a
// stub, so the command could run without a real git remote) — not one print
// statement was touched, so the recorded bytes are main's. That mattered
// because #354 moves ~19 os.Stdout writes onto c.App.Writer, and until this
// test existed nothing pinned the abort, continue, "Already up to date.",
// variable-change, hook-change, conflict or completion lines. The pre-existing
// update-summary fixture covered only printUpdateSummary, which is four of
// them.
//
// The stub results below MUST stay identical to the ones used at capture time,
// or these fixtures pin nothing. If one fails, the output changed — do NOT
// rerun with -update-golden.
func TestUT_UpdateTextGolden(t *testing.T) {
	// richUpdateResult is the capture-time fixture: one file per MergeOp that
	// prints (and one, MergeKeep, that deliberately prints nothing), plus a
	// variable change of each direction and an added hook.
	richUpdateResult := func() *templateupdate.UpdateResult {
		return &templateupdate.UpdateResult{
			OldSHA: "abc1234567890def",
			NewSHA: "def4567890abc123",
			Applied: []templateupdate.MergeResult{
				{Path: "cmd/main.go", Op: templateupdate.MergeUpdate},
				{Path: "internal/new.go", Op: templateupdate.MergeAdd},
				{Path: "internal/old.go", Op: templateupdate.MergeDelete},
				{Path: "README.md", Op: templateupdate.MergeKeep},
			},
			VarChanges: []templateupdate.VarChange{
				{Name: "author", Type: templateupdate.VarAdded, NewDef: &tmplconfig.VariableDef{Type: "string", Default: "unset"}},
				{Name: "legacy", Type: templateupdate.VarRemoved, OldDef: &tmplconfig.VariableDef{Type: "string"}},
			},
			HookChanges: []templateupdate.HookChange{
				{Phase: "post_scaffold", Type: templateupdate.HookAdded, NewHooks: []string{"make fmt"}},
			},
			NewFiles:     1,
			UpdatedFiles: 1,
			DeletedFiles: 1,
		}
	}

	conflicted := richUpdateResult()
	conflicted.Applied = append(conflicted.Applied,
		templateupdate.MergeResult{Path: "conf.go", Op: templateupdate.MergeConflict, Conflicted: true})
	conflicted.Conflicts = &templateupdate.ConflictReport{
		Conflicts: []templateupdate.ConflictedFile{{Path: "conf.go", MarkerCount: 3}},
		Skipped:   []string{"vendor/x.go"},
	}

	tests := []struct {
		name    string
		result  *templateupdate.UpdateResult
		argv    []string
		wantErr bool
		fixture string
	}{
		{name: "abort", result: &templateupdate.UpdateResult{}, argv: []string{"update", "--abort"}, fixture: "update-abort"},
		{name: "continue", result: &templateupdate.UpdateResult{NewSHA: "def4567890abc123"}, argv: []string{"update", "--continue"}, fixture: "update-continue"},
		{name: "up-to-date", result: &templateupdate.UpdateResult{OldSHA: "same111", NewSHA: "same111"}, argv: []string{"update"}, fixture: "update-up-to-date"},
		{name: "apply", result: richUpdateResult(), argv: []string{"update"}, fixture: "update-apply"},
		{name: "dry-run", result: richUpdateResult(), argv: []string{"update", "--dry-run"}, fixture: "update-dry-run"},
		// A conflict still exits non-zero after printing.
		{name: "conflict", result: conflicted, argv: []string{"update"}, wantErr: true, fixture: "update-conflict"},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			stubTemplateUpdater(t, tt.result, nil)

			run := runCLICapturingAll(t, UpdateTemplateCommand(), tt.argv...)
			if tt.wantErr {
				require.Error(t, run.Err)
			} else {
				require.NoError(t, run.Err)
			}

			// os.Stdout must be empty now that update writes through the
			// injected writer — that emptiness is #354's AC-12, and asserting
			// on run.All() alone could not distinguish the two sinks.
			require.Empty(t, run.Stdout, "update must not write to os.Stdout directly")

			assertGolden(t, tt.fixture, run.All())
		})
	}
}

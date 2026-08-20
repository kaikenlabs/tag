package commands

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/convert"
	"github.com/kaikenlabs/tag/internal/history"
	"github.com/kaikenlabs/tag/internal/templateupdate"
)

// TestUT_ActionTextGolden pins the text output of the six action commands
// (#353/#354/#355) byte-for-byte, so that adding --format json to them cannot
// silently reword, reorder, or drop a line of the human-facing output.
//
// PROVENANCE (see the header of golden_text_test.go for why this matters):
// every fixture in this file was captured in the FIRST commit on branch
// feat/json-action-commands, before any source file was touched by those three
// stories. Regenerating one of these from a later working tree launders exactly
// the regression it exists to catch — if a fixture fails, the change is wrong.
//
// The fixtures deliberately use relative paths and fixed timestamps so no
// t.TempDir() path leaks into the recorded bytes.
//
// One correction, worth recording because it is the only fixture in this file
// whose bytes were changed after capture: extract-summary was first captured
// against a test App that declared NO flags, so the global --path flag was
// unregistered and c.String(PathFlag) read back as "" instead of its real
// default ".tag" — the fixture recorded "Extracted template: handler/..." which
// the shipped binary never prints. It was corrected to the output of an actual
// `tag extract` run of the unmodified binary (".tag/handler/..."), and the test
// App now builds its flags from commands.GlobalFlags(), the same list main.go
// uses, so the harness cannot drift from the binary this way again.
func TestUT_ActionTextGolden(t *testing.T) {
	t.Run("undo-list", func(t *testing.T) {
		dir := seedUndoProject(t)
		require.NoError(t, history.Append(filepath.Join(dir, ".tag"), history.Generation{
			ID:        "gen_1_aaa",
			Timestamp: time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
			Template:  "model",
			Command:   "generate",
			Files: []history.FileEntry{
				{Path: "handler.go", Action: history.ActionCreate, HashAfter: "sha256:abc"},
			},
		}))

		run := runCLICapturingStdout(t, UndoCommand(), "undo", "--list")
		require.NoError(t, run.Err)
		assertGolden(t, "undo-list", run.All())
	})

	t.Run("undo-list-empty", func(t *testing.T) {
		seedUndoProject(t)
		run := runCLICapturingStdout(t, UndoCommand(), "undo", "--list")
		require.NoError(t, run.Err)
		assertGolden(t, "undo-list-empty", run.All())
	})

	// No --yes: the preview prints, then promptConfirm takes the
	// non-interactive branch and cancels. This is the exact path a JSON
	// consumer would otherwise hit, so its wording is worth pinning.
	//
	// stdin is forced non-TTY: promptConfirm branches on isTerminal(), so
	// without this the fixture records the interactive "Proceed? [y/N]"
	// wording when the developer runs `go test` from a terminal and the
	// non-interactive wording in CI. Same test, two different bytes.
	t.Run("undo-preview-cancelled", func(t *testing.T) {
		withNonTTYStdin(t)
		dir := seedUndoProject(t)
		require.NoError(t, history.Append(filepath.Join(dir, ".tag"), history.Generation{
			ID:       "gen_1_aaa",
			Template: "model",
			Command:  "generate",
			Files: []history.FileEntry{
				{Path: "handler.go", Action: history.ActionCreate, HashAfter: "sha256:abc"},
			},
		}))

		run := runCLICapturingStdout(t, UndoCommand(), "undo")
		require.NoError(t, run.Err)
		assertGolden(t, "undo-preview-cancelled", run.All())
	})

	t.Run("undo-revert", func(t *testing.T) {
		dir := seedUndoProject(t)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "handler.go"), []byte("package main\n"), 0o600))
		require.NoError(t, history.Append(filepath.Join(dir, ".tag"), history.Generation{
			ID:       "gen_1_aaa",
			Template: "model",
			Command:  "generate",
			Files: []history.FileEntry{
				{Path: "handler.go", Action: history.ActionCreate, HashAfter: history.HashBytes([]byte("package main\n"))},
			},
		}))

		run := runCLICapturingStdout(t, UndoCommand(), "undo", "--yes")
		require.NoError(t, run.Err)
		assertGolden(t, "undo-revert", run.All())
	})

	t.Run("undo-conflict", func(t *testing.T) {
		dir := seedUndoProject(t)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "handler.go"), []byte("user modified\n"), 0o600))
		require.NoError(t, history.Append(filepath.Join(dir, ".tag"), history.Generation{
			ID:       "gen_1_aaa",
			Template: "model",
			Command:  "generate",
			Files: []history.FileEntry{
				{Path: "handler.go", Action: history.ActionCreate, HashAfter: history.HashBytes([]byte("original\n"))},
			},
		}))

		run := runCLICapturingStdout(t, UndoCommand(), "undo", "--yes")
		require.Error(t, run.Err)
		assertGolden(t, "undo-conflict", run.All())
	})

	t.Run("convert-summary", func(t *testing.T) {
		var sb testWriter
		printConversionResult(&sb, richConvertResult(false))
		assertGolden(t, "convert-summary", sb.String())
	})

	t.Run("convert-summary-dry-run", func(t *testing.T) {
		var sb testWriter
		printConversionResult(&sb, richConvertResult(true))
		assertGolden(t, "convert-summary-dry-run", sb.String())
	})

	// printUpdateSummary writes to os.Stdout directly today; #354 moves it onto
	// c.App.Writer. Capturing stdout here is what makes that move provably
	// output-preserving.
	t.Run("update-summary", func(t *testing.T) {
		got := captureStdout(t, func() {
			printUpdateSummary(&templateupdate.UpdateResult{
				Applied: []templateupdate.MergeResult{
					{Path: "a.txt", Op: templateupdate.MergeAdd},
					{Path: "b.txt", Op: templateupdate.MergeUpdate},
					{Path: "c.txt", Op: templateupdate.MergeDelete},
					{Path: "d.txt", Op: templateupdate.MergeConflict},
					{Path: "e.txt", Op: templateupdate.MergeKeep},
				},
			})
		})
		assertGolden(t, "update-summary", got)
	})

	t.Run("extract-summary", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		require.NoError(t, os.WriteFile("user_handler.go", []byte(
			"package handler\n\ntype UserHandler struct{}\n\nfunc NewUserHandler() *UserHandler { return &UserHandler{} }\n",
		), 0o600))

		run := runCLICapturingStdout(t, ExtractCommand(),
			"extract", "--name", "user", "--as", "handler", "user_handler.go")
		require.NoError(t, run.Err)
		assertGolden(t, "extract-summary", run.All())
	})
}

// withNonTTYStdin points os.Stdin at a pipe for the duration of the test, so
// isTerminal() reports false regardless of how the suite was launched. It
// replaces a process global, so a test using it must NOT call t.Parallel.
func withNonTTYStdin(t *testing.T) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		_ = w.Close()
		_ = r.Close()
	})
}

// seedUndoProject creates a temp project with an empty .tag/ directory and
// chdirs into it, so history entries can use relative paths and no temp-dir
// path leaks into a golden fixture.
func seedUndoProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag"), 0o750))
	t.Chdir(dir)
	return dir
}

func richConvertResult(dryRun bool) *convert.Result {
	return &convert.Result{
		Source:             "./cookiecutter-demo",
		Destination:        "/tmp/output",
		VariablesConverted: 7,
		DirsRenamed:        2,
		FilesRenamed:       3,
		FilesProcessed:     12,
		HooksCopied:        1,
		DryRun:             dryRun,
		Incompatibilities: []convert.Incompatibility{{
			Path:       "src/main.py",
			Line:       4,
			Kind:       "filter-syntax",
			Message:    "Jinja2 filter has no Gonja equivalent",
			Original:   "{{ cookiecutter.name|jsonify }}",
			Suggestion: "{{ vars.name | tojson }}",
			Severity:   convert.SeverityWarning,
		}},
		Warnings: []string{"hooks/pre_gen_project.py must be reviewed by hand"},
	}
}

// testWriter is a minimal io.Writer that accumulates into a string, avoiding a
// bytes.Buffer import collision with the other golden test file's helpers.
type testWriter struct{ b []byte }

func (w *testWriter) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }
func (w *testWriter) String() string              { return string(w.b) }

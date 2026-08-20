package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/fileaction"
)

// TestUT_GenerateJSON_InjectAndAppendStayDistinct is the #352 lesson applied
// to the new JSON surface: FileOpDetail.DisplayOp() deliberately collapses
// ActionAppend and ActionInject to the single text word "modified", and
// neither the compiler nor `exhaustive` (disabled in .golangci.yaml) can see
// a regression that re-collapses them in the JSON action field. This asserts
// the WHOLE []generateFileJSON slice by equality, not DisplayOp() and not
// Contains, so a re-collapse fails loudly here.
func TestUT_GenerateJSON_InjectAndAppendStayDistinct(t *testing.T) {
	result := engine.GenerateResult{
		Created:     1,
		Overwritten: 1,
		Modified:    2,
		Details: []engine.FileOpDetail{
			{Path: "a.go", Action: fileaction.ActionCreate},
			{Path: "b.go", Action: fileaction.ActionAppend},
			{Path: "c.go", Action: fileaction.ActionInject},
			{Path: "d.go", Action: fileaction.ActionOverwrite},
		},
	}

	doc := newGenerateDoc(result, false, nil)

	want := []generateFileJSON{
		{Path: "a.go", Action: fileaction.ActionCreate},
		{Path: "b.go", Action: fileaction.ActionAppend},
		{Path: "c.go", Action: fileaction.ActionInject},
		{Path: "d.go", Action: fileaction.ActionOverwrite},
	}
	assert.Equal(t, want, doc.Files)
}

// TestUT_GenerateJSON_EmptyDetailsSerialisesAsEmptyArray pins the []T,0,n
// convention: a GenerateResult with no Details must serialise "files" as
// [], never null.
func TestUT_GenerateJSON_EmptyDetailsSerialisesAsEmptyArray(t *testing.T) {
	doc := newGenerateDoc(engine.GenerateResult{}, false, nil)

	data, err := json.Marshal(doc)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"files":[]`)
	assert.NotContains(t, string(data), "null")
}

// TestUT_GenerateJSON_ConflictsOmittedWhenAbsent verifies the "conflicts"
// field is entirely absent (omitempty) on a clean run, rather than present as
// an empty array — a JSON consumer greping for the key should see no
// conflicts key at all on a non-conflicting run.
func TestUT_GenerateJSON_ConflictsOmittedWhenAbsent(t *testing.T) {
	doc := newGenerateDoc(engine.GenerateResult{Created: 1}, false, nil)

	data, err := json.Marshal(doc)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "conflicts")
}

// TestUT_GenerateJSON_ExactlyOneDocumentAndNothingOnBypassedSinks drives a
// full generate through a real cli.App and asserts: stdout decodes to
// exactly one JSON document (decode-then-EOF, since Contains("{") cannot see
// a prepended summary line or two concatenated documents), the real
// os.Stdout was never written to directly (run.Stdout == ""), and the
// document's field set/values are correct.
func TestUT_GenerateJSON_ExactlyOneDocumentAndNothingOnBypassedSinks(t *testing.T) {
	tmpDir := setupTempDir(t)
	createGenerator(t, tmpDir, "hello", "---\nto: widget.txt\n---\nHello {{ name }}\n")
	createSharedDir(t, tmpDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	run := runCLICapturingAll(t, GenerateCommand(cfg), "generate", "hello", "widget", "--no-hooks", "--format", "json")
	require.NoError(t, run.Err)

	require.Empty(t, run.Stdout, "nothing should bypass c.App.Writer to the real os.Stdout")

	dec := json.NewDecoder(bytes.NewReader([]byte(run.Writer)))
	var doc generateDoc
	require.NoError(t, dec.Decode(&doc))
	_, eofErr := dec.Token()
	require.ErrorIs(t, eofErr, io.EOF, "exactly one JSON document must be on the wire")

	assert.Equal(t, []generateFileJSON{{Path: "widget.txt", Action: fileaction.ActionCreate}}, doc.Files)
	assert.Equal(t, 1, doc.Created)
	assert.Equal(t, 0, doc.Skipped)
	assert.Equal(t, 0, doc.Overwritten)
	assert.Equal(t, 0, doc.Modified)
	assert.False(t, doc.DryRun)
	assert.Nil(t, doc.Conflicts)
}

// TestUT_GenerateJSON_TrailingFormatFlagIsRecognised verifies --format json
// works whether it appears before or after the positional generator/name
// pair, exercising reparseTrailingFlags through a real cli.App (a hand-built
// flag.FlagSet cannot see flag-vs-positional ordering at all).
func TestUT_GenerateJSON_TrailingFormatFlagIsRecognised(t *testing.T) {
	tmpDir := setupTempDir(t)
	createGenerator(t, tmpDir, "hello", "---\nto: widget.txt\n---\nHello {{ name }}\n")
	createSharedDir(t, tmpDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	run := runCLICapturingAll(t, GenerateCommand(cfg), "generate", "--no-hooks", "hello", "widget", "--format", "json")
	require.NoError(t, run.Err)

	var doc generateDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	assert.Equal(t, 1, doc.Created)
}

// TestUT_GenerateJSON_DryRunCompletesWithoutBlocking exercises the D1 path at
// the command level: --dry-run --format json must not prompt for input. This
// is not proof the TTY predicate is correct (see engine's own
// TestUT_DiffPromptEnabled_OnlyWhenSinkIsRealTerminal for that — no test
// under `go test` can make os.Stdout a real terminal), only that the command
// completes and stdin is never read from in this mode.
func TestUT_GenerateJSON_DryRunCompletesWithoutBlocking(t *testing.T) {
	tmpDir := setupTempDir(t)
	createGenerator(t, tmpDir, "hello", "---\nto: widget.txt\n---\nHello {{ name }}\n")
	createSharedDir(t, tmpDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	run := runCLICapturingAll(t, GenerateCommand(cfg), "generate", "--no-hooks", "hello", "widget", "--dry-run", "--format", "json")
	require.NoError(t, run.Err)

	var doc generateDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	assert.True(t, doc.DryRun)
	// The file was NOT actually written in dry-run mode.
	_, statErr := os.Stat("widget.txt")
	assert.True(t, os.IsNotExist(statErr))
}

// TestUT_GenerateJSON_ConflictWritesDocumentAndExitCode exercises D5 against
// a REAL engine.ConflictError (not a mocked generator): pre-creating the
// target file makes the default OnExistingFail policy refuse to overwrite
// it, exactly the path a JSON consumer hits. Both halves of D5 are asserted
// in one run: the document is written AND the exit code is non-zero.
func TestUT_GenerateJSON_ConflictWritesDocumentAndExitCode(t *testing.T) {
	tmpDir := setupTempDir(t)
	createGenerator(t, tmpDir, "hello", "---\nto: widget.txt\n---\nHello {{ name }}\n")
	createSharedDir(t, tmpDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	require.NoError(t, os.WriteFile("widget.txt", []byte("pre-existing\n"), 0o600))

	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{Pre: [][]string{}, Post: [][]string{}}

	run := runCLICapturingAll(t, GenerateCommand(cfg), "generate", "--no-hooks", "hello", "widget", "--format", "json")
	require.Error(t, run.Err)

	type exitCoder interface{ ExitCode() int }
	ec, ok := run.Err.(exitCoder)
	require.True(t, ok, "conflict error must carry a non-zero exit code")
	assert.Equal(t, 1, ec.ExitCode())

	var doc generateDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc), "a document must still be written on conflict")
	assert.Equal(t, []string{"widget.txt"}, doc.Conflicts)
}

// TestUT_GenerateJSON_HookOutputGoesToErrWriter exercises D6: hook output
// must stay visible to a human (c.App.ErrWriter) without polluting the JSON
// document on stdout.
func TestUT_GenerateJSON_HookOutputGoesToErrWriter(t *testing.T) {
	tmpDir := setupTempDir(t)
	createGenerator(t, tmpDir, "hello", "---\nto: widget.txt\n---\nHello {{ name }}\n")
	createSharedDir(t, tmpDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg := createTestConfig(t, tmpDir)
	cfg.Hooks = config.Hooks{
		Pre:  [][]string{{"echo", "pre-hook-sentinel"}},
		Post: [][]string{{"echo", "post-hook-sentinel"}},
	}

	run := runCLICapturingAll(t, GenerateCommand(cfg), "generate", "hello", "widget", "--format", "json")
	require.NoError(t, run.Err)

	assert.NotContains(t, run.Writer, "pre-hook-sentinel")
	assert.NotContains(t, run.Writer, "post-hook-sentinel")
	assert.Contains(t, run.ErrOut, "pre-hook-sentinel")
	assert.Contains(t, run.ErrOut, "post-hook-sentinel")

	// And stdout still decodes cleanly as exactly one document.
	var doc generateDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
}

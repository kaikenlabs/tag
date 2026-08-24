package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/templateupdate"
	"github.com/kaikenlabs/tag/pkg/app"
)

// newJSONErrorApp builds a real cli.App, as opposed to a hand-built
// flag.FlagSet context: a hand-built context has App == nil, which makes
// stdoutLatch inert and every stderr assertion read a buffer nothing wrote.
// Unlike newFormatApp in format_conformance_test.go, ErrWriter is a real
// buffer, not io.Discard — these tests assert on stderr content.
func newJSONErrorApp(t *testing.T) (a *cli.App, out, errOut *bytes.Buffer) {
	t.Helper()

	out = &bytes.Buffer{}
	errOut = &bytes.Buffer{}
	cfg := createTestConfig(t, t.TempDir())
	return &cli.App{
		Writer:         out,
		ErrWriter:      errOut,
		Flags:          GlobalFlags(),
		Commands:       RootCommands(cfg, "test", SkillDocs{}),
		ExitErrHandler: func(*cli.Context, error) {},
	}, out, errOut
}

// decodeOneJSONDoc decodes exactly one JSON document from r and fails the
// test if a second one follows. json.Unmarshal cannot see trailing content,
// so "exactly one document" is only provable by decoding once with
// json.Decoder and then asserting the next Token is io.EOF.
func decodeOneJSONDoc(t *testing.T, r []byte, v any) {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(r))
	require.NoError(t, dec.Decode(v), "stdout must parse as JSON: %q", string(r))

	_, err := dec.Token()
	require.ErrorIs(t, err, io.EOF, "exactly one JSON document must be on the wire, got %q", string(r))
}

func TestUT_ErrorCode_MapsErrorChains(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "auth required",
			err:  fmt.Errorf("clone failed: %w", remote.ErrAuthRequired),
			want: codeAuthRequired,
		},
		{
			name: "version not found",
			err:  fmt.Errorf("resolve failed: %w", remote.ErrVersionNotFound),
			want: codeVersionNotFound,
		},
		{
			name: "template not found (remote)",
			err:  fmt.Errorf("resolve failed: %w", remote.ErrNotFound),
			want: codeTemplateNotFound,
		},
		{
			name: "required variable missing",
			err:  fmt.Errorf("collect vars failed: %w", scaffold.ErrRequiredVariableMissing),
			want: codeRequiredVariableMissing,
		},
		{
			name: "output exists",
			err:  fmt.Errorf("scaffold failed: %w", scaffold.ErrOutputExists),
			want: codeOutputExists,
		},
		{
			name: "circular dependency",
			err:  fmt.Errorf("resolve vars failed: %w", scaffold.ErrCircularDependency),
			want: codeCircularDependency,
		},
		{
			name: "invalid reference parse error",
			err:  fmt.Errorf("parse failed: %w", &remote.ParseError{Input: "bad", Message: "invalid"}),
			want: codeInvalidReference,
		},
		{
			name: "usage error",
			err:  app.UsageErrorf("missing argument: %s", "name"),
			want: codeUsage,
		},
		{
			name: "unmatched error falls back to internal",
			err:  errors.New("something else broke"),
			want: codeInternal,
		},
	}

	produced := make(map[string]struct{}, len(tests))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errorCode(tt.err)
			assert.Equal(t, tt.want, got, "errorCode(%v)", tt.err)
			produced[got] = struct{}{}
		})
	}

	assert.Len(t, produced, len(tests),
		"each row must produce a distinct code — a lazy collapse to one code must not pass")

	t.Run("template not found is agreed by every sentinel in the group", func(t *testing.T) {
		for _, err := range []error{
			fmt.Errorf("%w", remote.ErrSubPathNotFound),
			fmt.Errorf("%w", scaffold.ErrTemplateNotFound),
			fmt.Errorf("%w", scaffold.ErrConfigNotFound),
			fmt.Errorf("%w", library.ErrTemplateNotFound),
		} {
			assert.Equal(t, codeTemplateNotFound, errorCode(err), "errorCode(%v)", err)
		}
	})

	t.Run("auth required wins over not found in a mixed chain", func(t *testing.T) {
		chain := fmt.Errorf("github 404: %w, %w", remote.ErrAuthRequired, remote.ErrNotFound)
		assert.Equal(t, codeAuthRequired, errorCode(chain))
	})
}

func TestUT_JSONErrorDoc_OneDocumentWithPlainStderr(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{
			name: "template info",
			argv: []string{"tag", "template", "info", "/nonexistent-xyz", "--format", "json"},
		},
		{
			name: "scaffold",
			argv: []string{"tag", "scaffold", "/nonexistent-xyz", "--format", "json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			a, out, errOut := newJSONErrorApp(t)

			err := a.Run(tt.argv)
			require.Error(t, err)

			var doc errorDoc
			decodeOneJSONDoc(t, out.Bytes(), &doc)

			assert.NotEmpty(t, doc.Error.Code, "argv=%v", tt.argv)
			assert.NotEmpty(t, doc.Error.Message, "argv=%v", tt.argv)
			assert.NotZero(t, doc.Error.ExitCode, "argv=%v", tt.argv)

			assert.True(t, strings.HasPrefix(errOut.String(), "tag error: "),
				"stderr must start with 'tag error: ', got %q", errOut.String())
			assert.NotRegexp(t, `^\[\d{2}:\d{2}:\d{2}\.\d{3}\]`, errOut.String(),
				"stderr must not carry the prettylog timestamp prefix")
		})
	}
}

// TestUT_JSONErrorDoc_ExitCodeSurvivesWrapping drives a USAGE error (exit 2),
// not an exit-1 row: a table of only exit-1 rows would still pass even if
// reportedError lost its Unwrap method, because app.ExitGeneral (1) is also
// what main.go falls back to when errors.As finds nothing. Only a non-default
// exit code proves the *app.CommandError is still reachable through the
// wrapping, exactly as main.go's own extraction at main.go:71-74 requires.
func TestUT_JSONErrorDoc_ExitCodeSurvivesWrapping(t *testing.T) {
	t.Chdir(t.TempDir())
	a, out, _ := newJSONErrorApp(t)

	argv := []string{"tag", "template", "info", "--format", "json"}
	err := a.Run(argv)
	require.Error(t, err)

	var doc errorDoc
	decodeOneJSONDoc(t, out.Bytes(), &doc)
	require.Equal(t, app.ExitUsage, doc.Error.ExitCode, "the document itself must carry exit code 2")

	var cmdErr *app.CommandError
	require.True(t, errors.As(err, &cmdErr),
		"main.go's errors.As(err, &cmdErr) must still find *app.CommandError through reportedError")
	assert.Equal(t, app.ExitUsage, cmdErr.ExitCode())
}

// TestUT_WithJSONErrorDoc_SkipsWhenStdoutWritten covers the double-emit guard:
// fn writes a partial success document to stdout and THEN returns an error. A
// second, error, document must not follow it — the single-document invariant
// wins over reporting the error at all. This is a different fixture from
// TestUT_JSONErrorDoc_OneDocumentWithPlainStderr, whose fn never writes
// anything before failing.
func TestUT_WithJSONErrorDoc_SkipsWhenStdoutWritten(t *testing.T) {
	a, out, _ := newJSONErrorApp(t)

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	set.String("format", formatText, "")
	require.NoError(t, set.Set("format", formatJSON))
	c := cli.NewContext(a, set, nil)

	boom := errors.New("boom after partial write")
	fn := func() error {
		_, writeErr := fmt.Fprintln(cmdOut(c), `{"partial":true}`)
		require.NoError(t, writeErr)
		return boom
	}

	err := withJSONErrorDoc(c, 1, "test", fn)
	assert.Equal(t, boom, err, "the latch must return fn's error unchanged, not reportedError")

	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	var doc any
	require.NoError(t, dec.Decode(&doc), "stdout must parse as JSON: %q", out.String())
	_, tokErr := dec.Token()
	require.ErrorIs(t, tokErr, io.EOF,
		"exactly one JSON document must survive — the partial write, not a second error document, got %q", out.String())
}

// jsonErrorScopedCommands are the only two commands allowed to emit a
// top-level "error" key. Kept independent of formatCommands' own golden list
// so a bug in that list's maintenance cannot silently widen this guard too.
var jsonErrorScopedCommands = map[string]bool{
	"template info": true,
	"scaffold":      true,
}

// TestUT_JSONErrorDoc_ScopedToTwoCommands censuses every --format-capable
// command (formatCommands, the same traversal the conformance suite uses) with
// a guaranteed-failing invocation and asserts that only "template info" and
// "scaffold" ever produce a document carrying a top-level "error" key. This is
// what guards the wrapper from silently spreading to a third command.
func TestUT_JSONErrorDoc_ScopedToTwoCommands(t *testing.T) {
	cmds := formatCommands(t)
	require.NotEmpty(t, cmds)

	// lib search hits GitHub over HTTP unless searchBaseURL is overridden.
	// Point it at a local server that always errors, so the "guaranteed
	// failure" invocation stays guaranteed and local — no test in this suite
	// may reach the network.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	originalSearchBaseURL := searchBaseURL
	searchBaseURL = srv.URL
	t.Cleanup(func() { searchBaseURL = originalSearchBaseURL })

	for _, fc := range cmds {
		t.Run(fc.name(), func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			t.Setenv("XDG_CACHE_HOME", t.TempDir())

			argv := append([]string{"tag"}, fc.path...)
			if fc.takesPositional {
				argv = append(argv, "does-not-exist-census-probe")
			}
			argv = append(argv, "--format", "json")

			a, out, _ := newJSONErrorApp(t)
			_ = a.Run(argv)

			var m map[string]any
			if err := json.Unmarshal(out.Bytes(), &m); err != nil {
				return
			}
			if _, hasError := m["error"]; hasError {
				assert.True(t, jsonErrorScopedCommands[fc.name()],
					"%v produced a top-level 'error' key but is not one of the two scoped commands", fc.name())
			}
		})
	}
}

// TestUT_JSONErrorDoc_TextModeUntouched pins today's text-mode behaviour: the
// wrapper's text-mode branch is a pure passthrough, so stdout must stay empty
// and the returned error's message must be byte-identical to the literal
// captured from a clean-main build (/tmp/t396/base/info_local_missing_text.err
// and scaffold_local_missing_text.err, minus the "tag error: " prefix and
// prettylog's timestamp/trailing space, neither of which main.go's own error
// carries — those are artifacts of main()'s slog handler, not of the error
// value itself).
func TestUT_JSONErrorDoc_TextModeUntouched(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "template info",
			argv: []string{"tag", "template", "info", "/nonexistent-xyz"},
			want: `failed to resolve template "/nonexistent-xyz": invalid template reference: invalid reference "/nonexistent-xyz": local path not found`,
		},
		{
			name: "scaffold",
			argv: []string{"tag", "scaffold", "/nonexistent-xyz"},
			want: `failed to resolve template: invalid template reference: invalid reference "/nonexistent-xyz": local path not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			a, out, _ := newJSONErrorApp(t)

			err := a.Run(tt.argv)
			require.Error(t, err)
			assert.Empty(t, out.Bytes(), "text mode must not write to stdout")
			assert.Equal(t, tt.want, err.Error())

			_, alreadyReported := err.(reportedError)
			assert.False(t, alreadyReported, "text mode must return the original error, not reportedError")
		})
	}
}

// TestUT_ErrorExitCommands_EmitOneJSONDocument covers the four commands that
// were already write-then-exit-nonzero before this ticket (check, generate,
// undo, update — none of them route through withJSONErrorDoc) and pins that
// they still emit exactly one JSON document. This ticket does not touch these
// commands; the point is to prove that fact, not to exercise new behaviour.
// Fixtures are the minimal reproductions of each command's own
// "*ConflictWritesDocumentAndExitCode" test.
func TestUT_ErrorExitCommands_EmitOneJSONDocument(t *testing.T) {
	t.Run("check", func(t *testing.T) {
		dir := seedProject(t, "abc1234567890", "def0987654321")

		run := runCLICapturingAll(t, CheckCommand(), "check", "--dir", dir, "--format", "json")
		require.Error(t, run.Err)

		var doc any
		decodeOneJSONDoc(t, []byte(run.Writer), &doc)
	})

	t.Run("generate", func(t *testing.T) {
		tmpDir := setupTempDir(t)
		createGenerator(t, tmpDir, "hello", "---\nto: widget.txt\n---\nHello {{ name }}\n")
		createSharedDir(t, tmpDir)

		origDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		require.NoError(t, os.WriteFile("widget.txt", []byte("pre-existing\n"), 0o600))

		cfg := createTestConfig(t, tmpDir)

		run := runCLICapturingAll(t, GenerateCommand(cfg), "generate", "--no-hooks", "hello", "widget", "--format", "json")
		require.Error(t, run.Err)

		var doc any
		decodeOneJSONDoc(t, []byte(run.Writer), &doc)
	})

	t.Run("undo", func(t *testing.T) {
		dir, target := seedUndoProjectWithGeneration(t, "gen_1_aaa")
		require.NoError(t, os.WriteFile(filepath.Join(dir, target), []byte("user modified\n"), 0o600))

		run := runCLICapturingAll(t, UndoCommand(), "undo", "--yes", "--format", "json")
		require.Error(t, run.Err)

		var doc any
		decodeOneJSONDoc(t, []byte(run.Writer), &doc)
	})

	t.Run("update", func(t *testing.T) {
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

		var doc any
		decodeOneJSONDoc(t, []byte(run.Writer), &doc)
	})
}

// TestUT_JSONUsageError_ParseFailureEmitsOneDocument covers the seam
// withJSONErrorDoc cannot reach: urfave/cli rejects an unknown flag inside
// Command.Run, before Action is ever called, and its default path dumps
// "Incorrect Usage" plus the full help text to STDOUT — over a kilobyte of
// non-JSON for a consumer that asked for JSON.
//
// The rows deliberately vary where --format sits relative to the bad flag.
// Command.parseFlags returns a nil *flag.FlagSet on any parse error, so the
// failing command's own context cannot answer "was json requested?" at all;
// the answer has to come from the parent context's raw argv, which is intact
// in both orderings.
func TestUT_JSONUsageError_ParseFailureEmitsOneDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		argv []string
	}{
		{"format before the bad flag", []string{"tag", "template", "info", "--format", "json", "--bogus"}},
		{"format after the bad flag", []string{"tag", "template", "info", "--bogus", "--format", "json"}},
		{"equals form", []string{"tag", "template", "info", "--format=json", "--bogus"}},
		{"scaffold", []string{"tag", "scaffold", "--format", "json", "--bogus"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testApp, out, errOut := newJSONErrorApp(t)
			err := testApp.Run(tt.argv)
			require.Error(t, err)

			var doc errorDoc
			decodeOneJSONDoc(t, out.Bytes(), &doc)

			assert.Equal(t, codeUsage, doc.Error.Code)
			assert.NotEmpty(t, doc.Error.Message)
			assert.NotContains(t, out.String(), "Incorrect Usage",
				"urfave's help dump must not reach stdout in JSON mode")
			assert.Contains(t, errOut.String(), "tag error: ")
		})
	}
}

// TestUT_JSONUsageError_TextModeKeepsUrfaveOutput is the positive control for
// the test above: without it, deleting the whole OnUsageError handler would
// still satisfy nothing, but replacing its text branch with a bare `return err`
// would silently drop the "Incorrect Usage" banner and the help text that
// urfave prints today. Byte-identity with a main build is pinned separately by
// the integration suite.
func TestUT_JSONUsageError_TextModeKeepsUrfaveOutput(t *testing.T) {
	t.Parallel()

	testApp, out, _ := newJSONErrorApp(t)
	err := testApp.Run([]string{"tag", "template", "info", "--bogus"})

	require.Error(t, err)
	assert.Contains(t, out.String(), "Incorrect Usage: flag provided but not defined: -bogus")
	assert.Contains(t, out.String(), "tag template info")
}

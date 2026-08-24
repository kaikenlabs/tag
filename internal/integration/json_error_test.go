package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var timestampPrefix = regexp.MustCompile(`^\[\d{2}:\d{2}:\d{2}\.\d{3}\]`)

type jsonErrorDoc struct {
	SchemaVersion int `json:"schema_version"`
	Error         struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		ExitCode int    `json:"exit_code"`
	} `json:"error"`
}

// decodeOneJSONErrorDoc decodes exactly one JSON document from stdout and
// fails if a second one follows — json.Unmarshal alone cannot see trailing
// content, so "exactly one document" is only provable by decoding once and
// then demanding the next token is io.EOF.
func decodeOneJSONErrorDoc(t *testing.T, stdout []byte) jsonErrorDoc {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(stdout))
	var doc jsonErrorDoc
	require.NoError(t, dec.Decode(&doc), "stdout must parse as JSON: %q", string(stdout))

	_, tokErr := dec.Token()
	require.ErrorIs(t, tokErr, io.EOF, "exactly one JSON document must be on the wire, got %q", string(stdout))
	return doc
}

// writeProbeTemplate creates a minimal template with two required variables
// (project_name, needed) and no defaults, so omitting either one deterministically
// hits scaffold.ErrRequiredVariableMissing.
func writeProbeTemplate(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "template"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(`{
  "name": "probe",
  "vars": {
    "project_name": {"type": "string", "prompt": "Project name", "required": true},
    "needed": {"type": "string", "prompt": "Needed", "required": true}
  }
}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "template", "README.md"),
		[]byte("hi {{ vars.project_name }}\n"), 0o600))
}

func processExitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr), "subprocess error must be an *exec.ExitError: %v", err)
	return exitErr.ExitCode()
}

// TestIT_JSONError_OneDocumentAndPlainStderr is the ONLY test that can see
// main.go's slog suppression: prettylog is installed in setLogger(), which is
// unreachable from an in-package test. A nonexistent local path is used for
// both commands so no network and no fixture setup is needed.
func TestIT_JSONError_OneDocumentAndPlainStderr(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tests := []struct {
		name string
		argv []string
	}{
		{name: "template info", argv: []string{"template", "info", "/nonexistent-xyz", "--format", "json"}},
		{name: "scaffold", argv: []string{"scaffold", "/nonexistent-xyz", "--format", "json"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			stdout, stderr, err := runTagSubprocess(t, ctx, dir, tt.argv...)
			require.Error(t, err)

			doc := decodeOneJSONErrorDoc(t, stdout)
			assert.NotEmpty(t, doc.Error.Code)
			assert.NotEmpty(t, doc.Error.Message)
			assert.NotZero(t, doc.Error.ExitCode)

			// The whole of stderr must be exactly the one plain line the JSON
			// error seam writes. Checking only a "^tag error: " prefix would be
			// vacuous here: main.go's slog.Error call, if not suppressed, adds a
			// SECOND "[HH:MM:SS.mmm] tag error: ..." line right after the first
			// one, and a prefix-only check cannot see a second line at all.
			assert.Equal(t, "tag error: "+doc.Error.Message+"\n", string(stderr),
				"stderr must be exactly one plain line — main.go must not re-log an error the JSON seam already reported")
			assert.False(t, timestampPrefix.Match(stderr),
				"stderr must not carry prettylog's timestamp prefix, got %q", string(stderr))
		})
	}
}

// TestIT_JSONError_ExitCodesAndCodesMatchText is the differential test: for
// each scenario, the process exit code must be identical whether or not
// --format json is used, the JSON document's own error.exit_code must equal
// that process exit code, and the four scenarios must map to four distinct
// error codes. It also pins that TEXT mode still carries prettylog's
// [HH:MM:SS.mmm] prefix — the only thing stopping someone from "fixing" the
// JSON case by removing the slog handler for everyone.
func TestIT_JSONError_ExitCodesAndCodesMatchText(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	newScaffoldDirs := func(t *testing.T) (templateDir string) {
		t.Helper()
		dir := t.TempDir()
		writeProbeTemplate(t, dir)
		return dir
	}

	type scenario struct {
		name     string
		textArgv func(t *testing.T, workDir string) []string
		jsonArgv func(t *testing.T, workDir string) []string
	}

	scenarios := []scenario{
		{
			name: "invalid reference",
			textArgv: func(t *testing.T, workDir string) []string {
				t.Helper()
				return []string{"template", "info", "/nonexistent-xyz"}
			},
			jsonArgv: func(t *testing.T, workDir string) []string {
				t.Helper()
				return []string{"template", "info", "/nonexistent-xyz", "--format", "json"}
			},
		},
		{
			name: "output exists",
			textArgv: func(t *testing.T, workDir string) []string {
				t.Helper()
				templateDir := newScaffoldDirs(t)
				out := filepath.Join(workDir, "out-text")
				require.NoError(t, os.MkdirAll(out, 0o750))
				require.NoError(t, os.WriteFile(filepath.Join(out, "keep.txt"), []byte("x"), 0o600))
				return []string{"scaffold", templateDir, "--no-input", "-m", "project_name=demo", "-m", "needed=x", "--output", out}
			},
			jsonArgv: func(t *testing.T, workDir string) []string {
				t.Helper()
				templateDir := newScaffoldDirs(t)
				out := filepath.Join(workDir, "out-json")
				require.NoError(t, os.MkdirAll(out, 0o750))
				require.NoError(t, os.WriteFile(filepath.Join(out, "keep.txt"), []byte("x"), 0o600))
				return []string{"scaffold", templateDir, "--no-input", "-m", "project_name=demo", "-m", "needed=x", "--output", out, "--format", "json"}
			},
		},
		{
			name: "missing required variable",
			textArgv: func(t *testing.T, workDir string) []string {
				t.Helper()
				templateDir := newScaffoldDirs(t)
				out := filepath.Join(workDir, "reqvar-text")
				return []string{"scaffold", templateDir, "--no-input", "-m", "project_name=demo", "--output", out}
			},
			jsonArgv: func(t *testing.T, workDir string) []string {
				t.Helper()
				templateDir := newScaffoldDirs(t)
				out := filepath.Join(workDir, "reqvar-json")
				return []string{"scaffold", templateDir, "--no-input", "-m", "project_name=demo", "--output", out, "--format", "json"}
			},
		},
		{
			// A business-logic usage error (infoAction's own len(args)<1
			// check), which reaches withJSONErrorDoc rather than the
			// OnUsageError seam. The parse-time seam is a separate row below,
			// because the two produce the same code by different routes.
			name: "usage error",
			textArgv: func(t *testing.T, workDir string) []string {
				t.Helper()
				return []string{"template", "info"}
			},
			jsonArgv: func(t *testing.T, workDir string) []string {
				t.Helper()
				return []string{"template", "info", "--format", "json"}
			},
		},
	}

	codes := make(map[string]bool, len(scenarios))
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			workDir := t.TempDir()

			_, textStderr, textErr := runTagSubprocess(t, ctx, workDir, sc.textArgv(t, workDir)...)
			textExit := processExitCode(t, textErr)
			require.NotZero(t, textExit, "%s: text-mode invocation must fail", sc.name)
			assert.True(t, timestampPrefix.Match(textStderr),
				"%s: text mode must still carry prettylog's timestamp prefix, got %q", sc.name, string(textStderr))
			assert.Regexp(t, `\] tag error: `, string(textStderr), "%s", sc.name)

			jsonStdout, _, jsonErr := runTagSubprocess(t, ctx, workDir, sc.jsonArgv(t, workDir)...)
			jsonExit := processExitCode(t, jsonErr)

			assert.Equal(t, textExit, jsonExit, "%s: exit code must not depend on --format", sc.name)

			doc := decodeOneJSONErrorDoc(t, jsonStdout)
			assert.Equal(t, jsonExit, doc.Error.ExitCode, "%s: document exit_code must equal the process exit code", sc.name)

			codes[doc.Error.Code] = true
		})
	}

	assert.Len(t, codes, len(scenarios), "the four scenarios must map to four distinct error codes, got %v", codes)
}

// TestIT_JSONError_ParseFailureEmitsOneDocument covers the OnUsageError seam
// through the real binary. It is deliberately separate from the differential
// table above: a parse failure also maps to "usage", so folding it in would
// break that table's distinct-codes assertion.
//
// This is the one seam whose JSON branch an earlier revision of this file
// claimed was unreachable dead code. It is not — the claim came from analysing
// resolveFormat, which the shipped code does not use here. Running the built
// binary is what settles it, which is why this test spawns a subprocess rather
// than asserting in-process.
func TestIT_JSONError_ParseFailureEmitsOneDocument(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	tests := []struct {
		name string
		argv []string
	}{
		{"format before the bad flag", []string{"template", "info", "--format", "json", "--bogus"}},
		{"format after the bad flag", []string{"template", "info", "--bogus", "--format", "json"}},
		{"scaffold", []string{"scaffold", "--format", "json", "--bogus"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			workDir := t.TempDir()
			stdout, stderr, runErr := runTagSubprocess(t, ctx, workDir, tt.argv...)

			exit := processExitCode(t, runErr)
			require.NotZero(t, exit)

			doc := decodeOneJSONErrorDoc(t, stdout)
			assert.Equal(t, "usage", doc.Error.Code)
			assert.Equal(t, exit, doc.Error.ExitCode,
				"document exit_code must equal the process exit code")
			assert.NotContains(t, string(stdout), "Incorrect Usage",
				"urfave's help dump must not reach stdout when JSON was requested")

			assert.False(t, timestampPrefix.Match(stderr),
				"JSON mode stderr must not carry prettylog's timestamp prefix, got %q", string(stderr))
			assert.Contains(t, string(stderr), "tag error: ")
		})
	}
}

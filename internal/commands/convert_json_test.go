package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/convert"
)

// writeCookiecutterFixture builds a minimal Cookiecutter template under a
// fresh temp dir: cookiecutter.json plus one path-templated file that also
// triggers a content incompatibility (a Jinja2 filter with no Gonja
// equivalent), and returns the source directory.
func writeCookiecutterFixture(t *testing.T) string {
	t.Helper()
	sourceDir := t.TempDir()

	ccJSON := `{"project_name": "my_project", "version": "0.1.0"}`
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "cookiecutter.json"), []byte(ccJSON), 0o644))

	projectDir := filepath.Join(sourceDir, "{{cookiecutter.project_name}}")
	require.NoError(t, os.MkdirAll(projectDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDir, "README.md"),
		// The |upper('x') filter-parens form is a real, deliberate
		// incompatibility trigger (see filterParenRegex in
		// internal/convert/content.go) — plain {{ cookiecutter.x }}
		// references convert cleanly and trigger no finding at all.
		[]byte("# {{cookiecutter.project_name|upper('x')}}\nVersion: {{cookiecutter.version}}\n"),
		0o644,
	))

	return sourceDir
}

// TestUT_ConvertJSON_ProducesResultDocument exercises the full conversion
// through a real cli.App and asserts the new Files/Variables fields the
// command layer now populates (approach.md's #355 scope): a per-file path
// conversion and a per-variable conversion, both absent from the pre-existing
// counters-only Result.
func TestUT_ConvertJSON_ProducesResultDocument(t *testing.T) {
	sourceDir := writeCookiecutterFixture(t)
	outDir := filepath.Join(t.TempDir(), "converted")

	run := runCLICapturingAll(t, convertCookiecutterCommand(), "cookiecutter", sourceDir, "--output", outDir, "--format", "json")
	require.NoError(t, run.Err)

	var result convert.Result
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &result))

	assert.Equal(t, outDir, result.Destination)
	assert.False(t, result.DryRun)
	assert.Equal(t, 2, result.VariablesConverted)
	require.Len(t, result.Variables, 2)

	require.NotEmpty(t, result.Files)
	var readme *convert.PathConversion
	for i := range result.Files {
		if result.Files[i].From == filepath.Join("{{cookiecutter.project_name}}", "README.md") {
			readme = &result.Files[i]
		}
	}
	require.NotNil(t, readme, "expected a Files entry for the templated README")
	// ConvertPath rewrites the cookiecutter.* namespace to vars.* IN PLACE
	// inside the {{ }} block; it does not rewrite to a double-underscore
	// __var__ form (verified against the actual internal/convert/paths.go
	// behaviour, not assumed).
	assert.Equal(t, filepath.Join("{{vars.project_name}}", "README.md"), readme.To)

	// The README's content triggers a real incompatibility finding (the
	// |upper('x') filter-parens form).
	require.NotEmpty(t, result.Incompatibilities)
	assert.Equal(t, convert.SeverityWarning, result.Incompatibilities[0].Severity, "Severity is a plain string type; assert its literal wire value")
	assert.Equal(t, "filter-syntax", result.Incompatibilities[0].Kind)
}

// TestUT_ConvertJSON_NoFindingsStillEmitsEmptyArrays pins the []T,0,n
// convention: incompatibilities/warnings must serialise as [], not null,
// even on a template that triggers neither.
func TestUT_ConvertJSON_NoFindingsStillEmitsEmptyArrays(t *testing.T) {
	sourceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "cookiecutter.json"), []byte(`{}`), 0o644))
	projectDir := filepath.Join(sourceDir, "plain")
	require.NoError(t, os.MkdirAll(projectDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package plain\n"), 0o644))

	outDir := filepath.Join(t.TempDir(), "converted")
	run := runCLICapturingAll(t, convertCookiecutterCommand(), "cookiecutter", sourceDir, "--output", outDir, "--format", "json")
	require.NoError(t, run.Err)

	assert.Contains(t, run.Writer, `"incompatibilities": []`)
	assert.Contains(t, run.Writer, `"warnings": []`)
	assert.Contains(t, run.Writer, `"variables": []`)
}

// TestUT_ConvertJSON_TrailingDryRunAndFormatAreRecognised drives a trailing
// --dry-run (an App-level global flag, not declared on this command) and a
// trailing --format through reparseTrailingFlags in one invocation.
func TestUT_ConvertJSON_TrailingDryRunAndFormatAreRecognised(t *testing.T) {
	sourceDir := writeCookiecutterFixture(t)
	outDir := filepath.Join(t.TempDir(), "converted")

	run := runCLICapturingAll(t, convertCookiecutterCommand(), "cookiecutter", sourceDir, "--output", outDir, "--dry-run", "--format", "json")
	require.NoError(t, run.Err)

	var result convert.Result
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &result))
	assert.True(t, result.DryRun)

	_, statErr := os.Stat(outDir)
	assert.True(t, os.IsNotExist(statErr), "dry-run must not create the output directory")
}

// TestUT_ConvertJSON_ExactlyOneDocumentAndNothingOnBypassedSinks decodes
// stdout then requires io.EOF, and asserts nothing bypassed c.App.Writer to
// the real os.Stdout.
func TestUT_ConvertJSON_ExactlyOneDocumentAndNothingOnBypassedSinks(t *testing.T) {
	sourceDir := writeCookiecutterFixture(t)
	outDir := filepath.Join(t.TempDir(), "converted")

	run := runCLICapturingAll(t, convertCookiecutterCommand(), "cookiecutter", sourceDir, "--output", outDir, "--format", "json")
	require.NoError(t, run.Err)
	require.Empty(t, run.Stdout, "nothing should bypass c.App.Writer to the real os.Stdout")

	dec := json.NewDecoder(bytes.NewReader([]byte(run.Writer)))
	var result convert.Result
	require.NoError(t, dec.Decode(&result))
	_, eofErr := dec.Token()
	require.ErrorIs(t, eofErr, io.EOF, "exactly one JSON document must be on the wire")
}

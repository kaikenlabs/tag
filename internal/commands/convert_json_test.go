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

// TestUT_ConvertJSON_DryRunFileListMatchesRealRun covers #355's "--dry-run is
// reflected in the JSON and produces the same file list as a real run".
//
// The two runs go through genuinely different branches of Convert and
// processTemplateFiles — a dry run creates no directories, writes no files and
// never writes tag.template.json — so "the preview tells you what the real run
// will do" is a claim about two code paths agreeing, not a tautology. It is
// asserted on the same fixture, comparing the whole files array (and the
// counters that must accompany it), with only dry_run itself expected to
// differ.
func TestUT_ConvertJSON_DryRunFileListMatchesRealRun(t *testing.T) {
	type convertDoc struct {
		DryRun         bool                     `json:"dry_run"`
		Files          []convert.PathConversion `json:"files"`
		FilesProcessed int                      `json:"files_processed"`
		FilesRenamed   int                      `json:"files_renamed"`
		DirsRenamed    int                      `json:"dirs_renamed"`
	}

	run := func(t *testing.T, argv ...string) convertDoc {
		t.Helper()
		r := runCLICapturingAll(t, ConvertCommand(), argv...)
		require.NoError(t, r.Err)
		require.Empty(t, r.Stdout, "nothing may bypass the command writer")
		var doc convertDoc
		require.NoError(t, json.Unmarshal([]byte(r.Writer), &doc))
		return doc
	}

	sourceDir := writeCookiecutterFixture(t)
	realOut := filepath.Join(t.TempDir(), "real")
	dryOut := filepath.Join(t.TempDir(), "dry")

	dry := run(t, "convert", "cookiecutter", sourceDir, "-o", dryOut, "--dry-run", "--format", "json")
	actual := run(t, "convert", "cookiecutter", sourceDir, "-o", realOut, "--format", "json")

	assert.True(t, dry.DryRun)
	assert.False(t, actual.DryRun)

	assert.Equal(t, actual.Files, dry.Files, "the dry-run preview must list exactly the files a real run converts")
	assert.Equal(t, actual.FilesProcessed, dry.FilesProcessed)
	assert.Equal(t, actual.FilesRenamed, dry.FilesRenamed)
	assert.Equal(t, actual.DirsRenamed, dry.DirsRenamed)
	require.NotEmpty(t, actual.Files, "a fixture that converts nothing would make this vacuous")

	// The dry run must genuinely not have written anything, or it was not a
	// dry run and the comparison above proves nothing.
	_, statErr := os.Stat(dryOut)
	assert.True(t, os.IsNotExist(statErr), "dry run must not create the output directory")
	require.DirExists(t, realOut)
}

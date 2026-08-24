package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/types"
)

func createScaffoldJSONTemplate(t *testing.T, dir string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, types.TemplateConfigFile), []byte(`{
  "name": "test-template",
  "vars": {
    "project_name": {
      "type": "string",
      "default": "my-project",
      "prompt": "Project name"
    }
  }
}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# {{ vars.project_name }}"), 0o644))
}

func chdirTemp(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	return tmpDir
}

func TestUT_ScaffoldJSON_DocumentShape(t *testing.T) {
	chdirTemp(t)
	templateDir := t.TempDir()
	createScaffoldJSONTemplate(t, templateDir)

	run := runCLICapturingAll(t, ScaffoldCommand(testVersion), "scaffold", templateDir, "widget", "--format", "json")
	require.NoError(t, run.Err)

	// Exact key set, not Contains: a subset check passes whether or not a key
	// was dropped and whether or not a stray one appeared.
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))

	got := make([]string, 0, len(doc))
	for k := range doc {
		got = append(got, k)
	}
	assert.ElementsMatch(t, []string{
		"schema_version", "tag_version", "output_dir", "project_root",
		"template", "files", "created", "dry_run",
	}, got)

	assert.Equal(t, "1", string(doc["schema_version"]))
	// Asserted through the real command, not newScaffoldDoc directly: this is
	// the only thing that can catch runScaffold passing "" down the chain.
	assert.JSONEq(t, `"`+testVersion+`"`, string(doc["tag_version"]))
}

func TestUT_ScaffoldJSON_StdoutIsOnlyTheDocument(t *testing.T) {
	chdirTemp(t)
	templateDir := t.TempDir()
	createScaffoldJSONTemplate(t, templateDir)

	run := runCLICapturingAll(t, ScaffoldCommand(testVersion), "scaffold", templateDir, "widget", "--format", "json")
	require.NoError(t, run.Err)

	require.Empty(t, run.Stdout, "nothing should bypass c.App.Writer to the real os.Stdout")

	dec := json.NewDecoder(bytes.NewReader([]byte(run.Writer)))
	var doc scaffoldDoc
	require.NoError(t, dec.Decode(&doc))
	_, eofErr := dec.Token()
	require.ErrorIs(t, eofErr, io.EOF, "exactly one JSON document must be on the wire")
}

func TestUT_ScaffoldJSON_DryRunListsFilesAndWritesNothing(t *testing.T) {
	cwd := chdirTemp(t)
	templateDir := t.TempDir()
	createScaffoldJSONTemplate(t, templateDir)

	run := runCLICapturingAll(t, ScaffoldCommand(testVersion), "scaffold", templateDir, "widget", "--dry-run", "--format", "json")
	require.NoError(t, run.Err)

	var doc scaffoldDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	assert.True(t, doc.DryRun)
	assert.NotEmpty(t, doc.Files)

	_, statErr := os.Stat(filepath.Join(cwd, "widget"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestUT_ScaffoldJSON_NoTemplateIsUsageError(t *testing.T) {
	run := runCLICapturingAll(t, ScaffoldCommand(testVersion), "scaffold", "--format", "json")
	require.Error(t, run.Err)

	// A JSON-mode failure now comes back as reportedError (see
	// withJSONErrorDoc), so the exit code is reached via errors.As through its
	// Unwrap, not a direct type assertion — the same idiom main.go itself uses.
	var coder cli.ExitCoder
	require.True(t, errors.As(run.Err, &coder), "no-template-in-JSON-mode must carry a usage exit code")
	assert.Equal(t, 2, coder.ExitCode())
}

func TestUT_ScaffoldJSON_MissingRequiredVarIsError(t *testing.T) {
	chdirTemp(t)
	templateDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, types.TemplateConfigFile), []byte(`{
  "name": "test-template",
  "vars": {
    "project_name": {
      "type": "string",
      "default": "my-project",
      "prompt": "Project name"
    },
    "author": {
      "type": "string",
      "required": true,
      "prompt": "Author"
    }
  }
}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "README.md"), []byte("# {{ vars.project_name }}"), 0o644))

	run := runCLICapturingAll(t, ScaffoldCommand(testVersion), "scaffold", templateDir, "widget", "--format", "json")
	require.Error(t, run.Err)
	assert.Contains(t, run.Err.Error(), "author")
}

func TestUT_ScaffoldJSON_TrailingFormatFlagWorks(t *testing.T) {
	chdirTemp(t)
	templateDir := t.TempDir()
	createScaffoldJSONTemplate(t, templateDir)

	run := runCLICapturingAll(t, ScaffoldCommand(testVersion), "scaffold", templateDir, "widget", "--format", "json")
	require.NoError(t, run.Err)

	var doc scaffoldDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	assert.Equal(t, 1, doc.Created)
}

package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeExtractSource creates a small Go source file in dir suitable as an
// extract source, and returns its path.
func writeExtractSource(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "user_handler.go")
	require.NoError(t, os.WriteFile(src, []byte(
		"package handler\n\ntype UserHandler struct{}\n\nfunc NewUserHandler() *UserHandler { return &UserHandler{} }\n",
	), 0o600))
	return src
}

// TestUT_ExtractJSON_DryRunIncludesContent exercises D8: in dry-run nothing
// is written to disk, so the JSON document must carry the generated content
// or a JSON consumer has no way to see the result at all.
func TestUT_ExtractJSON_DryRunIncludesContent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	src := writeExtractSource(t, dir)

	run := runCLICapturingAll(t, ExtractCommand(),
		"extract", "--name", "user", "--as", "handler", "--dry-run", src, "--format", "json")
	require.NoError(t, run.Err)

	var doc extractDoc
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &doc))
	assert.NotEmpty(t, doc.Content)
	assert.Contains(t, doc.Content, "{{ name")
	assert.Equal(t, filepath.Join(".tag", "handler", "user_handler.go"), doc.TemplatePath)
	assert.Positive(t, doc.Replacements)

	// Nothing was actually written in dry-run.
	_, statErr := os.Stat(doc.TemplatePath)
	assert.True(t, os.IsNotExist(statErr))
}

// TestUT_ExtractJSON_RealRunOmitsContent exercises the other half of D8: on a
// real run the file is already on disk, so Content must be entirely absent
// (not merely empty) from the document — a whole-key-set comparison so an
// accidental extra key is caught, which assert.NotContains on a substring
// cannot see (this is the class of bug the repo's cache-ls leak was).
func TestUT_ExtractJSON_RealRunOmitsContent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	src := writeExtractSource(t, dir)

	run := runCLICapturingAll(t, ExtractCommand(),
		"extract", "--name", "user", "--as", "handler", src, "--format", "json")
	require.NoError(t, run.Err)

	var raw map[string]any
	dec := json.NewDecoder(bytes.NewReader([]byte(run.Writer)))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(&raw))

	got := slices.Sorted(maps.Keys(raw))
	assert.Equal(t, []string{"replacements", "template_path", "to_path"}, got)

	// The template really was written to disk this time.
	templatePath, ok := raw["template_path"].(string)
	require.True(t, ok)
	_, statErr := os.Stat(templatePath)
	assert.NoError(t, statErr)
}

// TestUT_ExtractJSON_InteractiveIsUsageError exercises D3: silently
// disabling an explicitly requested flag is worse than refusing outright.
func TestUT_ExtractJSON_InteractiveIsUsageError(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	src := writeExtractSource(t, dir)

	run := runCLICapturingAll(t, ExtractCommand(),
		"extract", "--name", "user", "--as", "handler", "-i", src, "--format", "json")
	require.Error(t, run.Err)
	assert.Contains(t, run.Err.Error(), "interactive")
	assert.Empty(t, run.Writer, "a usage error must not also emit a document")
}

// TestUT_ExtractJSON_PreviewStaysOffStdout verifies the dry-run preview text
// (written through extract.Options.Writer) is rerouted off stdout in JSON
// mode: present on ErrOut, absent from the document sink, and the document
// itself still decodes as exactly one JSON value.
func TestUT_ExtractJSON_PreviewStaysOffStdout(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	src := writeExtractSource(t, dir)

	run := runCLICapturingAll(t, ExtractCommand(),
		"extract", "--name", "user", "--as", "handler", "--dry-run", src, "--format", "json")
	require.NoError(t, run.Err)

	assert.Contains(t, run.ErrOut, "Dry Run")
	assert.NotContains(t, run.Writer, "Dry Run")
	require.Empty(t, run.Stdout, "nothing should bypass c.App.Writer to the real os.Stdout")

	dec := json.NewDecoder(bytes.NewReader([]byte(run.Writer)))
	var doc extractDoc
	require.NoError(t, dec.Decode(&doc))
	_, eofErr := dec.Token()
	require.ErrorIs(t, eofErr, io.EOF, "exactly one JSON document must be on the wire")
}

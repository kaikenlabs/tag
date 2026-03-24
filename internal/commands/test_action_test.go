package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_TestAction_DryRunWithBooleanVars(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a minimal tag.template.json with boolean vars
	tmplConfig := `{
  "name": "test-template",
  "vars": {
    "project_name": {
      "type": "string",
      "default": "test-proj",
      "prompt": "Project name"
    },
    "use_docker": {
      "type": "boolean",
      "default": true,
      "prompt": "Use Docker?"
    },
    "use_ci": {
      "type": "boolean",
      "default": false,
      "prompt": "Use CI?"
    }
  }
}`
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, types.TemplateConfigFile),
		[]byte(tmplConfig),
		0o644,
	))

	// Create the wrapper dir so scaffold can work
	wrapperDir := filepath.Join(dir, "{{ vars.project_name }}")
	require.NoError(t, os.MkdirAll(wrapperDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(wrapperDir, "README.md"),
		[]byte("# {{ vars.project_name }}"),
		0o644,
	))

	ctx := newTestCLIContext(t, []string{dir}, map[string]string{
		"dry-run": "true",
	})

	var buf bytes.Buffer
	err := testAction(ctx, &buf, dir)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Template:")
	assert.Contains(t, out, "Boolean variables:")
	assert.Contains(t, out, "Combinations:")
}

func TestUT_TestAction_DryRunWithNoVars(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Template with only string vars (no booleans to permute)
	tmplConfig := `{
  "name": "test-template",
  "vars": {
    "project_name": {
      "type": "string",
      "default": "hello",
      "prompt": "Name?"
    }
  }
}`
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, types.TemplateConfigFile),
		[]byte(tmplConfig),
		0o644,
	))

	wrapperDir := filepath.Join(dir, "{{ vars.project_name }}")
	require.NoError(t, os.MkdirAll(wrapperDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(wrapperDir, "main.txt"),
		[]byte("hello"),
		0o644,
	))

	ctx := newTestCLIContext(t, nil, map[string]string{
		"dry-run": "true",
	})

	var buf bytes.Buffer
	err := testAction(ctx, &buf, dir)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Template:")
}

func TestUT_TestAction_WithRunCommands(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	tmplConfig := `{
  "name": "test-template",
  "vars": {
    "project_name": {
      "type": "string",
      "default": "runtest",
      "prompt": "Name?"
    }
  }
}`
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, types.TemplateConfigFile),
		[]byte(tmplConfig),
		0o644,
	))

	wrapperDir := filepath.Join(dir, "{{ vars.project_name }}")
	require.NoError(t, os.MkdirAll(wrapperDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(wrapperDir, "hello.txt"),
		[]byte("hello {{ vars.project_name }}"),
		0o644,
	))

	ctx := newTestCLIContext(t, nil, map[string]string{
		"run":    "true",
		"format": "text",
	})

	var buf bytes.Buffer
	err := testAction(ctx, &buf, dir)
	// This should succeed - "true" always returns 0
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Template:")
}

func TestUT_TestAction_NonExistentDir(t *testing.T) {
	t.Parallel()

	ctx := newTestCLIContext(t, nil, nil)

	var buf bytes.Buffer
	err := testAction(ctx, &buf, "/nonexistent/path/to/template")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test plan")
}

func TestUT_TestAction_JSONFormat_DryRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	tmplConfig := `{
  "name": "test-template",
  "vars": {
    "project_name": {
      "type": "string",
      "default": "json-test",
      "prompt": "Name?"
    },
    "use_x": {
      "type": "boolean",
      "default": false,
      "prompt": "X?"
    }
  }
}`
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, types.TemplateConfigFile),
		[]byte(tmplConfig),
		0o644,
	))

	wrapperDir := filepath.Join(dir, "{{ vars.project_name }}")
	require.NoError(t, os.MkdirAll(wrapperDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(wrapperDir, "f.txt"),
		[]byte("x"),
		0o644,
	))

	ctx := newTestCLIContext(t, nil, map[string]string{
		"dry-run": "true",
		"format":  "json",
	})

	var buf bytes.Buffer
	err := testAction(ctx, &buf, dir)
	require.NoError(t, err)

	out := buf.String()
	// In dry-run + json format, the header should be suppressed
	assert.NotContains(t, out, "Template:")
}

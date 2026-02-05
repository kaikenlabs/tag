package convert

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Converter_DryRun(t *testing.T) {
	// Create a minimal cookiecutter template
	srcDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "output")

	// Create cookiecutter.json
	ccConfig := `{
		"project_name": "test_project",
		"author": "Test Author"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(ccConfig), 0o644))

	// Create a template file
	templateDir := filepath.Join(srcDir, "{{ cookiecutter.project_name }}")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "main.go"),
		[]byte("package main\n"),
		0o644,
	))

	// Run in dry-run mode
	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
		DryRun:      true,
	})

	require.NoError(t, err)
	assert.True(t, result.DryRun)
	assert.Equal(t, 2, result.VariablesConverted)
	assert.Equal(t, 1, result.DirsRenamed) // {{ cookiecutter.project_name }}

	// Verify nothing was written
	_, err = os.Stat(destDir)
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Converter_LocalTemplate(t *testing.T) {
	// Create a cookiecutter template
	srcDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "output")

	// Create cookiecutter.json
	ccConfig := `{
		"project_name": "my_project",
		"use_docker": true,
		"license": ["MIT", "Apache-2.0"]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(ccConfig), 0o644))

	// Create template structure
	projectDir := filepath.Join(srcDir, "{{ cookiecutter.project_name }}")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Create a template file
	mainContent := `package main

func main() {
	// Project: {{ cookiecutter.project_name }}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "main.go.tmpl"), []byte(mainContent), 0o644))

	// Run conversion
	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})

	require.NoError(t, err)
	assert.Equal(t, destDir, result.Destination)
	assert.Equal(t, 3, result.VariablesConverted)
	assert.Equal(t, 1, result.DirsRenamed)

	// Verify tag.template.json was created
	tagConfigPath := filepath.Join(destDir, "tag.template.json")
	_, err = os.Stat(tagConfigPath)
	require.NoError(t, err)

	tagConfig, err := os.ReadFile(tagConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(tagConfig), "project_name")
	assert.Contains(t, string(tagConfig), "use_docker")

	// Verify directory was renamed
	convertedProjectDir := filepath.Join(destDir, "__project_name__")
	_, err = os.Stat(convertedProjectDir)
	require.NoError(t, err)
}

func TestUT_Converter_OutputExists_NoForce(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir() // Already exists

	// Create cookiecutter.json
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(`{}`), 0o644))

	converter, err := NewConverter()
	require.NoError(t, err)

	_, err = converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
		Force:       false,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutputExists)
}

func TestUT_Converter_OutputExists_WithForce(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create cookiecutter.json
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(`{"name": "test"}`), 0o644))

	// Pre-populate destination
	require.NoError(t, os.WriteFile(filepath.Join(destDir, "old_file.txt"), []byte("old"), 0o644))

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
		Force:       true,
	})

	require.NoError(t, err)
	assert.Equal(t, destDir, result.Destination)

	// Verify old file was removed
	_, err = os.Stat(filepath.Join(destDir, "old_file.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Converter_MissingCookiecutterJSON(t *testing.T) {
	srcDir := t.TempDir()
	// Don't create cookiecutter.json

	converter, err := NewConverter()
	require.NoError(t, err)

	_, err = converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: t.TempDir(),
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoCookiecutterConfig)
}

func TestUT_Converter_WithHooks(t *testing.T) {
	srcDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "output")

	// Create cookiecutter.json
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(`{"name": "test"}`), 0o644))

	// Create hooks
	hooksDir := filepath.Join(srcDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre_gen_project.py"), []byte("# hook"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post_gen_project.sh"), []byte("#!/bin/bash"), 0o755))

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})

	require.NoError(t, err)
	assert.Equal(t, 2, result.HooksCopied)
	assert.True(t, len(result.Warnings) >= 2) // Warnings about hooks

	// Verify hooks were copied
	_, err = os.Stat(filepath.Join(destDir, "hooks", "pre_gen_project.py"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(destDir, "hooks", "post_gen_project.sh"))
	require.NoError(t, err)
}

func TestUT_Converter_ContentAnalysis(t *testing.T) {
	srcDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "output")

	// Create cookiecutter.json
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(`{"name": "test"}`), 0o644))

	// Create a template with Jinja2-specific syntax
	content := `{% for k, v in items.items() %}
{{ k }}: {{ v | default('none') }}
{% endfor %}`
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "template.tmpl"), []byte(content), 0o644))

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})

	require.NoError(t, err)
	assert.Greater(t, len(result.Incompatibilities), 0)

	// Check for specific incompatibilities
	kinds := make(map[string]bool)
	for _, inc := range result.Incompatibilities {
		kinds[inc.Kind] = true
	}
	assert.True(t, kinds["dict-iteration"] || kinds["filter-syntax"])
}

func TestUT_Converter_DefaultDestination(t *testing.T) {
	srcDir := t.TempDir()
	// Simulate a cookiecutter template directory name
	srcDir = filepath.Join(srcDir, "cookiecutter-myproject")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(`{}`), 0o644))

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: "", // Empty - should infer
	})

	require.NoError(t, err)
	// Should be "myproject-tag" (strips cookiecutter- prefix)
	assert.Equal(t, "myproject-tag", result.Destination)

	// Clean up
	os.RemoveAll(result.Destination)
}

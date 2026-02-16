package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
)

// createTemplateConfig writes a tag.template.json file in the given directory.
func createTemplateConfig(t *testing.T, dir string, config map[string]any) {
	t.Helper()
	data, err := json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, types.TemplateConfigFile), data, 0o644))
}

func TestUT_DisplayTemplateInfo_FullTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name":        "my-template",
		"version":     "1.2.0",
		"description": "A test template",
		"vars": map[string]any{
			"project_name": "default-name",
			"use_docker": map[string]any{
				"type":    "boolean",
				"default": true,
			},
			"license": map[string]any{
				"type":    "choice",
				"options": []string{"MIT", "Apache-2.0", "GPL-3.0"},
			},
		},
		"hooks": map[string]any{
			"pre_scaffold":  []string{"echo pre"},
			"post_scaffold": []string{"go mod tidy", "git init"},
		},
	})

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# My Template\nThis is great."), 0o644)
	os.WriteFile(filepath.Join(dir, "HOWTO.md"), []byte("# How To\nStep 1: do things."), 0o644)

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.NoError(t, err)

	out := buf.String()

	// Metadata
	assert.Contains(t, out, "Name:         my-template")
	assert.Contains(t, out, "Version:      1.2.0")
	assert.Contains(t, out, "Description:  A test template")

	// Variables
	assert.Contains(t, out, "Variables:")
	assert.Contains(t, out, "project_name")
	assert.Contains(t, out, "use_docker")
	assert.Contains(t, out, "(boolean)")
	assert.Contains(t, out, "license")
	assert.Contains(t, out, "(choice:")

	// Hooks
	assert.Contains(t, out, "Hooks:")
	assert.Contains(t, out, "pre_scaffold:")
	assert.Contains(t, out, "echo pre")
	assert.Contains(t, out, "post_scaffold:")
	assert.Contains(t, out, "go mod tidy")

	// README
	assert.Contains(t, out, "--- README ---")
	assert.Contains(t, out, "My Template")

	// HOWTO
	assert.Contains(t, out, "--- HOWTO ---")
	assert.Contains(t, out, "How To")
}

func TestUT_DisplayTemplateInfo_MinimalTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name": "minimal",
		"vars": map[string]any{},
	})

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Name:         minimal")
	assert.NotContains(t, out, "Version:")
	assert.NotContains(t, out, "Variables:")
	assert.NotContains(t, out, "Hooks:")
	assert.NotContains(t, out, "--- README ---")
	assert.NotContains(t, out, "--- HOWTO ---")
}

func TestUT_DisplayTemplateInfo_MissingConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a TAG template")
}

func TestUT_DisplayTemplateInfo_NoReadmeNoHowto(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name": "no-docs",
		"vars": map[string]any{
			"name": "test",
		},
	})

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Name:         no-docs")
	assert.Contains(t, out, "Variables:")
	assert.NotContains(t, out, "--- README ---")
	assert.NotContains(t, out, "--- HOWTO ---")
}

func TestUT_DisplayTemplateInfo_OnlyHowto(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name": "howto-only",
		"vars": map[string]any{},
	})

	os.WriteFile(filepath.Join(dir, "HOWTO.md"), []byte("# Steps\n1. Do this"), 0o644)

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.NoError(t, err)

	out := buf.String()
	assert.NotContains(t, out, "--- README ---")
	assert.Contains(t, out, "--- HOWTO ---")
	assert.Contains(t, out, "Steps")
}

func TestUT_DisplayTemplateInfo_EmptyReadme(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name": "empty-readme",
		"vars": map[string]any{},
	})

	os.WriteFile(filepath.Join(dir, "README.md"), []byte(""), 0o644)

	var buf bytes.Buffer
	err := displayTemplateInfo(&buf, dir)
	require.NoError(t, err)

	out := buf.String()
	assert.NotContains(t, out, "--- README ---")
}

// Tests that library resolution is tried first.
// WARNING: This test mutates package-level newLocalLibrary; do NOT use t.Parallel().
func TestUT_ResolveTemplateDir_LibraryFirst(t *testing.T) {
	templateDir := setupFakeLibrary(t, "my-lib-template")

	// Write a minimal config so the template is valid
	createTemplateConfig(t, templateDir, map[string]any{
		"name": "my-lib-template",
		"vars": map[string]any{},
	})

	c := createTestCLIContext(t, []string{"my-lib-template"}, nil)
	resolved, err := resolveTemplateDir(c, "my-lib-template")
	require.NoError(t, err)
	assert.Equal(t, templateDir, resolved)
}

func TestUT_ResolveTemplateDir_LocalPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	createTemplateConfig(t, dir, map[string]any{
		"name": "local-test",
		"vars": map[string]any{},
	})

	c := createTestCLIContext(t, []string{dir}, nil)
	resolved, err := resolveTemplateDir(c, dir)
	require.NoError(t, err)
	assert.Equal(t, dir, resolved)
}

func TestUT_DisplayMetadata_AllFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	displayMetadata(&buf, &scaffold.TemplateConfig{
		Name:        "test",
		Version:     "2.0.0",
		Description: "A great template",
	})

	out := buf.String()
	assert.Contains(t, out, "Name:         test")
	assert.Contains(t, out, "Version:      2.0.0")
	assert.Contains(t, out, "Description:  A great template")
}

func TestUT_DisplayMetadata_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	displayMetadata(&buf, &scaffold.TemplateConfig{})

	assert.Empty(t, buf.String())
}

func TestUT_DisplayVariables_Sorted(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	displayVariables(&buf, &scaffold.TemplateConfig{
		Vars: map[string]scaffold.VariableDef{
			"zebra":   {Type: scaffold.VarTypeString},
			"alpha":   {Type: scaffold.VarTypeString, Default: "hello"},
			"beta":    {Type: scaffold.VarTypeBoolean},
			"charlie": {Type: scaffold.VarTypeChoice, Options: []string{"a", "b", "c"}},
		},
	})

	out := buf.String()
	// Check order: alpha before beta before charlie before zebra
	alphaIdx := strings.Index(out, "alpha")
	betaIdx := strings.Index(out, "beta")
	charlieIdx := strings.Index(out, "charlie")
	zebraIdx := strings.Index(out, "zebra")

	assert.Greater(t, betaIdx, alphaIdx)
	assert.Greater(t, charlieIdx, betaIdx)
	assert.Greater(t, zebraIdx, charlieIdx)

	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "= hello")
	assert.Contains(t, out, "(boolean)")
	assert.Contains(t, out, "(choice: [a b c])")
}

func TestUT_DisplayHooks_NoHooks(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	displayHooks(&buf, &scaffold.TemplateConfig{})
	assert.Empty(t, buf.String())
}

func TestUT_DisplayHooks_EmptyHooksConfig(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	displayHooks(&buf, &scaffold.TemplateConfig{
		Hooks: &types.HooksConfig{},
	})
	assert.Empty(t, buf.String())
}

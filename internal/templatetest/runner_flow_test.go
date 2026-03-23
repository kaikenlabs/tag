package templatetest_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/templatetest"
)

// setupScaffoldTemplate creates a minimal scaffold template with
// tag.template.json and a simple file. Returns the template root path.
func setupScaffoldTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// tag.template.json
	cfg := map[string]any{
		"name": "test-scaffold",
		"vars": map[string]any{
			"project_name": map[string]any{
				"type":    "string",
				"default": "myproject",
			},
		},
	}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), data, 0o644))

	// Template file: {{vars.project_name}}/main.go
	projDir := filepath.Join(dir, "{{vars.project_name}}")
	require.NoError(t, os.MkdirAll(projDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "main.go"), []byte("package main\n"), 0o644))

	return dir
}

// setupGenerateTemplate creates a template root with a .tag/ generator directory.
// Returns the template root path.
func setupGenerateTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create .tag/mygen/ with a generator template.
	genDir := filepath.Join(dir, ".tag", "mygen")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	tmpl := "---\nto: {{ name }}.go\n---\npackage {{ name }}\n"
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "gen.go"), []byte(tmpl), 0o644))

	return dir
}

func TestUT_Run_ScaffoldFixture(t *testing.T) {
	templateRoot := setupScaffoldTemplate(t)

	// Create fixture file.
	fixtureDir := t.TempDir()
	fixture := templatetest.Fixture{
		Name:     "scaffold basic",
		Mode:     templatetest.ModeScaffold,
		Template: ".",
		Vars: map[string]any{
			"project_name": "myproject",
		},
		Assertions: []templatetest.Assertion{
			{Type: templatetest.AssertFileExists, Path: "myproject/main.go"},
			{Type: templatetest.AssertContentContains, Path: "myproject/main.go", Value: "package main"},
		},
	}
	fixturePath := writeFixture(t, fixtureDir, "scaffold-test", fixture)

	report, err := templatetest.Run(context.Background(), templatetest.RunOptions{
		TemplateRoot: templateRoot,
		FixtureFiles: []string{fixturePath},
	})
	require.NoError(t, err)
	require.Len(t, report.Cases, 1)
	assert.Equal(t, templatetest.CasePassed, report.Cases[0].Status, "case failed: %+v", report.Cases[0])
	assert.Equal(t, 1, report.Passed)
	assert.Equal(t, 0, report.Failed)
}

func TestUT_Run_GenerateFixture(t *testing.T) {
	templateRoot := setupGenerateTemplate(t)

	fixtureDir := t.TempDir()
	fixture := templatetest.Fixture{
		Name:     "generate basic",
		Mode:     templatetest.ModeGenerate,
		Template: "mygen",
		Target:   "widget",
		Assertions: []templatetest.Assertion{
			{Type: templatetest.AssertFileExists, Path: "widget.go"},
			{Type: templatetest.AssertContentContains, Path: "widget.go", Value: "package widget"},
		},
	}
	fixturePath := writeFixture(t, fixtureDir, "generate-test", fixture)

	report, err := templatetest.Run(context.Background(), templatetest.RunOptions{
		TemplateRoot: templateRoot,
		FixtureFiles: []string{fixturePath},
	})
	require.NoError(t, err)
	require.Len(t, report.Cases, 1)
	assert.Equal(t, templatetest.CasePassed, report.Cases[0].Status, "case failed: %+v", report.Cases[0])
	assert.Equal(t, 1, report.Passed)
}

func TestUT_Run_Integration_MultipleFixtures(t *testing.T) {
	templateRoot := setupScaffoldTemplate(t)

	fixtureDir := t.TempDir()

	// Fixture 1: passing scaffold test.
	f1 := templatetest.Fixture{
		Name:     "pass case",
		Mode:     templatetest.ModeScaffold,
		Template: ".",
		Vars:     map[string]any{"project_name": "myproject"},
		Assertions: []templatetest.Assertion{
			{Type: templatetest.AssertFileExists, Path: "myproject/main.go"},
		},
	}
	path1 := writeFixture(t, fixtureDir, "pass", f1)

	// Fixture 2: failing scaffold test (assert nonexistent file exists).
	f2 := templatetest.Fixture{
		Name:     "fail case",
		Mode:     templatetest.ModeScaffold,
		Template: ".",
		Vars:     map[string]any{"project_name": "myproject"},
		Assertions: []templatetest.Assertion{
			{Type: templatetest.AssertFileExists, Path: "myproject/nonexistent.txt"},
		},
	}
	path2 := writeFixture(t, fixtureDir, "fail", f2)

	report, err := templatetest.Run(context.Background(), templatetest.RunOptions{
		TemplateRoot: templateRoot,
		FixtureFiles: []string{path1, path2},
	})
	require.NoError(t, err)
	assert.Len(t, report.Cases, 2)
	assert.Equal(t, 1, report.Passed)
	assert.Equal(t, 1, report.Failed)
	assert.Equal(t, 1, report.ExitCode())
}

func TestUT_Run_ScaffoldFixture_WithSetupFiles(t *testing.T) {
	templateRoot := setupScaffoldTemplate(t)

	fixtureDir := t.TempDir()
	fixture := templatetest.Fixture{
		Name:     "scaffold with setup files",
		Mode:     templatetest.ModeScaffold,
		Template: ".",
		Vars:     map[string]any{"project_name": "myproject"},
		Assertions: []templatetest.Assertion{
			{Type: templatetest.AssertFileExists, Path: "myproject/main.go"},
		},
	}
	fixturePath := writeFixture(t, fixtureDir, "setup", fixture)

	report, err := templatetest.Run(context.Background(), templatetest.RunOptions{
		TemplateRoot: templateRoot,
		FixtureFiles: []string{fixturePath},
	})
	require.NoError(t, err)
	require.Len(t, report.Cases, 1)
	assert.Equal(t, templatetest.CasePassed, report.Cases[0].Status)
}

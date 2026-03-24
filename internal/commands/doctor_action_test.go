package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/pkg/app"
)

func TestUT_DoctorAction_AllSectionsPresent(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	var buf bytes.Buffer
	_ = doctorAction(context.Background(), &buf, "dev")

	out := buf.String()
	assert.Contains(t, out, "ENVIRONMENT")
	assert.Contains(t, out, "PROJECT")
	assert.Contains(t, out, "TEMPLATES")
	assert.Contains(t, out, "LIBRARIES")
}

func TestUT_DoctorAction_FailExitCode(t *testing.T) {
	// Create a directory where .tag is a file (causing a fail)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, types.TemplatesDir), []byte("x"), 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	t.Setenv("GITHUB_TOKEN", "test-token")

	var buf bytes.Buffer
	err = doctorAction(context.Background(), &buf, "dev")
	require.Error(t, err)

	var cmdErr *app.CommandError
	require.ErrorAs(t, err, &cmdErr)
	assert.Equal(t, doctorExitFailures, cmdErr.Code)
}

func TestUT_DoctorCheckTemplates_ValidTemplate(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, types.TemplatesDir)
	tmplDir := filepath.Join(tagDir, "valid-tmpl")
	require.NoError(t, os.MkdirAll(tmplDir, 0o750))

	// Write a minimal valid tag.template.json
	cfg := `{"name": "test", "vars": {"project_name": {"type": "string", "prompt": "Name?"}}}`
	require.NoError(t, os.WriteFile(
		filepath.Join(tmplDir, types.TemplateConfigFile),
		[]byte(cfg),
		0o644,
	))

	// Write a template file that the linter can check
	wrapperDir := filepath.Join(tmplDir, "{{ vars.project_name }}")
	require.NoError(t, os.MkdirAll(wrapperDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(wrapperDir, "main.go"),
		[]byte("package main\n"),
		0o644,
	))

	results := doctorCheckTemplates(dir)
	require.NotEmpty(t, results)

	// At least one result should reference the template
	var found bool
	for _, r := range results {
		if r.status == doctorPass || r.status == doctorWarn {
			found = true
		}
	}
	assert.True(t, found, "expected at least one pass or warn for valid template")
}

func TestUT_DoctorCheckTemplates_SkipsDirsWithoutConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tagDir := filepath.Join(dir, types.TemplatesDir)
	// Create a directory without tag.template.json — should not be treated as a template
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, "not-a-template"), 0o750))

	results := doctorCheckTemplates(dir)
	require.Len(t, results, 1)
	assert.Equal(t, doctorPass, results[0].status)
	assert.Contains(t, results[0].label, "none found")
}

func TestUT_DoctorCheckTemplates_SkipsUnderscoreDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tagDir := filepath.Join(dir, types.TemplatesDir)
	// _shared and _bundles should be skipped
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, types.SharedDir), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, types.BundlesDir), 0o750))

	results := doctorCheckTemplates(dir)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].label, "none found")
}

func TestUT_DoctorCheckLibraries_WithInstalledTemplate(t *testing.T) {
	// Cannot use t.Parallel() — uses setupFakeLibrary via newLocalLibrary override
	dataDir := t.TempDir()

	tmplDir := filepath.Join(dataDir, "templates", "test-lib")
	require.NoError(t, os.MkdirAll(tmplDir, 0o750))

	reg := library.Registry{
		Version: 1,
		Entries: map[string]*library.Entry{
			"test-lib": {
				Name:    "test-lib",
				Source:  "gh:test/lib",
				AddedAt: time.Now(),
			},
		},
	}
	regData, err := json.Marshal(reg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "library.json"), regData, 0o644))

	// Override XDG data home to use our temp dir
	t.Setenv("XDG_DATA_HOME", dataDir)

	results := doctorCheckLibraries()
	require.NotEmpty(t, results)

	var foundPass bool
	for _, r := range results {
		if r.status == doctorPass {
			foundPass = true
		}
	}
	assert.True(t, foundPass, "expected pass for accessible library template")
}

func TestUT_DoctorCheckEnvironment_IncludesAllChecks(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test")

	results := doctorCheckEnvironment(context.Background(), "dev")
	require.Len(t, results, 3) // git, github token, version

	labels := make([]string, len(results))
	for i, r := range results {
		labels[i] = r.label
	}
	assert.Contains(t, labels[0], "Git")
	assert.Contains(t, labels[1], "GITHUB_TOKEN")
	assert.Contains(t, labels[2], "TAG version")
}

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
	// doctorAction -> doctorCheckLibraries reads $HOME/$XDG_DATA_HOME directly
	// (it does not go through the overridable newLocalLibrary var), so without
	// seedHome this would read the developer's real library and touch their
	// real ~/.tag/cache. Not parallel: seedHome uses t.Setenv.
	seedHome(t)
	t.Setenv("GITHUB_TOKEN", "test-token")

	var buf bytes.Buffer
	_ = doctorAction(context.Background(), &buf, "dev", formatText)

	out := buf.String()
	assert.Contains(t, out, "ENVIRONMENT")
	assert.Contains(t, out, "PROJECT")
	assert.Contains(t, out, "TEMPLATES")
	assert.Contains(t, out, "LIBRARIES")
}

func TestUT_DoctorAction_FailExitCode(t *testing.T) {
	// Not parallel: seedHome and os.Chdir below use t.Setenv/mutate process state.
	seedHome(t)

	// Create a directory where .tag is a file (causing a fail)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, types.TemplatesDir), []byte("x"), 0o644))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	t.Setenv("GITHUB_TOKEN", "test-token")

	var buf bytes.Buffer
	err = doctorAction(context.Background(), &buf, "dev", formatText)
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
		if r.Status == doctorPass || r.Status == doctorWarn {
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
	assert.Equal(t, doctorPass, results[0].Status)
	assert.Contains(t, results[0].Label, "none found")
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
	assert.Contains(t, results[0].Label, "none found")
}

func TestUT_DoctorCheckLibraries_WithInstalledTemplate(t *testing.T) {
	// doctorCheckLibraries does NOT go through the overridable newLocalLibrary
	// var — it calls xdg.DataHome()+library.New directly, and library.New also
	// builds a resolver that touches $HOME/.tag/cache. seedHome isolates both.
	// Not parallel: seedHome uses t.Setenv.
	//
	// xdg.DataHome() resolves to $XDG_DATA_HOME/tag, so the registry and
	// template dir must live under seedHome's home/data/tag, not directly
	// under home/data — the previous version of this test wrote the registry
	// one level too shallow, so doctorCheckLibraries never saw it, silently
	// fell back to the "no libraries installed" pass branch, and the
	// "expected pass" assertion passed for the wrong reason.
	home := seedHome(t)
	libDataDir := filepath.Join(home, "data", "tag")

	tmplDir := filepath.Join(libDataDir, "templates", "test-lib")
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
	require.NoError(t, os.WriteFile(filepath.Join(libDataDir, "library.json"), regData, 0o644))

	results := doctorCheckLibraries()
	require.NotEmpty(t, results)

	var foundPass bool
	for _, r := range results {
		if r.Status == doctorPass {
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
		labels[i] = r.Label
	}
	assert.Contains(t, labels[0], "Git")
	assert.Contains(t, labels[1], "GITHUB_TOKEN")
	assert.Contains(t, labels[2], "TAG version")
}

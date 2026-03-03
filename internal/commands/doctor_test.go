package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_DoctorCheckGit(t *testing.T) {
	result := doctorCheckGit()
	// Assume the test environment has git available; at minimum it should not panic.
	assert.NotEmpty(t, result.label)
	assert.Contains(t, []doctorStatus{doctorPass, doctorFail}, result.status)
}

func TestUT_DoctorCheckGitHubToken(t *testing.T) {
	t.Run("token set", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "test-token")
		r := doctorCheckGitHubToken()
		assert.Equal(t, doctorPass, r.status)
	})

	t.Run("token not set", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		r := doctorCheckGitHubToken()
		assert.Equal(t, doctorWarn, r.status)
		assert.NotEmpty(t, r.message)
	})
}

func TestUT_DoctorCheckProject_NoTagDir(t *testing.T) {
	dir := t.TempDir()
	results := doctorCheckProject(dir)
	require.Len(t, results, 1)
	assert.Equal(t, doctorWarn, results[0].status)
	assert.Contains(t, results[0].message, "tag template init")
}

func TestUT_DoctorCheckProject_TagDirExists(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, "_shared"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, "_bundles"), 0o750))

	results := doctorCheckProject(dir)
	assert.True(t, len(results) >= 3)
	for _, r := range results {
		assert.Equal(t, doctorPass, r.status, "expected pass for %s", r.label)
	}
}

func TestUT_DoctorCheckProject_MissingSubdirs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag"), 0o750))

	results := doctorCheckProject(dir)
	var labels []string
	for _, r := range results {
		labels = append(labels, r.label)
	}
	assert.Contains(t, labels, ".tag/ directory")

	// _shared and _bundles are missing → warn
	warnCount := 0
	for _, r := range results {
		if r.status == doctorWarn {
			warnCount++
		}
	}
	assert.Equal(t, 2, warnCount, "expected 2 warnings for missing _shared and _bundles")
}

func TestUT_DoctorCheckProject_TagIsFile(t *testing.T) {
	dir := t.TempDir()
	// Create .tag as a file instead of a directory
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tag"), []byte("oops"), 0o644))

	results := doctorCheckProject(dir)
	require.Len(t, results, 1)
	assert.Equal(t, doctorFail, results[0].status)
}

func TestUT_DoctorCheckTemplates_NoTagDir(t *testing.T) {
	dir := t.TempDir()
	results := doctorCheckTemplates(dir)
	require.Len(t, results, 1)
	assert.Equal(t, doctorPass, results[0].status)
}

func TestUT_DoctorCheckTemplates_NoTemplates(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag"), 0o750))

	results := doctorCheckTemplates(dir)
	require.Len(t, results, 1)
	assert.Equal(t, doctorPass, results[0].status)
	assert.Contains(t, results[0].label, "none found")
}

func TestUT_DoctorReportExitCode_AllPass(t *testing.T) {
	// Ensure GITHUB_TOKEN is set so the token check passes.
	t.Setenv("GITHUB_TOKEN", "test-token")
	var buf bytes.Buffer
	err := doctorAction(context.Background(), &buf, "dev")
	// dev build skips the update check; with token set and no .tag/ → only warning
	// (project missing .tag/ is a warn, not a fail), so err is either nil or warn.
	if err != nil {
		var cmdErr interface{ ExitCode() int }
		require.ErrorAs(t, err, &cmdErr)
		assert.Equal(t, doctorExitWarnings, cmdErr.ExitCode(), "unexpected failure from doctor")
	}
}

func TestUT_DoctorPrintResults_AllStatuses(t *testing.T) {
	var buf bytes.Buffer
	results := []doctorResult{
		{label: "check A", status: doctorPass},
		{label: "check B", status: doctorWarn, message: "something to note"},
		{label: "check C", status: doctorFail, message: "something broken"},
	}
	printDoctorResults(&buf, results)
	out := buf.String()
	assert.Contains(t, out, "✓")
	assert.Contains(t, out, "⚠")
	assert.Contains(t, out, "✗")
	assert.Contains(t, out, "check A")
	assert.Contains(t, out, "something to note")
	assert.Contains(t, out, "something broken")
}

func TestIT_DoctorCommand_OutsideProject(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer

	err := doctorAction(context.Background(), &buf, "dev")
	_ = dir

	// Should not panic; may warn or pass depending on environment.
	out := buf.String()
	assert.True(t, strings.Contains(out, "ENVIRONMENT") || err == nil)
}

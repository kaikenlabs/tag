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

func TestUT_DoctorCheckGit_PassWhenGitInstalled(t *testing.T) {
	t.Parallel()

	result := doctorCheckGit()
	// Git is installed in the test environment.
	assert.Equal(t, doctorPass, result.status)
	assert.Equal(t, "Git installed", result.label)
}

func TestUT_DoctorCheckGitHubToken_WarnWhenUnset(t *testing.T) {
	// Not parallel — modifies env.
	t.Setenv("GITHUB_TOKEN", "")

	result := doctorCheckGitHubToken()
	assert.Equal(t, doctorWarn, result.status)
	assert.Contains(t, result.message, "not set")
}

func TestUT_DoctorCheckGitHubToken_PassWhenSet(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_test123")

	result := doctorCheckGitHubToken()
	assert.Equal(t, doctorPass, result.status)
}

func TestUT_DoctorCheckSubdir_Exists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sub := "child"
	require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), types.DirMode))

	results := doctorCheckSubdir(dir, sub, "test-label")
	require.Len(t, results, 1)
	assert.Equal(t, doctorPass, results[0].status)
}

func TestUT_DoctorCheckSubdir_Missing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	results := doctorCheckSubdir(dir, "nope", "test-label")
	require.Len(t, results, 1)
	assert.Equal(t, doctorWarn, results[0].status)
	assert.Contains(t, results[0].message, "not found")
}

func TestUT_DoctorCheckProject_TagDirNotADir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create a file named .tag instead of a directory.
	require.NoError(t, os.WriteFile(filepath.Join(dir, types.TemplatesDir), []byte("x"), types.FileMode))

	results := doctorCheckProject(dir)
	require.NotEmpty(t, results)
	assert.Equal(t, doctorFail, results[0].status)
	assert.Contains(t, results[0].message, "not a directory")
}

func TestUT_DoctorCheckProject_WithAllSubdirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tagDir := filepath.Join(dir, types.TemplatesDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, types.SharedDir), types.DirMode))
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, types.BundlesDir), types.DirMode))

	results := doctorCheckProject(dir)
	for _, r := range results {
		assert.Equal(t, doctorPass, r.status, "expected pass for %s", r.label)
	}
}

func TestUT_PrintDoctorResults_FormatsCorrectly(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	results := []doctorResult{
		doctorResultPass("check-ok"),
		doctorResultWarn("check-warn", "something odd"),
		doctorResultFail("check-fail", "broken"),
	}

	printDoctorResults(&buf, results)
	out := buf.String()

	assert.Contains(t, out, "check-ok")
	assert.Contains(t, out, "check-warn")
	assert.Contains(t, out, "something odd")
	assert.Contains(t, out, "check-fail")
	assert.Contains(t, out, "broken")
}

func TestUT_DoctorCheckTAGVersion_DevBuild(t *testing.T) {
	t.Parallel()

	result := doctorCheckTAGVersion(t.Context(), "dev")
	assert.Equal(t, doctorPass, result.status)
	assert.Contains(t, result.label, "dev")
}

func TestUT_DoctorResultConstructors(t *testing.T) {
	t.Parallel()

	pass := doctorResultPass("ok")
	assert.Equal(t, doctorPass, pass.status)
	assert.Equal(t, "ok", pass.label)
	assert.Empty(t, pass.message)

	warn := doctorResultWarn("w", "msg")
	assert.Equal(t, doctorWarn, warn.status)
	assert.Equal(t, "msg", warn.message)

	fail := doctorResultFail("f", "err")
	assert.Equal(t, doctorFail, fail.status)
	assert.Equal(t, "err", fail.message)
}

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

	"github.com/kaikenlabs/tag/internal/jsonout"
)

func TestUT_DoctorCheckGit(t *testing.T) {
	result := doctorCheckGit()
	// Assume the test environment has git available; at minimum it should not panic.
	assert.NotEmpty(t, result.Label)
	assert.Contains(t, []doctorStatus{doctorPass, doctorFail}, result.Status)
}

func TestUT_DoctorCheckGitHubToken(t *testing.T) {
	t.Run("token set", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "test-token")
		r := doctorCheckGitHubToken()
		assert.Equal(t, doctorPass, r.Status)
	})

	t.Run("token not set", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		r := doctorCheckGitHubToken()
		assert.Equal(t, doctorWarn, r.Status)
		assert.NotEmpty(t, r.Message)
	})
}

func TestUT_DoctorCheckProject_NoTagDir(t *testing.T) {
	dir := t.TempDir()
	results := doctorCheckProject(dir)
	require.Len(t, results, 1)
	assert.Equal(t, doctorWarn, results[0].Status)
	assert.Contains(t, results[0].Message, "tag template init")
}

func TestUT_DoctorCheckProject_TagDirExists(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, "_shared"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(tagDir, "_bundles"), 0o750))

	results := doctorCheckProject(dir)
	assert.True(t, len(results) >= 3)
	for _, r := range results {
		assert.Equal(t, doctorPass, r.Status, "expected pass for %s", r.Label)
	}
}

func TestUT_DoctorCheckProject_MissingSubdirs(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag"), 0o750))

	results := doctorCheckProject(dir)
	var labels []string
	for _, r := range results {
		labels = append(labels, r.Label)
	}
	assert.Contains(t, labels, ".tag/ directory")

	// _shared and _bundles are missing → warn
	warnCount := 0
	for _, r := range results {
		if r.Status == doctorWarn {
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
	assert.Equal(t, doctorFail, results[0].Status)
}

func TestUT_DoctorCheckTemplates_NoTagDir(t *testing.T) {
	dir := t.TempDir()
	results := doctorCheckTemplates(dir)
	require.Len(t, results, 1)
	assert.Equal(t, doctorPass, results[0].Status)
}

func TestUT_DoctorCheckTemplates_NoTemplates(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag"), 0o750))

	results := doctorCheckTemplates(dir)
	require.Len(t, results, 1)
	assert.Equal(t, doctorPass, results[0].Status)
	assert.Contains(t, results[0].Label, "none found")
}

func TestUT_DoctorReportExitCode_AllPass(t *testing.T) {
	// doctorAction -> doctorCheckLibraries reads $HOME/$XDG_DATA_HOME directly,
	// bypassing the overridable newLocalLibrary var, so without seedHome this
	// would read the developer's real library. Not parallel: seedHome and
	// t.Setenv below mutate process env.
	seedHome(t)
	// Ensure GITHUB_TOKEN is set so the token check passes.
	t.Setenv("GITHUB_TOKEN", "test-token")
	var buf bytes.Buffer
	err := doctorAction(context.Background(), &buf, "dev", formatText)
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
	results := []DoctorResult{
		{Label: "check A", Status: doctorPass},
		{Label: "check B", Status: doctorWarn, Message: "something to note"},
		{Label: "check C", Status: doctorFail, Message: "something broken"},
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
	// The temp dir was previously allocated and never used (no chdir into
	// it), so doctorCheckProject scanned this package's real checkout
	// directory instead of an empty one. seedHome additionally isolates
	// doctorCheckLibraries, which reads $HOME/$XDG_DATA_HOME directly and
	// bypasses the overridable newLocalLibrary var. Not parallel: t.Chdir and
	// seedHome mutate process state.
	seedHome(t)
	dir := t.TempDir()
	t.Chdir(dir)

	var buf bytes.Buffer
	err := doctorAction(context.Background(), &buf, "dev", formatText)

	// Should not panic; may warn or pass depending on environment.
	out := buf.String()
	assert.True(t, strings.Contains(out, "ENVIRONMENT") || err == nil)
}

// TestUT_DoctorReport_EmptyChecksSerializeAsEmptyArray covers #348's
// acceptance criterion that sections and checks serialise as [] and never
// null. json.Unmarshal cannot distinguish the two — both [] and null decode
// into a nil Go slice — so a regression here (e.g. an uninitialised `var
// checks []DoctorResult` replacing buildDoctorReport's make+append pattern)
// would be invisible to any assertion made after decoding. This asserts on
// the raw encoded bytes instead.
func TestUT_DoctorReport_EmptyChecksSerializeAsEmptyArray(t *testing.T) {
	t.Parallel()

	// Mirrors buildDoctorReport's construction: both Sections and each
	// section's Checks are built via make(..., 0, n), never left as a nil
	// slice, even when there are zero results to append.
	report := DoctorReport{
		Status:   doctorPass,
		Sections: []DoctorSection{{Name: "EMPTY", Checks: make([]DoctorResult, 0)}},
	}

	var buf bytes.Buffer
	require.NoError(t, jsonout.Write(&buf, report))

	out := buf.String()
	assert.Contains(t, out, `"checks": []`)
	assert.NotContains(t, out, "null")

	// The same guarantee must hold one level up: a report with zero sections
	// must serialise its "sections" key as [] too.
	buf.Reset()
	require.NoError(t, jsonout.Write(&buf, DoctorReport{Status: doctorPass, Sections: make([]DoctorSection, 0)}))
	out = buf.String()
	assert.Contains(t, out, `"sections": []`)
	assert.NotContains(t, out, "null")
}

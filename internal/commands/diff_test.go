package commands

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/templateupdate"
)

// newDiffContext builds a context from DiffCommand().Flags — never a
// hand-rolled flag list — so this helper cannot silently drift from the
// command it exercises.
func newDiffContext(t *testing.T, values map[string]string) *cli.Context {
	t.Helper()

	cmdFlags := DiffCommand().Flags
	app := &cli.App{Writer: io.Discard, Flags: cmdFlags}

	set := flag.NewFlagSet("diff-test", flag.ContinueOnError)
	for _, f := range cmdFlags {
		require.NoError(t, f.Apply(set))
	}
	for k, v := range values {
		require.NoError(t, set.Set(k, v))
	}
	require.NoError(t, set.Parse(nil))

	return cli.NewContext(app, set, nil)
}

func TestUT_DiffCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := DiffCommand()
	require.NotNil(t, cmd)

	assert.Equal(t, "diff", cmd.Name)
	require.NotNil(t, cmd.Action)

	// Flag arity is not a behavior — asserting a fixed count breaks every
	// time a flag is added. The name-set assertion below is what actually
	// matters.
	flagNames := map[string]bool{}
	for _, f := range cmd.Flags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}
	for _, n := range []string{"dir", "ref", "stat", "no-color", "format"} {
		assert.True(t, flagNames[n], "expected flag %q", n)
	}
}

func TestUT_IsStdoutTTY_WithRegularFile_ReturnsFalse(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "stdout-*")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tmp.Close())
	}()

	orig := os.Stdout
	os.Stdout = tmp
	t.Cleanup(func() {
		os.Stdout = orig
	})

	assert.False(t, isStdoutTTY())
}

func TestUT_DiffAction_MissingProjectConfig_ReturnsValidationError(t *testing.T) {
	t.Parallel()

	ctx := newDiffContext(t, map[string]string{"dir": t.TempDir()})
	err := diffAction(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "diff: load project config")
}

// --- #351: `tag diff --format json` -----------------------------------------

func TestUT_Diff_EmptyFormat_IsUsageError(t *testing.T) {
	t.Parallel()

	run := runCLI(t, DiffCommand(), "diff", "--dir", t.TempDir(), "--format=")
	require.Error(t, run.Err)
	assert.Contains(t, run.Err.Error(), `unsupported format ""`)
}

// TestUT_Diff_RejectsPositional is the regression test named in the ticket:
// `tag diff stray --format json` must not silently print text. urfave/cli
// stops parsing at the first non-flag token ("stray"), so --format never
// reaches the flag parser at all; diff must reject the positional itself
// rather than reparsing (it takes none).
func TestUT_Diff_RejectsPositional(t *testing.T) {
	t.Parallel()

	run := runCLI(t, DiffCommand(), "diff", "stray", "--format", "json")
	require.Error(t, run.Err)
	assert.NotContains(t, run.Err.Error(), "unsupported format",
		"a positional-argument error must not be reported as a format error")
}

func TestUT_DiffJSON_UpToDateEmitsSummary(t *testing.T) {
	// seedProject mutates package-level newGitResolver — no t.Parallel.
	dir := seedProject(t, "abc1234567890", "abc1234567890")

	run := runCLI(t, DiffCommand(), "diff", "--dir", dir, "--format", formatJSON)
	require.NoError(t, run.Err)

	var summary templateupdate.DiffSummary
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &summary), "output: %s", run.Writer)
	assert.Equal(t, "abc1234567890", summary.OldSHA)
	assert.Equal(t, "abc1234567890", summary.NewSHA)
	assert.NotNil(t, summary.Files)
	assert.Empty(t, summary.Files)
	assert.NotContains(t, run.Writer, "Already up to date",
		"a script must receive a JSON body, not the text sentinel, even when up to date")
}

// TestUT_Diff_ExitCodeUnchangedAcrossFormats compares the actual exit CODE
// between the two formats, not just the error text. An earlier version of this
// test asserted only that both runs failed with a similar message, which would
// have passed even if one format exited 1 and the other 2 — i.e. it did not
// test the property its name claims. Both the failure path and the success
// path are covered, since a format that diverged only on success would
// otherwise slip through.
func TestUT_Diff_ExitCodeUnchangedAcrossFormats(t *testing.T) {
	// seedProject mutates the package-level newGitResolver — no t.Parallel.
	upToDate := seedProject(t, "abc1234567890", "abc1234567890")
	missing := t.TempDir()

	codes := map[string]map[string]int{}
	for _, format := range []string{formatText, formatJSON} {
		codes[format] = map[string]int{
			"up-to-date":     exitCodeOf(runCLI(t, DiffCommand(), "diff", "--dir", upToDate, "--format", format).Err),
			"missing-config": exitCodeOf(runCLI(t, DiffCommand(), "diff", "--dir", missing, "--format", format).Err),
		}
	}

	assert.Equal(t, codes[formatText], codes[formatJSON],
		"exit codes must not depend on --format")
	assert.Equal(t, 0, codes[formatJSON]["up-to-date"], "up-to-date must exit 0 in both formats")
	assert.NotEqual(t, 0, codes[formatJSON]["missing-config"], "a missing project config must be non-zero in both formats")
}

// exitCodeOf extracts the process exit code an action's error would produce:
// nil means 0, a cli.ExitCoder carries its own code, and any other error is
// urfave/cli's default of 1.
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var coder cli.ExitCoder
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}
	return 1
}

func TestUT_Diff_JSONIgnoresPresentationFlags(t *testing.T) {
	// seedProject mutates package-level newGitResolver — no t.Parallel.
	dir := seedProject(t, "abc1234567890", "abc1234567890")

	run := runCLI(t, DiffCommand(), "diff", "--dir", dir, "--format", formatJSON, "--stat", "--no-color")
	require.NoError(t, run.Err, "--stat/--no-color must be accepted (not rejected) under --format json")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed), "output: %s", run.Writer)
}

func TestUT_DiffJSON_LeavesStdoutClean(t *testing.T) {
	// seedProject mutates package-level newGitResolver; runCLICapturingStdout
	// replaces the process os.Stdout — no t.Parallel for either reason.
	dir := seedProject(t, "abc1234567890", "abc1234567890")

	run := runCLICapturingStdout(t, DiffCommand(), "diff", "--dir", dir, "--format", formatJSON)
	require.NoError(t, run.Err)
	assert.Empty(t, run.Stdout, "a JSON command must not bypass c.App.Writer")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed), "writer did not hold the JSON: %s", run.Writer)
}

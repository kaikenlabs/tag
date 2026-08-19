package commands

import (
	"encoding/json"
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

func TestUT_Diff_ExitCodeUnchangedAcrossFormats(t *testing.T) {
	t.Parallel()

	for _, format := range []string{formatText, formatJSON} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			run := runCLI(t, DiffCommand(), "diff", "--dir", t.TempDir(), "--format", format)
			require.Error(t, run.Err)
			assert.Contains(t, run.Err.Error(), "diff: load project config")
		})
	}
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

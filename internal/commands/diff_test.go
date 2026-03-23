package commands

import (
	"flag"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func newDiffContext(t *testing.T, values map[string]string) *cli.Context {
	t.Helper()

	app := &cli.App{
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dir", Value: "."},
			&cli.StringFlag{Name: "ref"},
			&cli.BoolFlag{Name: "stat"},
			&cli.BoolFlag{Name: "no-color"},
		},
	}

	set := flag.NewFlagSet("diff-test", flag.ContinueOnError)
	for _, f := range app.Flags {
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
	require.Len(t, cmd.Flags, 4)

	flagNames := map[string]bool{}
	for _, f := range cmd.Flags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}
	for _, n := range []string{"dir", "ref", "stat", "no-color"} {
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

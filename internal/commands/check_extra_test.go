package commands

import (
	"flag"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func newCheckContext(t *testing.T, values map[string]string) *cli.Context {
	t.Helper()

	app := &cli.App{
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dir", Value: "."},
			&cli.StringFlag{Name: "ref"},
			&cli.BoolFlag{Name: "quiet"},
		},
	}
	set := flag.NewFlagSet("check-test", flag.ContinueOnError)
	for _, f := range app.Flags {
		require.NoError(t, f.Apply(set))
	}
	for k, v := range values {
		require.NoError(t, set.Set(k, v))
	}
	require.NoError(t, set.Parse(nil))

	return cli.NewContext(app, set, nil)
}

func TestUT_CheckCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := CheckCommand()
	require.NotNil(t, cmd)

	assert.Equal(t, "check", cmd.Name)
	require.NotNil(t, cmd.Action)
	require.Len(t, cmd.Flags, 3)

	flagNames := map[string]bool{}
	for _, f := range cmd.Flags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}
	assert.True(t, flagNames["dir"])
	assert.True(t, flagNames["ref"])
	assert.True(t, flagNames["quiet"])
}

func TestUT_NewGitResolver_ReturnsResolver(t *testing.T) {
	t.Parallel()

	resolver := newGitResolver()
	assert.NotNil(t, resolver)
}

func TestUT_CheckAction_MissingProjectConfig_ReturnsValidationError(t *testing.T) {
	t.Parallel()

	ctx := newCheckContext(t, map[string]string{"dir": t.TempDir(), "quiet": "true"})

	err := checkAction(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check: load project config")
}

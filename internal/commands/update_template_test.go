package commands

import (
	"flag"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

// --- parseSetFlags ---

func TestUT_ParseSetFlags_ValidPairs(t *testing.T) {
	t.Parallel()
	result, err := parseSetFlags([]string{"name=foo", "version=1.0"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"name": "foo", "version": "1.0"}, result)
}

func TestUT_ParseSetFlags_Empty(t *testing.T) {
	t.Parallel()
	result, err := parseSetFlags(nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestUT_ParseSetFlags_ValueWithEquals(t *testing.T) {
	t.Parallel()
	result, err := parseSetFlags([]string{"dsn=host=localhost dbname=mydb"})
	require.NoError(t, err)
	assert.Equal(t, "host=localhost dbname=mydb", result["dsn"])
}

func TestUT_ParseSetFlags_MissingEquals(t *testing.T) {
	t.Parallel()
	_, err := parseSetFlags([]string{"badformat"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected key=value format")
}

// --- updateTemplateAction flag validation ---

func newUpdateCLIContext(t *testing.T, flagValues map[string]string) *cli.Context {
	t.Helper()
	app := &cli.App{
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "continue"},
			&cli.BoolFlag{Name: "abort"},
			&cli.BoolFlag{Name: "accept-ours"},
			&cli.BoolFlag{Name: "accept-theirs"},
			&cli.BoolFlag{Name: "dry-run"},
			&cli.BoolFlag{Name: "backup"},
			&cli.BoolFlag{Name: "skip-hooks"},
			&cli.BoolFlag{Name: "accept-hooks"},
			&cli.StringFlag{Name: "dir", Value: "."},
			&cli.StringFlag{Name: "ref"},
			&cli.StringSliceFlag{Name: "set"},
			&cli.StringSliceFlag{Name: "skip"},
		},
	}

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range app.Flags {
		require.NoError(t, f.Apply(set))
	}
	for name, value := range flagValues {
		require.NoError(t, set.Set(name, value))
	}
	require.NoError(t, set.Parse(nil))

	return cli.NewContext(app, set, nil)
}

func TestUT_UpdateTemplateAction_ContinueAbortConflict(t *testing.T) {
	t.Parallel()
	ctx := newUpdateCLIContext(t, map[string]string{
		"continue": "true",
		"abort":    "true",
	})

	err := updateTemplateAction(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use --continue and --abort together")
}

func TestUT_UpdateTemplateAction_AcceptOursTheirsConflict(t *testing.T) {
	t.Parallel()
	ctx := newUpdateCLIContext(t, map[string]string{
		"accept-ours":   "true",
		"accept-theirs": "true",
	})

	err := updateTemplateAction(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use --accept-ours and --accept-theirs together")
}

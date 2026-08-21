package commands

import (
	"flag"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/types/flags"
)

func newScaffoldCLIContext(t *testing.T, flagValues map[string]string) *cli.Context {
	t.Helper()
	app := &cli.App{
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Value: "."},
			&cli.StringFlag{Name: "values"},
			&cli.BoolFlag{Name: "no-input"},
			&cli.BoolFlag{Name: "force"},
			&cli.BoolFlag{Name: "replay"},
			&cli.BoolFlag{Name: "no-save"},
			&cli.BoolFlag{Name: "accept-hooks"},
			&cli.BoolFlag{Name: "allow-recursive-render"},
			&cli.BoolFlag{Name: flags.UpdateLockFlag},
			&cli.BoolFlag{Name: flags.IgnoreLockFlag},
			&cli.BoolFlag{Name: flags.DryRunFlag},
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

func TestUT_BuildScaffoldOpts_AllFlagsMapped(t *testing.T) {
	t.Parallel()
	ctx := newScaffoldCLIContext(t, map[string]string{
		"output":                 "/out",
		"values":                 "vals.json",
		"no-input":               "true",
		"force":                  "true",
		"replay":                 "true",
		"no-save":                "true",
		"accept-hooks":           "true",
		"allow-recursive-render": "true",
		flags.UpdateLockFlag:     "true",
		flags.IgnoreLockFlag:     "true",
		flags.DryRunFlag:         "true",
	})

	meta := map[string]string{"key": "val"}
	opts := buildScaffoldOpts(ctx, "/tmpl", "myproject", meta, false)

	assert.Equal(t, "/tmpl", opts.TemplateDir)
	assert.Equal(t, "/out", opts.OutputDir)
	assert.Equal(t, "myproject", opts.ProjectName)
	assert.Equal(t, "vals.json", opts.ValuesFile)
	assert.Equal(t, meta, opts.Meta)
	assert.True(t, opts.NoInput)
	assert.True(t, opts.Force)
	assert.True(t, opts.Replay)
	assert.True(t, opts.NoSave)
	assert.True(t, opts.AcceptHooks)
	assert.True(t, opts.AllowRecursiveRender)
	assert.True(t, opts.UpdateLock)
	assert.True(t, opts.IgnoreLock)
	assert.True(t, opts.DryRun)
}

func TestUT_BuildScaffoldOpts_Defaults(t *testing.T) {
	t.Parallel()
	ctx := newScaffoldCLIContext(t, nil)

	opts := buildScaffoldOpts(ctx, "/tmpl", "proj", nil, false)

	assert.Equal(t, ".", opts.OutputDir)
	assert.False(t, opts.NoInput)
	assert.False(t, opts.Force)
	assert.False(t, opts.DryRun)
}

package commands

import (
	"bytes"
	"flag"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/templateupdate"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	// Restored via defer so a t.Fatal or panic inside fn cannot leave the
	// process-global pointing at a pipe nobody is draining.
	defer func() { os.Stdout = orig }()

	fn()

	require.NoError(t, w.Close())
	os.Stdout = orig

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	require.NoError(t, r.Close())

	return buf.String()
}

func TestUT_PrintUpdateSummary_PrintsAllOps(t *testing.T) {
	result := &templateupdate.UpdateResult{
		Applied: []templateupdate.MergeResult{
			{Path: "a.txt", Op: templateupdate.MergeAdd},
			{Path: "b.txt", Op: templateupdate.MergeUpdate},
			{Path: "c.txt", Op: templateupdate.MergeDelete},
			{Path: "d.txt", Op: templateupdate.MergeConflict},
		},
	}

	output := captureStdout(t, func() {
		printUpdateSummary(result)
	})

	assert.Contains(t, output, "+ a.txt (added)")
	assert.Contains(t, output, "✓ b.txt (updated)")
	assert.Contains(t, output, "- c.txt (deleted)")
	assert.Contains(t, output, "⚠ d.txt (conflict)")
}

func TestUT_UpdateTemplateCommand_Structure(t *testing.T) {
	t.Parallel()

	cmd := UpdateTemplateCommand()
	require.NotNil(t, cmd)

	assert.Equal(t, "update", cmd.Name)
	assert.Contains(t, cmd.Aliases, "up")
	require.NotNil(t, cmd.Action)
	assert.GreaterOrEqual(t, len(cmd.Flags), 12)

	flagNames := map[string]bool{}
	for _, f := range cmd.Flags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}

	for _, name := range []string{"dir", "ref", "set", "accept-ours", "accept-theirs", "skip", "dry-run", "backup", "continue", "abort", "skip-hooks", "accept-hooks"} {
		assert.True(t, flagNames[name], "expected flag %q", name)
	}
}

func TestUT_UpdateTemplateAction_InvalidSetValue_ValidationError(t *testing.T) {
	t.Parallel()

	app := &cli.App{
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "continue"},
			&cli.BoolFlag{Name: "abort"},
			&cli.BoolFlag{Name: "accept-ours"},
			&cli.BoolFlag{Name: "accept-theirs"},
			&cli.BoolFlag{Name: "dry-run"},
			&cli.BoolFlag{Name: "backup", Value: true},
			&cli.BoolFlag{Name: "skip-hooks"},
			&cli.BoolFlag{Name: "accept-hooks"},
			&cli.StringFlag{Name: "dir", Value: "."},
			&cli.StringFlag{Name: "ref"},
			&cli.StringSliceFlag{Name: "set"},
			&cli.StringSliceFlag{Name: "skip"},
		},
	}

	set := flag.NewFlagSet("update-test", flag.ContinueOnError)
	for _, f := range app.Flags {
		require.NoError(t, f.Apply(set))
	}
	require.NoError(t, set.Set("set", "not-a-pair"))
	require.NoError(t, set.Parse(nil))

	ctx := cli.NewContext(app, set, nil)
	err := updateTemplateAction(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected key=value format")
}

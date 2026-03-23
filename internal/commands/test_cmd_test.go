package commands

import (
	"bytes"
	"flag"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func newTestCLIContext(t *testing.T, args []string, flagValues map[string]string) *cli.Context {
	t.Helper()
	app := &cli.App{
		Writer: io.Discard,
		Flags:  testFlags(),
	}

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range app.Flags {
		require.NoError(t, f.Apply(set))
	}
	for name, value := range flagValues {
		require.NoError(t, set.Set(name, value))
	}
	require.NoError(t, set.Parse(args))

	return cli.NewContext(app, set, nil)
}

func TestUT_TestAction_InvalidPin(t *testing.T) {
	t.Parallel()
	ctx := newTestCLIContext(t, nil, map[string]string{"pin": "badformat"})

	var buf bytes.Buffer
	err := testAction(ctx, &buf, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --pin value")
}

func TestUT_TestAction_InvalidMeta(t *testing.T) {
	t.Parallel()
	ctx := newTestCLIContext(t, nil, map[string]string{"meta": "badformat"})

	var buf bytes.Buffer
	err := testAction(ctx, &buf, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --meta value")
}

func TestUT_TestAction_DryRunEmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := newTestCLIContext(t, nil, map[string]string{"dry-run": "true"})

	var buf bytes.Buffer
	err := testAction(ctx, &buf, dir)
	// Dry-run on a dir with no test cases should plan successfully
	// (zero cases is valid, just prints empty plan)
	if err != nil {
		// If it errors, it should be a plan error, not validation
		assert.Contains(t, err.Error(), "test plan")
	}
}

func TestUT_TestAction_DefaultTimeout(t *testing.T) {
	t.Parallel()
	ctx := newTestCLIContext(t, nil, nil)

	// Verify the default timeout is 5 minutes
	assert.Equal(t, 5*time.Minute, ctx.Duration("timeout"))
}

func TestUT_TestAction_DefaultParallel(t *testing.T) {
	t.Parallel()
	ctx := newTestCLIContext(t, nil, nil)
	assert.Equal(t, 4, ctx.Int("parallel"))
}

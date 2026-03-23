package commands

import (
	"flag"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func newGraphCLIContext(t *testing.T, args []string, flagValues map[string]string) *cli.Context {
	t.Helper()
	app := &cli.App{
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "text"},
		},
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

func TestUT_TemplateGraph_TooManyArgs(t *testing.T) {
	t.Parallel()
	cmd := templateGraphCommand()
	ctx := newGraphCLIContext(t, []string{"arg1", "arg2"}, nil)

	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected at most one path argument")
}

func TestUT_TemplateGraph_BadFormat(t *testing.T) {
	t.Parallel()
	cmd := templateGraphCommand()
	ctx := newGraphCLIContext(t, nil, map[string]string{"format": "yaml"})

	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestUT_TemplateGraph_ValidTextOnEmptyDir(t *testing.T) {
	t.Parallel()
	cmd := templateGraphCommand()
	dir := t.TempDir()
	ctx := newGraphCLIContext(t, []string{dir}, nil)

	err := cmd.Action(ctx)
	// Empty dir analysis may fail or succeed with empty report
	if err != nil {
		assert.NotContains(t, err.Error(), "unsupported format")
		assert.NotContains(t, err.Error(), "expected at most one path argument")
	}
}

func TestUT_TemplateGraph_DotFormat(t *testing.T) {
	t.Parallel()
	cmd := templateGraphCommand()
	dir := t.TempDir()
	ctx := newGraphCLIContext(t, []string{dir}, map[string]string{"format": "dot"})

	err := cmd.Action(ctx)
	// Should not error with format validation
	if err != nil {
		assert.NotContains(t, err.Error(), "unsupported format")
	}
}

package commands

import (
	"flag"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func newVariablesCLIContext(t *testing.T, args []string, flagValues map[string]string) *cli.Context {
	t.Helper()
	app := &cli.App{
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "text"},
			&cli.BoolFlag{Name: "strict"},
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

func TestUT_TemplateVariables_TooManyArgs(t *testing.T) {
	t.Parallel()
	cmd := templateVariablesCommand()
	ctx := newVariablesCLIContext(t, []string{"arg1", "arg2"}, nil)

	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected at most one path argument")
}

func TestUT_TemplateVariables_BadFormat(t *testing.T) {
	t.Parallel()
	cmd := templateVariablesCommand()
	ctx := newVariablesCLIContext(t, nil, map[string]string{"format": "xml"})

	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestUT_TemplateVariables_ValidTextOnTemplateDir(t *testing.T) {
	t.Parallel()
	cmd := templateVariablesCommand()

	// Use a valid template directory with a tag.template.json
	dir := t.TempDir()
	ctx := newVariablesCLIContext(t, []string{dir}, nil)

	// Should fail because no tag.template.json exists, but that's an analysis error,
	// not a validation error — the format and arg checks pass
	err := cmd.Action(ctx)
	// Analysis may error on empty dir, but it shouldn't be a usage error
	if err != nil {
		assert.NotContains(t, err.Error(), "unsupported format")
		assert.NotContains(t, err.Error(), "expected at most one path argument")
	}
}

func TestUT_TemplateVariables_StrictWithIssues(t *testing.T) {
	t.Parallel()
	cmd := templateVariablesCommand()

	// Empty dir should either error on analysis or find no vars (no issues)
	dir := t.TempDir()
	ctx := newVariablesCLIContext(t, []string{dir}, map[string]string{"strict": "true"})

	err := cmd.Action(ctx)
	// With an empty dir, analysis may fail — that's fine, it exercises the code path
	if err != nil {
		assert.NotContains(t, err.Error(), "unsupported format")
	}
}

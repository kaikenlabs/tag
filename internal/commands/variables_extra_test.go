package commands

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func newVarsCLIContext2(t *testing.T, args []string, flagValues map[string]string) *cli.Context {
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

func TestUT_TemplateVariables_JSONFormat(t *testing.T) {
	t.Parallel()

	// Create a template dir with tag.template.json containing vars.
	dir := t.TempDir()
	configJSON := `{"name":"test","vars":{"project_name":"default","use_db":{"type":"boolean","default":false}}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(configJSON), 0o644))

	cmd := templateVariablesCommand()
	ctx := newVarsCLIContext2(t, []string{dir}, map[string]string{"format": "json"})

	err := cmd.Action(ctx)
	require.NoError(t, err)
}

func TestUT_TemplateVariables_TextWithVars(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configJSON := `{"name":"test","vars":{"project_name":"myproject"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(configJSON), 0o644))

	cmd := templateVariablesCommand()
	ctx := newVarsCLIContext2(t, []string{dir}, nil)

	err := cmd.Action(ctx)
	require.NoError(t, err)
}

func TestUT_TemplateVariables_StrictWithUnusedVar(t *testing.T) {
	t.Parallel()

	// Declared var with no template files using it → flagged as unused issue.
	dir := t.TempDir()
	configJSON := `{"name":"test","vars":{"project_name":"default"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(configJSON), 0o644))

	cmd := templateVariablesCommand()
	ctx := newVarsCLIContext2(t, []string{dir}, map[string]string{"strict": "true"})

	err := cmd.Action(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "variable audit found issues")
}

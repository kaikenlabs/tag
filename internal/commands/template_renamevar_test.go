package commands

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/pkg/app"
)

func newRenameVarCLIContext(t *testing.T, args []string, dryRun bool) *cli.Context {
	t.Helper()
	cliApp := &cli.App{
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "dry-run"},
		},
	}

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliApp.Flags {
		require.NoError(t, f.Apply(set))
	}
	if dryRun {
		require.NoError(t, set.Set("dry-run", "true"))
	}
	require.NoError(t, set.Parse(args))

	return cli.NewContext(cliApp, set, nil)
}

// renameVarTemplate writes a minimal template and returns its root.
func renameVarTemplate(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "tag.template.json"),
		[]byte(`{"vars": {"old": {"type": "string"}}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"),
		[]byte("# {{ vars.old }}\n"), 0o644))
	return root
}

func TestUT_TemplateRenameVar_RegisteredAsSubcommand(t *testing.T) {
	t.Parallel()

	var names []string
	for _, sub := range TemplateCommand(nil).Subcommands {
		names = append(names, sub.Name)
	}
	assert.Contains(t, names, "rename-var")
}

func TestUT_TemplateRenameVar_RenamesTemplate(t *testing.T) {
	t.Parallel()

	root := renameVarTemplate(t)
	cmd := templateRenameVarCommand()
	ctx := newRenameVarCLIContext(t, []string{"old", "renamed", root}, false)

	require.NoError(t, cmd.Action(ctx))

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	require.NoError(t, err)
	assert.Equal(t, "# {{ vars.renamed }}\n", string(readme))

	config, err := os.ReadFile(filepath.Join(root, "tag.template.json"))
	require.NoError(t, err)
	assert.Contains(t, string(config), `"renamed"`)
}

func TestUT_TemplateRenameVar_DryRunDoesNotWrite(t *testing.T) {
	t.Parallel()

	root := renameVarTemplate(t)
	cmd := templateRenameVarCommand()
	ctx := newRenameVarCLIContext(t, []string{"old", "renamed", root}, true)

	require.NoError(t, cmd.Action(ctx))

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	require.NoError(t, err)
	assert.Equal(t, "# {{ vars.old }}\n", string(readme))
}

func TestUT_TemplateRenameVar_ArgumentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no arguments", args: nil, wantErr: "expected"},
		{name: "one argument", args: []string{"old"}, wantErr: "expected"},
		{name: "too many arguments", args: []string{"a", "b", "c", "d"}, wantErr: "expected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := templateRenameVarCommand()
			ctx := newRenameVarCLIContext(t, tt.args, false)

			err := cmd.Action(ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)

			var cmdErr *app.CommandError
			require.ErrorAs(t, err, &cmdErr)
			assert.Equal(t, app.ExitUsage, cmdErr.ExitCode())
		})
	}
}

func TestUT_TemplateRenameVar_PlanningFailureIsReported(t *testing.T) {
	t.Parallel()

	root := renameVarTemplate(t)
	cmd := templateRenameVarCommand()
	ctx := newRenameVarCLIContext(t, []string{"missing", "renamed", root}, false)

	err := cmd.Action(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not declared")
}

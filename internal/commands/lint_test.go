package commands

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/pkg/app"
)

const validTemplateConfig = `{
  "name": "test-template",
  "description": "A test template",
  "vars": {
    "project_name": "my-project"
  }
}`

func createLintTestTemplate(t *testing.T, dir, config string, files map[string]string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(config), 0o644))
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

func createLintCLIContext(t *testing.T, args []string, format string) *cli.Context {
	t.Helper()

	cliApp := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "text"},
		},
	}

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range cliApp.Flags {
		require.NoError(t, f.Apply(set))
	}
	if format != "" {
		require.NoError(t, set.Set("format", format))
	}
	require.NoError(t, set.Parse(args))

	return cli.NewContext(cliApp, set, nil)
}

func TestUT_LintCommand_ReturnsValidCommand(t *testing.T) {
	cmd := templateLintCommand()
	assert.Equal(t, "lint", cmd.Name)
	assert.NotEmpty(t, cmd.Usage)
	assert.NotNil(t, cmd.Action)

	// Check format flag exists
	hasFormatFlag := false
	for _, f := range cmd.Flags {
		if f.Names()[0] == "format" {
			hasFormatFlag = true
			break
		}
	}
	assert.True(t, hasFormatFlag, "expected --format flag")
}

func TestUT_LintCommand_NoErrors_ExitZero(t *testing.T) {
	dir := t.TempDir()
	createLintTestTemplate(t, dir, validTemplateConfig, map[string]string{
		"readme.txt": "# {{ vars.project_name }}",
	})

	ctx := createLintCLIContext(t, []string{dir}, "text")

	cmd := templateLintCommand()
	err := cmd.Action(ctx)
	assert.NoError(t, err)
}

func TestUT_LintCommand_WithErrors_ExitOne(t *testing.T) {
	dir := t.TempDir()
	createLintTestTemplate(t, dir, validTemplateConfig, map[string]string{
		"readme.txt": "{{ vars.undefined_var }}",
	})

	ctx := createLintCLIContext(t, []string{dir}, "text")

	cmd := templateLintCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)

	var cmdErr *app.CommandError
	require.ErrorAs(t, err, &cmdErr)
	assert.Equal(t, app.ExitGeneral, cmdErr.ExitCode())
}

func TestUT_LintCommand_FormatJSON(t *testing.T) {
	dir := t.TempDir()
	createLintTestTemplate(t, dir, validTemplateConfig, map[string]string{
		"readme.txt": "{{ vars.undefined_var }}",
	})

	ctx := createLintCLIContext(t, []string{dir}, "json")

	cmd := templateLintCommand()
	err := cmd.Action(ctx)
	require.Error(t, err) // Should have lint errors
}

func TestUT_LintCommand_InvalidFormat(t *testing.T) {
	dir := t.TempDir()
	createLintTestTemplate(t, dir, validTemplateConfig, nil)

	ctx := createLintCLIContext(t, []string{dir}, "xml")

	cmd := templateLintCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)

	var cmdErr *app.CommandError
	require.ErrorAs(t, err, &cmdErr)
	assert.Equal(t, app.ExitUsage, cmdErr.ExitCode())
}

func TestUT_LintCommand_MissingTemplate(t *testing.T) {
	dir := t.TempDir()
	// No tag.template.json

	ctx := createLintCLIContext(t, []string{dir}, "text")

	cmd := templateLintCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)

	var cmdErr *app.CommandError
	require.ErrorAs(t, err, &cmdErr)
	assert.Equal(t, app.ExitUsage, cmdErr.ExitCode())
}

func TestUT_LintCommand_NonexistentPath(t *testing.T) {
	ctx := createLintCLIContext(t, []string{"/nonexistent/path"}, "text")

	cmd := templateLintCommand()
	err := cmd.Action(ctx)
	require.Error(t, err)

	var cmdErr *app.CommandError
	require.ErrorAs(t, err, &cmdErr)
	assert.Equal(t, app.ExitUsage, cmdErr.ExitCode())
}

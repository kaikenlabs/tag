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

	"github.com/kaikenlabs/tag/internal/types/flags"
)

func newConvertCLIContext(t *testing.T, args []string, flagValues map[string]string) *cli.Context {
	t.Helper()
	app := &cli.App{
		Writer: io.Discard,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}},
			&cli.BoolFlag{Name: "force", Aliases: []string{"f"}},
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
	require.NoError(t, set.Parse(args))

	return cli.NewContext(app, set, nil)
}

func TestUT_ConvertCookiecutterAction_DryRun(t *testing.T) {
	t.Parallel()
	sourceDir := t.TempDir()

	// Create a minimal cookiecutter template
	ccJSON := `{"project_name": "my_project", "version": "0.1.0"}`
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "cookiecutter.json"), []byte(ccJSON), 0o644))

	projectDir := filepath.Join(sourceDir, "{{cookiecutter.project_name}}")
	require.NoError(t, os.MkdirAll(projectDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDir, "README.md"),
		[]byte("# {{cookiecutter.project_name}}\nVersion: {{cookiecutter.version}}"),
		0o644,
	))

	outDir := filepath.Join(t.TempDir(), "converted")
	ctx := newConvertCLIContext(t, []string{sourceDir}, map[string]string{
		"dry-run": "true",
		"output":  outDir,
	})

	err := convertCookiecutterAction(ctx)
	require.NoError(t, err)

	// Dry-run should not create the output directory
	_, statErr := os.Stat(outDir)
	assert.True(t, os.IsNotExist(statErr))
}

func TestUT_ConvertCookiecutterAction_HappyPath(t *testing.T) {
	t.Parallel()
	sourceDir := t.TempDir()

	// Create a minimal cookiecutter template
	ccJSON := `{"project_name": "my_project"}`
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "cookiecutter.json"), []byte(ccJSON), 0o644))

	projectDir := filepath.Join(sourceDir, "{{cookiecutter.project_name}}")
	require.NoError(t, os.MkdirAll(projectDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDir, "main.go"),
		[]byte("package {{cookiecutter.project_name}}"),
		0o644,
	))

	outDir := filepath.Join(t.TempDir(), "converted")
	ctx := newConvertCLIContext(t, []string{sourceDir}, map[string]string{
		"output": outDir,
	})

	err := convertCookiecutterAction(ctx)
	require.NoError(t, err)

	// Output directory should have been created
	require.DirExists(t, outDir)
}

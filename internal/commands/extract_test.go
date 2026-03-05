package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func newExtractTestApp() *cli.App {
	return &cli.App{
		Commands: []*cli.Command{ExtractCommand()},
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "dry-run"},
			&cli.StringFlag{Name: "path", Value: ".tag"},
		},
		ExitErrHandler: func(_ *cli.Context, _ error) {},
	}
}

func TestUT_ExtractCommand_MissingSourceFile(t *testing.T) {
	app := newExtractTestApp()

	err := app.Run([]string{"tag", "extract", "--name", "user", "--as", "handler"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "source file is required")
}

func TestUT_ExtractCommand_NonExistentFile(t *testing.T) {
	app := newExtractTestApp()

	err := app.Run([]string{"tag", "extract", "--name", "user", "--as", "handler", "/nonexistent/file.go"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUT_ExtractCommand_InvalidAsName(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "test.go")
	require.NoError(t, os.WriteFile(src, []byte("package main"), 0o644))

	app := newExtractTestApp()

	err := app.Run([]string{"tag", "extract", "--name", "user", "--as", "../escape", src})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid generator name")
}

func TestUT_ExtractCommand_DirectoryAsSource(t *testing.T) {
	dir := t.TempDir()
	app := newExtractTestApp()

	err := app.Run([]string{"tag", "extract", "--name", "user", "--as", "handler", dir})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

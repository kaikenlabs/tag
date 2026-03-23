package commands

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_ResolveTagDir_FindsNearestParentTagDir(t *testing.T) {
	project := t.TempDir()
	tagDir := filepath.Join(project, types.TemplatesDir)
	require.NoError(t, os.MkdirAll(tagDir, 0o755))

	nested := filepath.Join(project, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(nested))
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})

	resolved, err := resolveTagDir()
	require.NoError(t, err)
	assert.Equal(t, normalizePath(tagDir), normalizePath(resolved))
}

func TestUT_ResolveTagDir_FallbackToCurrentWorkingDir(t *testing.T) {
	project := t.TempDir()

	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(project))
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})

	resolved, err := resolveTagDir()
	require.NoError(t, err)
	assert.Equal(t, normalizePath(filepath.Join(project, types.TemplatesDir)), normalizePath(resolved))
}

func TestUT_PromptConfirm_NonInteractive_ReturnsFalseAndPrintsHint(t *testing.T) {
	origIn := os.Stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, w.Close())
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origIn
		_ = r.Close()
	})

	var out bytes.Buffer
	app := &cli.App{Writer: &out}
	ctx := cli.NewContext(app, flag.NewFlagSet("undo", flag.ContinueOnError), nil)

	ok := promptConfirm(ctx)
	assert.False(t, ok)
	assert.Contains(t, out.String(), "Non-interactive mode")
}

func TestUT_IsTerminal_WithPipeStdin_ReturnsFalse(t *testing.T) {
	origIn := os.Stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, w.Close())
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origIn
		_ = r.Close()
	})

	assert.False(t, isTerminal())
}

func normalizePath(p string) string {
	cleaned := filepath.Clean(p)
	return strings.TrimPrefix(cleaned, "/private")
}

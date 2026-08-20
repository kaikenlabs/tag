package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_HasSubdirScaffold_Exists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subdir := "scaffold"
	require.NoError(t, os.MkdirAll(filepath.Join(dir, subdir), 0o750))

	assert.True(t, hasSubdirScaffold(dir, subdir))
}

func TestUT_HasSubdirScaffold_Missing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	assert.False(t, hasSubdirScaffold(dir, "nonexistent"))
}

func TestUT_HasSubdirScaffold_FileNotDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notadir"), []byte("x"), 0o644))

	assert.False(t, hasSubdirScaffold(dir, "notadir"))
}

func TestUT_ResolveTemplateName_PositionalArg(t *testing.T) {
	t.Parallel()
	ctx := createTestCLIContext(t, []string{"my-template"}, nil)

	name, err := resolveTemplateName(ctx, nil, []string{"my-template"}, formatText)
	require.NoError(t, err)
	assert.Equal(t, "my-template", name)
}

func TestUT_ResolveTemplateName_NoArgs(t *testing.T) {
	t.Parallel()
	// resolveTemplateName with no positional args and non-TTY → error
	// (IsTTY returns false in test environment, so it hits the default case)
	ctx := createTestCLIContext(t, nil, nil)

	_, err := resolveTemplateName(ctx, nil, nil, formatText)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template argument required")
}

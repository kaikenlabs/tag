package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_LoadDialectRegistry_EmptyTemplateDirUsesGlobal(t *testing.T) {
	t.Parallel()
	reg, err := loadDialectRegistry("")
	require.NoError(t, err)
	assert.NotNil(t, reg)
}

func TestUT_LoadDialectRegistry_WithTemplateDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a template dir with a _dialects subdir
	dialectsDir := filepath.Join(dir, "_dialects")
	require.NoError(t, os.MkdirAll(dialectsDir, 0o750))

	reg, err := loadDialectRegistry(dir)
	require.NoError(t, err)
	assert.NotNil(t, reg)
}

func TestUT_LoadDialectRegistry_NonexistentTemplateDirStillWorks(t *testing.T) {
	t.Parallel()
	// A non-existent template dir should still work (falls back to builtins)
	reg, err := loadDialectRegistry("/nonexistent/template/dir")
	require.NoError(t, err)
	assert.NotNil(t, reg)
}

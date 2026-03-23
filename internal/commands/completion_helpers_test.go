package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_LibraryTemplateDir_NoTemplateOrigin(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	dir, ok := libraryTemplateDir(cfg)
	assert.False(t, ok)
	assert.Empty(t, dir)
}

func TestUT_LibraryTemplateDir_LibraryHasTemplate(t *testing.T) {
	// Cannot t.Parallel() — setupFakeLibrary mutates package state
	templateName := "completion-lib"
	templateDir := setupFakeLibrary(t, templateName)

	// Create the .tag dir inside the library template
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.TemplatesDir), 0o750))

	cfg := &config.Config{
		Template: &config.TemplateOrigin{
			Name:   templateName,
			Source: "gh:test/" + templateName,
		},
	}

	dir, ok := libraryTemplateDir(cfg)
	assert.True(t, ok)
	assert.Equal(t, templateDir, dir)
}

func TestUT_LibraryTemplateDir_LibraryMissingTemplate(t *testing.T) {
	// Cannot t.Parallel() — setupFakeLibrary mutates package state
	setupFakeLibrary(t, "existing-template")

	cfg := &config.Config{
		Template: &config.TemplateOrigin{
			Name:   "nonexistent-template",
			Source: "gh:test/nonexistent-template",
		},
	}

	dir, ok := libraryTemplateDir(cfg)
	assert.False(t, ok)
	assert.Empty(t, dir)
}

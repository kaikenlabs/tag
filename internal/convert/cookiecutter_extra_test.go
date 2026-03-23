package convert

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_Convert_NoCookiecutterConfig(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	dir := t.TempDir()
	// No cookiecutter.json present
	result, err := converter.Convert(context.Background(), Options{
		Source:      dir,
		Destination: filepath.Join(t.TempDir(), "out"),
	})
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNoCookiecutterConfig)
}

func TestUT_Convert_DryRun_NoCookiecutter(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	dir := t.TempDir()
	result, err := converter.Convert(context.Background(), Options{
		Source: dir,
		DryRun: true,
	})
	assert.Nil(t, result)
	assert.ErrorIs(t, err, ErrNoCookiecutterConfig)
}

func TestUT_Convert_FullConversion(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	// Set up a minimal Cookiecutter template
	srcDir := t.TempDir()
	cookiecutterJSON := `{
		"project_name": "my-project",
		"description": "A test project",
		"version": "1.0.0"
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(cookiecutterJSON),
		0o644,
	))

	// Create a template file with cookiecutter references
	projDir := filepath.Join(srcDir, "{{ cookiecutter.project_name }}")
	require.NoError(t, os.MkdirAll(projDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(projDir, "README.md"),
		[]byte("# {{ cookiecutter.project_name }}\n{{ cookiecutter.description }}"),
		0o644,
	))

	destDir := filepath.Join(t.TempDir(), "out")
	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, srcDir, result.Source)
	assert.Equal(t, destDir, result.Destination)
	assert.Greater(t, result.VariablesConverted, 0)
	assert.Greater(t, result.FilesProcessed, 0)

	// Verify tag.template.json was created
	assert.FileExists(t, filepath.Join(destDir, types.TemplateConfigFile))
}

func TestUT_Convert_DryRun_DoesNotWriteFiles(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test"}`),
		0o644,
	))

	destDir := filepath.Join(t.TempDir(), "dry-out")
	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
		DryRun:      true,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.DryRun)

	// Destination should not exist
	_, statErr := os.Stat(destDir)
	assert.True(t, os.IsNotExist(statErr), "dry run should not create output directory")
}

func TestUT_Convert_OutputExists_NoForce(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test"}`),
		0o644,
	))

	destDir := t.TempDir() // Already exists

	_, err = converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
		Force:       false,
	})
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrOutputExists)
}

func TestUT_Convert_OutputExists_WithForce(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test"}`),
		0o644,
	))

	destDir := filepath.Join(t.TempDir(), "force-out")
	require.NoError(t, os.MkdirAll(destDir, 0o755))

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
		Force:       true,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestUT_Convert_DefaultDestination(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	// Create source dir named "cookiecutter-myproject"
	parentDir := t.TempDir()
	srcDir := filepath.Join(parentDir, "cookiecutter-myproject")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test"}`),
		0o644,
	))

	// Use dry run to test default destination logic without writing files
	result, err := converter.Convert(context.Background(), Options{
		Source: srcDir,
		DryRun: true,
		// No destination — should default to "myproject-tag"
	})
	require.NoError(t, err)
	assert.Equal(t, "myproject-tag", result.Destination)
}

func TestUT_NewConverter_Success(t *testing.T) {
	t.Parallel()
	c, err := NewConverter()
	require.NoError(t, err)
	assert.NotNil(t, c)
	assert.NotNil(t, c.resolver)
	assert.NotNil(t, c.analyzer)
}

func TestUT_ResolveSource_LocalDir(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	dir := t.TempDir()
	result, err := converter.resolveSource(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, dir, result)
}

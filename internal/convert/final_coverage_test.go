package convert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

// ===========================================================================
// cookiecutter.go — coverage for processTemplateFiles edge cases,
// Convert safety checks, resolveSource
// ===========================================================================

func TestUT_Convert_UnsafeOutputDir_Root(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	// Create a cookiecutter.json so it passes the "is cookiecutter" check
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test"}`),
		0o644,
	))

	converter, err := NewConverter()
	require.NoError(t, err)

	_, err = converter.Convert(nil, Options{ //nolint:staticcheck // nil context acceptable for test
		Source:      srcDir,
		Destination: "/",
		Force:       true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe output directory")
}

func TestUT_Convert_OutputExistsForce(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create cookiecutter.json
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test"}`),
		0o644,
	))

	// Create a template file
	projDir := filepath.Join(srcDir, "{{ cookiecutter.project_name }}")
	require.NoError(t, os.MkdirAll(projDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(projDir, "main.go"),
		[]byte("package {{ cookiecutter.project_name }}"),
		0o644,
	))

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(nil, Options{ //nolint:staticcheck // nil context acceptable for test
		Source:      srcDir,
		Destination: destDir,
		Force:       true,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, destDir, result.Destination)
	assert.Greater(t, result.FilesProcessed, 0)
}

func TestUT_Convert_DryRunSkipsAll(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test", "version": "1.0"}`),
		0o644,
	))

	projDir := filepath.Join(srcDir, "{{ cookiecutter.project_name }}")
	require.NoError(t, os.MkdirAll(projDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(projDir, "README.md"),
		[]byte("# {{ cookiecutter.project_name }}"),
		0o644,
	))

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(nil, Options{ //nolint:staticcheck // nil context acceptable for test
		Source:      srcDir,
		Destination: filepath.Join(t.TempDir(), "output"),
		DryRun:      true,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.DryRun)
	assert.Greater(t, result.VariablesConverted, 0)
}

func TestUT_Convert_SkipsGitDirectory(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test"}`),
		0o644,
	))

	// Create a .git directory that should be skipped
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, ".git", "objects"), 0o755))

	// Create a normal file
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, "README.md"),
		[]byte("# readme"),
		0o644,
	))

	destDir := filepath.Join(t.TempDir(), "output")
	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(nil, Options{ //nolint:staticcheck // nil context acceptable for test
		Source:      srcDir,
		Destination: destDir,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)

	// .git should not be copied
	_, err = os.Stat(filepath.Join(destDir, ".git"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Convert_DefaultDestination_CookiecutterPrefix(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	// Simulate a directory named cookiecutter-something
	ccDir := filepath.Join(srcDir, "cookiecutter-myapp")
	require.NoError(t, os.MkdirAll(ccDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(ccDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test"}`),
		0o644,
	))

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(nil, Options{ //nolint:staticcheck // nil context acceptable for test
		Source:      ccDir,
		Destination: "", // Should generate default
		DryRun:      true,
	})
	require.NoError(t, err)
	assert.Equal(t, "myapp-tag", result.Destination)
}

func TestUT_Convert_WithHooksDir(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test"}`),
		0o644,
	))

	// Create hooks directory
	hooksDir := filepath.Join(srcDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "pre_gen_project.sh"),
		[]byte("#!/bin/bash\necho hello"),
		0o755,
	))

	destDir := filepath.Join(t.TempDir(), "output")
	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(nil, Options{ //nolint:staticcheck // nil context acceptable for test
		Source:      srcDir,
		Destination: destDir,
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestUT_Convert_NoCookiecutter(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	// No cookiecutter.json present

	converter, err := NewConverter()
	require.NoError(t, err)

	_, err = converter.Convert(nil, Options{ //nolint:staticcheck // nil context acceptable for test
		Source: srcDir,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoCookiecutterConfig)
}

func TestUT_Convert_OutputExistsNoForce(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	destDir := t.TempDir()

	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test"}`),
		0o644,
	))

	converter, err := NewConverter()
	require.NoError(t, err)

	_, err = converter.Convert(nil, Options{ //nolint:staticcheck // nil context acceptable for test
		Source:      srcDir,
		Destination: destDir,
		Force:       false,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutputExists)
}

func TestUT_Convert_BinaryFileCopied(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"project_name": "test"}`),
		0o644,
	))

	// Create a binary file (contains null bytes)
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00, 0x00, 0x00}
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "image.png"), binaryContent, 0o644))

	destDir := filepath.Join(t.TempDir(), "output")
	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(nil, Options{ //nolint:staticcheck // nil context acceptable for test
		Source:      srcDir,
		Destination: destDir,
	})
	require.NoError(t, err)
	assert.Greater(t, result.FilesProcessed, 0)

	// Binary file should be copied
	copied, err := os.ReadFile(filepath.Join(destDir, "image.png"))
	require.NoError(t, err)
	assert.Equal(t, binaryContent, copied)
}

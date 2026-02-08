package integration

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/convert"
	"github.com/kaikenlabs/tag/internal/scaffold"
)

// getTestdataDir returns the absolute path to the testdata directory.
func getTestdataDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to get caller information")
	}
	return filepath.Join(filepath.Dir(filename), "testdata")
}

// compareDirectories walks the expected directory tree and asserts that every file
// in expected also exists in actual with identical content.
// tag.template.json files are compared as parsed JSON (order-insensitive).
// All other files are compared byte-for-byte.
// Extra files in actual that are not in expected are allowed (e.g. .tagconfig.json).
func compareDirectories(t *testing.T, expectedDir, actualDir string) {
	t.Helper()

	err := filepath.WalkDir(expectedDir, func(path string, d os.DirEntry, err error) error {
		require.NoError(t, err, "walking expected directory")

		relPath, err := filepath.Rel(expectedDir, path)
		require.NoError(t, err)

		if relPath == "." {
			return nil
		}

		actualPath := filepath.Join(actualDir, relPath)

		if d.IsDir() {
			assert.DirExists(t, actualPath, "expected directory missing: %s", relPath)
			return nil
		}

		// Compare files
		require.FileExists(t, actualPath, "expected file missing: %s", relPath)

		expectedContent, err := os.ReadFile(path)
		require.NoError(t, err, "reading expected file: %s", relPath)

		actualContent, err := os.ReadFile(actualPath)
		require.NoError(t, err, "reading actual file: %s", relPath)

		if filepath.Base(relPath) == "tag.template.json" {
			// JSON comparison: order-insensitive
			compareJSON(t, relPath, expectedContent, actualContent)
		} else {
			// Byte-for-byte comparison
			assert.Equal(t, string(expectedContent), string(actualContent),
				"file content mismatch: %s", relPath)
		}

		return nil
	})
	require.NoError(t, err)
}

// compareJSON compares two JSON byte slices in an order-insensitive manner.
func compareJSON(t *testing.T, relPath string, expected, actual []byte) {
	t.Helper()
	assert.JSONEq(t, string(expected), string(actual),
		"JSON content mismatch: %s", relPath)
}

// TestIT_ConvertCookiecutter tests the Cookiecutter-to-TAG conversion pipeline.
// It converts a Cookiecutter template exercising all Tier 1 features and
// compares the output against committed golden files.
func TestIT_ConvertCookiecutter(t *testing.T) {
	testdataDir := getTestdataDir()
	srcDir := filepath.Join(testdataDir, "cookiecutter-fullstack")
	expectedDir := filepath.Join(testdataDir, "expected-convert")

	// Verify testdata exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		t.Skipf("testdata not found at %s", srcDir)
	}

	destDir := filepath.Join(t.TempDir(), "converted")

	// Run conversion
	converter, err := convert.NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), convert.Options{
		Source:      srcDir,
		Destination: destDir,
	})
	require.NoError(t, err)

	// Verify conversion stats
	assert.Equal(t, 8, result.VariablesConverted, "should convert 8 variables")
	assert.Equal(t, 2, result.DirsRenamed, "should rename 2 directories")
	assert.Equal(t, 5, result.FilesRenamed, "should rename 5 files")
	assert.Equal(t, 5, result.FilesProcessed, "should process 5 files")
	assert.Equal(t, 1, result.HooksCopied, "should copy 1 hook")
	assert.False(t, result.DryRun)

	// Compare against golden files
	compareDirectories(t, expectedDir, destDir)
}

// TestIT_ScaffoldCookiecutter tests the full Cookiecutter→TAG→scaffold pipeline.
// It converts a Cookiecutter template, then scaffolds a project with known inputs,
// and compares the rendered output against committed golden files.
func TestIT_ScaffoldCookiecutter(t *testing.T) {
	testdataDir := getTestdataDir()
	srcDir := filepath.Join(testdataDir, "cookiecutter-fullstack")
	expectedDir := filepath.Join(testdataDir, "expected-scaffold")

	// Verify testdata exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		t.Skipf("testdata not found at %s", srcDir)
	}

	tmpDir := t.TempDir()
	convertedDir := filepath.Join(tmpDir, "converted")
	scaffoldDir := filepath.Join(tmpDir, "scaffolded")

	// Step 1: Convert Cookiecutter → TAG
	converter, err := convert.NewConverter()
	require.NoError(t, err)

	_, err = converter.Convert(context.Background(), convert.Options{
		Source:      srcDir,
		Destination: convertedDir,
	})
	require.NoError(t, err)

	// Step 2: Scaffold with known inputs
	opts := scaffold.Options{
		TemplateDir: convertedDir,
		OutputDir:   scaffoldDir,
		Meta: map[string]string{
			"project_name": "My Project",
			"author":       "Test Author",
			"description":  "A short description",
			"license":      "MIT",
			"use_docker":   "true",
			"port":         "8080",
		},
		NoInput: true,
		NoSave:  true,
	}

	s, err := scaffold.NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Compare against golden files (walk expected, check in actual)
	compareDirectories(t, expectedDir, scaffoldDir)
}

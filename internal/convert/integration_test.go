package convert

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestdataDir returns the path to the testdata directory.
func getTestdataDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to get caller information")
	}
	return filepath.Join(filepath.Dir(filename), "testdata")
}

func TestIT_ConvertCookiecutter_FullTemplate(t *testing.T) {
	testdataDir := getTestdataDir()
	srcDir := filepath.Join(testdataDir, "cookiecutter-sample")
	destDir := filepath.Join(t.TempDir(), "converted-template")

	// Verify testdata exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		t.Skipf("testdata not found at %s", srcDir)
	}

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})

	require.NoError(t, err)

	// Verify basic stats
	assert.Greater(t, result.VariablesConverted, 0, "should convert some variables")
	assert.Greater(t, result.DirsRenamed, 0, "should rename some directories")
	assert.Greater(t, result.FilesProcessed, 0, "should process some files")
	assert.Greater(t, result.HooksCopied, 0, "should copy hooks")

	// Verify tag.template.json was created
	tagConfigPath := filepath.Join(destDir, "tag.template.json")
	tagConfigData, err := os.ReadFile(tagConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(tagConfigData), "project_name")
	assert.Contains(t, string(tagConfigData), "use_docker")
	assert.Contains(t, string(tagConfigData), "open_source_license")

	// Verify directory was converted ({{ cookiecutter.project_slug }} -> {{ vars.project_slug }})
	convertedProjectDir := filepath.Join(destDir, "{{ vars.project_slug }}")
	_, err = os.Stat(convertedProjectDir)
	require.NoError(t, err, "converted directory should exist")

	// Verify files exist in converted directory
	_, err = os.Stat(filepath.Join(convertedProjectDir, "README.md"))
	require.NoError(t, err, "README.md should exist")

	_, err = os.Stat(filepath.Join(convertedProjectDir, "src", "main.py"))
	require.NoError(t, err, "src/main.py should exist")

	// Verify hooks were copied
	_, err = os.Stat(filepath.Join(destDir, "hooks", "pre_gen_project.py"))
	require.NoError(t, err, "pre_gen_project.py should be copied")

	_, err = os.Stat(filepath.Join(destDir, "hooks", "post_gen_project.sh"))
	require.NoError(t, err, "post_gen_project.sh should be copied")

	// Verify incompatibilities were detected
	assert.Greater(t, len(result.Incompatibilities), 0, "should detect incompatibilities")

	// Check for specific incompatibility types
	hasFilterSyntax := false
	hasDictIteration := false
	for _, inc := range result.Incompatibilities {
		if inc.Kind == "filter-syntax" {
			hasFilterSyntax = true
		}
		if inc.Kind == "dict-iteration" {
			hasDictIteration = true
		}
	}
	assert.True(t, hasFilterSyntax, "should detect filter-syntax incompatibility")
	assert.True(t, hasDictIteration, "should detect dict-iteration incompatibility")

	// Verify warnings about hooks
	hasHookWarning := false
	for _, w := range result.Warnings {
		if len(w) > 0 {
			hasHookWarning = true
			break
		}
	}
	assert.True(t, hasHookWarning, "should have warnings about hooks")
}

func TestIT_ConvertCookiecutter_DryRunPreservesSource(t *testing.T) {
	testdataDir := getTestdataDir()
	srcDir := filepath.Join(testdataDir, "cookiecutter-sample")
	destDir := filepath.Join(t.TempDir(), "should-not-exist")

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		t.Skipf("testdata not found at %s", srcDir)
	}

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
		DryRun:      true,
	})

	require.NoError(t, err)
	assert.True(t, result.DryRun)
	assert.Greater(t, result.VariablesConverted, 0)

	// Verify destination was NOT created
	_, err = os.Stat(destDir)
	assert.True(t, os.IsNotExist(err), "destination should not exist in dry-run mode")
}

func TestIT_ConvertCookiecutter_VariableTypes(t *testing.T) {
	testdataDir := getTestdataDir()
	srcDir := filepath.Join(testdataDir, "cookiecutter-sample")
	destDir := filepath.Join(t.TempDir(), "converted")

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		t.Skipf("testdata not found at %s", srcDir)
	}

	converter, err := NewConverter()
	require.NoError(t, err)

	_, err = converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})

	require.NoError(t, err)

	// Read generated config
	tagConfigPath := filepath.Join(destDir, "tag.template.json")
	data, err := os.ReadFile(tagConfigPath)
	require.NoError(t, err)

	content := string(data)

	// Verify different variable types are handled
	assert.Contains(t, content, "project_name") // string
	assert.Contains(t, content, "use_docker")   // boolean
	assert.Contains(t, content, "boolean")      // type indicator for boolean
	assert.Contains(t, content, "options")      // choice arrays
	assert.Contains(t, content, "MIT")          // choice option
}

func TestIT_ConvertCookiecutter_PathConversion(t *testing.T) {
	testdataDir := getTestdataDir()
	srcDir := filepath.Join(testdataDir, "cookiecutter-sample")
	destDir := filepath.Join(t.TempDir(), "converted")

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		t.Skipf("testdata not found at %s", srcDir)
	}

	converter, err := NewConverter()
	require.NoError(t, err)

	_, err = converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})

	require.NoError(t, err)

	// The original directory {{ cookiecutter.project_slug }} should be converted to {{ vars.project_slug }}
	entries, err := os.ReadDir(destDir)
	require.NoError(t, err)

	hasConvertedDir := false
	for _, entry := range entries {
		if entry.Name() == "{{ vars.project_slug }}" && entry.IsDir() {
			hasConvertedDir = true
			break
		}
	}
	assert.True(t, hasConvertedDir, "should have {{ vars.project_slug }} directory")

	// Verify original cookiecutter path does NOT exist
	_, err = os.Stat(filepath.Join(destDir, "{{ cookiecutter.project_slug }}"))
	assert.True(t, os.IsNotExist(err), "original cookiecutter path should not exist")
}

func TestIT_ConvertCookiecutter_ContentPreserved(t *testing.T) {
	testdataDir := getTestdataDir()
	srcDir := filepath.Join(testdataDir, "cookiecutter-sample")
	destDir := filepath.Join(t.TempDir(), "converted")

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		t.Skipf("testdata not found at %s", srcDir)
	}

	converter, err := NewConverter()
	require.NoError(t, err)

	_, err = converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})
	require.NoError(t, err)

	// Read converted file and verify content is preserved
	readmePath := filepath.Join(destDir, "{{ vars.project_slug }}", "README.md")
	content, err := os.ReadFile(readmePath)
	require.NoError(t, err)

	// Content should be converted from cookiecutter.* to vars.* syntax
	assert.Contains(t, string(content), "{{ vars.project_name }}")
	assert.Contains(t, string(content), "{{ vars.description }}")
	assert.Contains(t, string(content), "{% if vars.open_source_license")
}

func TestIT_ConvertCookiecutter_HooksConfig(t *testing.T) {
	testdataDir := getTestdataDir()
	srcDir := filepath.Join(testdataDir, "cookiecutter-sample")
	destDir := filepath.Join(t.TempDir(), "converted")

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		t.Skipf("testdata not found at %s", srcDir)
	}

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})
	require.NoError(t, err)

	// Should have hook warnings (Python hooks not directly supported)
	hasPythonHookWarning := false
	for _, w := range result.Warnings {
		if len(w) > 0 && (contains(w, "Python") || contains(w, "python")) {
			hasPythonHookWarning = true
			break
		}
	}
	assert.True(t, hasPythonHookWarning, "should warn about Python hooks")

	// Read tag.template.json and verify shell hook is suggested
	tagConfigPath := filepath.Join(destDir, "tag.template.json")
	data, err := os.ReadFile(tagConfigPath)
	require.NoError(t, err)

	// Shell hook should be in the config
	assert.Contains(t, string(data), "post_scaffold")
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

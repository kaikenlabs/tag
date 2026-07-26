package engine

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/history"
	"github.com/kaikenlabs/tag/internal/template"
)

// setupGeneratorDir creates a temp directory with a single generator template
// and returns the directory path.
func setupGeneratorDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	tmplContent := "---\nto: output/{{ name }}.go\n---\npackage {{ name }}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mytemplate.go"), []byte(tmplContent), 0o644))
	return dir
}

// chdirToTempDir changes the working directory to a new temp dir and returns
// a cleanup function. The generator must be created AFTER this call so the
// writer captures the correct cwd for path safety checks.
func chdirToTempDir(t *testing.T) string {
	t.Helper()
	workDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	return workDir
}

func TestUT_NewGenerator_Success(t *testing.T) {
	dirPath := setupGeneratorDir(t)
	workDir := chdirToTempDir(t)

	gen, err := NewGenerator(false, dirPath, "", io.Discard)
	require.NoError(t, err)
	require.NotNil(t, gen)

	result, err := gen.Generate(Data{Name: "hello", RawMeta: []string{"name=hello"}})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)

	outputFile := filepath.Join(workDir, "output", "hello.go")
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "package hello")
}

func TestUT_NewGenerator_DryRun(t *testing.T) {
	dirPath := setupGeneratorDir(t)
	workDir := chdirToTempDir(t)

	var buf bytes.Buffer
	gen, err := NewGenerator(true, dirPath, "", &buf)
	require.NoError(t, err)
	require.NotNil(t, gen)

	_, err = gen.Generate(Data{Name: "hello", RawMeta: []string{"name=hello"}})
	require.NoError(t, err)

	// In dry-run mode, no files should be written to disk.
	outputFile := filepath.Join(workDir, "output", "hello.go")
	_, statErr := os.Stat(outputFile)
	assert.True(t, os.IsNotExist(statErr), "dry-run should not create files on disk")
}

func TestUT_NewGeneratorWithEngine_Success(t *testing.T) {
	dirPath := setupGeneratorDir(t)
	_ = chdirToTempDir(t)

	eng, err := template.NewEngine()
	require.NoError(t, err)

	gen, err := NewGeneratorWithEngine(eng, false, dirPath, "", io.Discard)
	require.NoError(t, err)
	require.NotNil(t, gen)

	result, err := gen.Generate(Data{Name: "test", RawMeta: []string{"name=test"}})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
}

func TestUT_NewGeneratorWithRecorder_NilRecorder(t *testing.T) {
	dirPath := setupGeneratorDir(t)
	_ = chdirToTempDir(t)

	eng, err := template.NewEngine()
	require.NoError(t, err)

	gen, err := NewGeneratorWithRecorder(eng, false, dirPath, "", nil, io.Discard)
	require.NoError(t, err)
	require.NotNil(t, gen)

	result, err := gen.Generate(Data{Name: "test", RawMeta: []string{"name=test"}})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
}

func TestUT_NewGeneratorWithRecorder_WithRecorder(t *testing.T) {
	dirPath := setupGeneratorDir(t)
	_ = chdirToTempDir(t)

	eng, err := template.NewEngine()
	require.NoError(t, err)

	tagDir := t.TempDir()
	rec := history.NewRecorder(tagDir)

	gen, err := NewGeneratorWithRecorder(eng, false, dirPath, "", rec, io.Discard)
	require.NoError(t, err)
	require.NotNil(t, gen)

	result, err := gen.Generate(Data{Name: "test", RawMeta: []string{"name=test"}})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
}

// TestUT_NewGeneratorWithRecorder_DryRun reproduces B1: a dry-run generate with a
// recorder must not try to hash files it deliberately never wrote. Before the fix the
// RecordingFileWriter wrapped the no-op dry-run writer and failed on "hash after write".
func TestUT_NewGeneratorWithRecorder_DryRun(t *testing.T) {
	dirPath := setupGeneratorDir(t)
	workDir := chdirToTempDir(t)

	eng, err := template.NewEngine()
	require.NoError(t, err)

	rec := history.NewRecorder(t.TempDir())

	var buf bytes.Buffer
	gen, err := NewGeneratorWithRecorder(eng, true, dirPath, "", rec, &buf)
	require.NoError(t, err)
	require.NotNil(t, gen)

	_, err = gen.Generate(Data{Name: "hello", RawMeta: []string{"name=hello"}})
	require.NoError(t, err, "dry-run with recorder must not error on hash-after-write")

	// Dry-run must not write files to disk.
	_, statErr := os.Stat(filepath.Join(workDir, "output", "hello.go"))
	assert.True(t, os.IsNotExist(statErr), "dry-run should not create files on disk")
}

func TestUT_DryRunWriterOpts_NotDryRun(t *testing.T) {
	t.Parallel()

	opts := dryRunWriterOpts(false, nil)
	assert.Nil(t, opts)
}

func TestUT_DryRunWriterOpts_DryRun(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	opts := dryRunWriterOpts(true, &buf)
	assert.NotNil(t, opts)
	assert.Len(t, opts, 1)
}

func TestUT_NewGenerator_WithSharedTemplates(t *testing.T) {
	// Create main template directory with a template that uses include.
	mainDir := t.TempDir()
	sharedDir := t.TempDir()

	// Create shared template.
	sharedContent := "// SHARED HEADER"
	require.NoError(t, os.WriteFile(filepath.Join(sharedDir, "header.tmpl"), []byte(sharedContent), 0o644))

	// Create main template that includes the shared template.
	mainTmpl := "---\nto: output/{{ name }}.go\n---\n{% include \"header.tmpl\" %}\npackage {{ name }}\n"
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "main.go"), []byte(mainTmpl), 0o644))

	workDir := chdirToTempDir(t)

	gen, err := NewGenerator(false, mainDir, sharedDir, io.Discard)
	require.NoError(t, err)
	require.NotNil(t, gen)

	result, err := gen.Generate(Data{Name: "test", RawMeta: []string{"name=test"}})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)

	content, err := os.ReadFile(filepath.Join(workDir, "output", "test.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "// SHARED HEADER")
	assert.Contains(t, string(content), "package test")
}

func TestUT_NewGenerator_InvalidDir(t *testing.T) {
	t.Parallel()

	_, err := NewGenerator(false, "/nonexistent/path/to/templates", "", io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot load templates")
}

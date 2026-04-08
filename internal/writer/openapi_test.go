package writer

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_MergeOpenAPIFile_InsertsNewPaths(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "openapi.yaml")

	spec := `openapi: "3.0.3"
info:
  title: Test API
paths:
  /existing:
    get:
      summary: Existing
components:
  schemas:
    Existing:
      type: object
`
	require.NoError(t, os.WriteFile(specPath, []byte(spec), 0o644))

	// Must run from the temp dir for path containment validation
	origDir, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(origDir) }()

	w := Write{
		mx:  sync.Mutex{},
		fs:  &realFS{},
		cwd: dir,
	}

	fragment := `paths:
  /widgets:
    get:
      summary: List widgets
components:
  schemas:
    Widget:
      type: object
`
	result, err := w.MergeOpenAPIFile(specPath, []byte(fragment), OpenAPIMergeOptions{})
	require.NoError(t, err)

	assert.True(t, result.Changed)
	assert.Equal(t, []string{"/widgets"}, result.AddedPaths)
	assert.Equal(t, []string{"Widget"}, result.AddedSchemas)

	// Verify file was written
	content, err := os.ReadFile(specPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "/widgets")
	assert.Contains(t, string(content), "Widget")
	assert.Contains(t, string(content), "/existing") // preserved
}

func TestUT_MergeOpenAPIFile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "openapi.yaml")

	spec := `openapi: "3.0.3"
paths:
  /widgets:
    get:
      summary: List widgets
`
	require.NoError(t, os.WriteFile(specPath, []byte(spec), 0o644))

	w := Write{
		mx:  sync.Mutex{},
		fs:  &realFS{},
		cwd: dir,
	}

	fragment := `paths:
  /widgets:
    get:
      summary: List widgets
`
	result, err := w.MergeOpenAPIFile(specPath, []byte(fragment), OpenAPIMergeOptions{})
	require.NoError(t, err)

	assert.False(t, result.Changed)
	assert.Equal(t, []string{"/widgets"}, result.SkippedPaths)
}

func TestUT_MergeOpenAPIFile_ConflictError(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "openapi.yaml")

	spec := `openapi: "3.0.3"
paths:
  /widgets:
    get:
      summary: Old summary
`
	require.NoError(t, os.WriteFile(specPath, []byte(spec), 0o644))

	w := Write{
		mx:  sync.Mutex{},
		fs:  &realFS{},
		cwd: dir,
	}

	fragment := `paths:
  /widgets:
    get:
      summary: New summary
`
	_, err := w.MergeOpenAPIFile(specPath, []byte(fragment), OpenAPIMergeOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestUT_MergeOpenAPIFile_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "nonexistent.yaml")

	w := Write{
		mx:  sync.Mutex{},
		fs:  &realFS{},
		cwd: dir,
	}

	_, err := w.MergeOpenAPIFile(specPath, []byte("paths: {}"), OpenAPIMergeOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read spec file")
}

func TestUT_MergeOpenAPIFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()

	w := Write{
		mx:  sync.Mutex{},
		fs:  &realFS{},
		cwd: dir,
	}

	_, err := w.MergeOpenAPIFile("/etc/passwd", []byte("paths: {}"), OpenAPIMergeOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path safety")
}

// realFS is a minimal real filesystem implementation for testing.
type realFS struct{}

func (r *realFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (r *realFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (r *realFS) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (r *realFS) Write(file *os.File, b []byte) (int, error) {
	return file.Write(b)
}

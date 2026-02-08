package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Loader_NewLoader(t *testing.T) {
	loader := NewLoader("/tmp/templates")
	assert.NotNil(t, loader)
	assert.Equal(t, "/tmp/templates", loader.BaseDir())
}

func TestUT_Loader_Load(t *testing.T) {
	// Create temporary directory and file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tmpl")
	expected := "Hello, {{ name }}!"
	err := os.WriteFile(tmpFile, []byte(expected), 0o644)
	require.NoError(t, err)

	loader := NewLoader(tmpDir)

	content, err := loader.Load("test.tmpl")
	require.NoError(t, err)
	assert.Equal(t, expected, content)
}

func TestUT_Loader_Load_NotFound(t *testing.T) {
	loader := NewLoader(t.TempDir())

	_, err := loader.Load("nonexistent.tmpl")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent.tmpl")
}

func TestUT_Loader_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "exists.tmpl")
	err := os.WriteFile(tmpFile, []byte("content"), 0o644)
	require.NoError(t, err)

	loader := NewLoader(tmpDir)

	assert.True(t, loader.Exists("exists.tmpl"))
	assert.False(t, loader.Exists("missing.tmpl"))
}

func TestUT_Loader_LoadAbsolutePath_Rejected(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tmpl")
	err := os.WriteFile(tmpFile, []byte("content"), 0o644)
	require.NoError(t, err)

	loader := NewLoader("/other/base")

	// Absolute paths should be rejected for security
	_, err = loader.Load(tmpFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "absolute paths not allowed")
}

func TestUT_GonjaLoader_Resolve(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tmpl")
	err := os.WriteFile(tmpFile, []byte("content"), 0o644)
	require.NoError(t, err)

	loader := NewLoader(tmpDir)
	gonjaLoader := NewGonjaLoader(loader)

	path, err := gonjaLoader.Resolve("test.tmpl")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "test.tmpl"), path)
}

func TestUT_GonjaLoader_Resolve_NotFound(t *testing.T) {
	loader := NewLoader(t.TempDir())
	gonjaLoader := NewGonjaLoader(loader)

	_, err := gonjaLoader.Resolve("missing.tmpl")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUT_GonjaLoader_Read(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tmpl")
	expected := "Hello, {{ name }}!"
	err := os.WriteFile(tmpFile, []byte(expected), 0o644)
	require.NoError(t, err)

	loader := NewLoader(tmpDir)
	gonjaLoader := NewGonjaLoader(loader)

	content, err := gonjaLoader.Read("test.tmpl")
	require.NoError(t, err)
	assert.Equal(t, expected, content)
}

func TestUT_CreateFileSystemLoader(t *testing.T) {
	tmpDir := t.TempDir()

	loader, err := CreateFileSystemLoader(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, loader)
}

func TestUT_CreateFileSystemLoader_NotExists(t *testing.T) {
	_, err := CreateFileSystemLoader("/nonexistent/directory")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestUT_LoadTemplateFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	err := os.WriteFile(filepath.Join(tmpDir, "a.tmpl"), []byte("template a"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "b.tmpl"), []byte("template b"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("not a template"), 0o644)
	require.NoError(t, err)

	templates, err := LoadTemplateFiles(tmpDir, "tmpl")
	require.NoError(t, err)

	assert.Len(t, templates, 2)
	assert.Equal(t, "template a", templates["a.tmpl"])
	assert.Equal(t, "template b", templates["b.tmpl"])
	assert.NotContains(t, templates, "c.txt")
}

func TestUT_LoadTemplateFiles_Subdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	err := os.Mkdir(subDir, 0o755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "root.tmpl"), []byte("root"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "nested.tmpl"), []byte("nested"), 0o644)
	require.NoError(t, err)

	templates, err := LoadTemplateFiles(tmpDir, "tmpl")
	require.NoError(t, err)

	assert.Len(t, templates, 2)
	assert.Equal(t, "root", templates["root.tmpl"])
	assert.Equal(t, "nested", templates[filepath.Join("sub", "nested.tmpl")])
}

func TestUT_LoadTemplateFiles_NotExists(t *testing.T) {
	_, err := LoadTemplateFiles("/nonexistent", "tmpl")
	assert.Error(t, err)
}

func TestUT_Loader_Integration_WithEngine(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a template file
	templateContent := `Hello, {{ name }}!`
	err := os.WriteFile(filepath.Join(tmpDir, "greeting.tmpl"), []byte(templateContent), 0o644)
	require.NoError(t, err)

	// Create engine with base dir
	engine := MustNewEngine(WithBaseDir(tmpDir))

	// Parse and execute
	tmpl, err := engine.ParseFile("greeting.tmpl")
	require.NoError(t, err)

	ctx := NewContext("World", nil, nil)
	result, err := tmpl.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", result)
}

func TestUT_LoadTemplateFiles_SkipsSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real template file
	err := os.WriteFile(filepath.Join(tmpDir, "real.tmpl"), []byte("real content"), 0o644)
	require.NoError(t, err)

	// Create a target file outside the template dir
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.tmpl")
	err = os.WriteFile(outsideFile, []byte("secret content"), 0o644)
	require.NoError(t, err)

	// Create a symlink to the outside file
	symlinkPath := filepath.Join(tmpDir, "linked.tmpl")
	err = os.Symlink(outsideFile, symlinkPath)
	require.NoError(t, err)

	templates, err := LoadTemplateFiles(tmpDir, "tmpl")
	require.NoError(t, err)

	// Should only contain the real file, not the symlinked one
	assert.Len(t, templates, 1)
	assert.Equal(t, "real content", templates["real.tmpl"])
	assert.NotContains(t, templates, "linked.tmpl")
}

func TestUT_LoadTemplateFiles_SkipsSymlinkDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real template file
	err := os.WriteFile(filepath.Join(tmpDir, "real.tmpl"), []byte("real content"), 0o644)
	require.NoError(t, err)

	// Create a directory outside with templates
	outsideDir := t.TempDir()
	err = os.WriteFile(filepath.Join(outsideDir, "secret.tmpl"), []byte("secret"), 0o644)
	require.NoError(t, err)

	// Create a symlink to the outside directory
	symlinkDir := filepath.Join(tmpDir, "linked-dir")
	err = os.Symlink(outsideDir, symlinkDir)
	require.NoError(t, err)

	templates, err := LoadTemplateFiles(tmpDir, "tmpl")
	require.NoError(t, err)

	// Should only contain the real file
	assert.Len(t, templates, 1)
	assert.Equal(t, "real content", templates["real.tmpl"])
}

func TestUT_Loader_PathTraversal_Rejected(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	// Test various path traversal attempts
	traversalPaths := []string{
		"../etc/passwd",
		"../../etc/passwd",
		"foo/../../../etc/passwd",
		"foo/bar/../../..",
	}

	for _, path := range traversalPaths {
		t.Run(path, func(t *testing.T) {
			_, err := loader.Load(path)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "path traversal not allowed")
		})
	}
}

func TestUT_Loader_PathTraversal_ValidRelativePaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory and file
	subDir := filepath.Join(tmpDir, "sub")
	err := os.Mkdir(subDir, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "test.tmpl"), []byte("content"), 0o644)
	require.NoError(t, err)

	loader := NewLoader(tmpDir)

	// These should all work
	validPaths := []string{
		"sub/test.tmpl",
		"./sub/test.tmpl",
	}

	for _, path := range validPaths {
		t.Run(path, func(t *testing.T) {
			content, err := loader.Load(path)
			require.NoError(t, err)
			assert.Equal(t, "content", content)
		})
	}
}

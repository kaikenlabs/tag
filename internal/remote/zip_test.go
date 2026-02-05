package remote

import (
	"archive/zip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestZip creates a zip file for testing.
func createTestZip(t *testing.T, destPath string, files map[string]string) {
	t.Helper()

	f, err := os.Create(destPath)
	require.NoError(t, err)
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// Track directories we've created
	createdDirs := make(map[string]bool)

	for name, content := range files {
		// Create parent directories first
		dir := filepath.Dir(name)
		if dir != "." && !createdDirs[dir] {
			// Create all parent directories
			parts := strings.Split(dir, "/")
			for i := range parts {
				parentDir := strings.Join(parts[:i+1], "/") + "/"
				if !createdDirs[parentDir] {
					_, err := w.Create(parentDir)
					require.NoError(t, err)
					createdDirs[parentDir] = true
				}
			}
		}

		fw, err := w.Create(name)
		require.NoError(t, err)
		_, err = fw.Write([]byte(content))
		require.NoError(t, err)
	}
}

// createTestZipWithRoot creates a zip file with content inside a root directory.
func createTestZipWithRoot(t *testing.T, destPath, rootDir string, files map[string]string) {
	t.Helper()

	prefixedFiles := make(map[string]string)
	for name, content := range files {
		prefixedFiles[rootDir+"/"+name] = content
	}
	createTestZip(t, destPath, prefixedFiles)
}

func TestUT_ZipFetcher_NewZipFetcher(t *testing.T) {
	fetcher := NewZipFetcher()
	assert.NotNil(t, fetcher)
	assert.NotNil(t, fetcher.client)
	assert.Equal(t, int64(500*1024*1024), fetcher.maxFileSize)
}

func TestUT_ZipFetcher_FetchWrongType(t *testing.T) {
	fetcher := NewZipFetcher()

	ref := &Reference{
		Type: ReferenceTypeGit,
		URL:  "https://github.com/user/repo.git",
	}

	_, err := fetcher.Fetch(context.Background(), ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a Zip reference")
}

func TestUT_ZipFetcher_LocalZip(t *testing.T) {
	fetcher := NewZipFetcher()

	// Create test zip
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"file.txt":          "content",
		"subdir/nested.txt": "nested content",
		"tag.template.json": `{"name": "test"}`,
	})

	ref := &Reference{
		Original: zipPath,
		Type:     ReferenceTypeZip,
		Provider: ProviderGeneric,
		URL:      zipPath,
	}

	path, err := fetcher.Fetch(context.Background(), ref)
	require.NoError(t, err)
	defer os.RemoveAll(path)

	// Verify extracted content
	content, err := os.ReadFile(filepath.Join(path, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))

	nested, err := os.ReadFile(filepath.Join(path, "subdir", "nested.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested content", string(nested))
}

func TestUT_ZipFetcher_UnwrapSingleRoot(t *testing.T) {
	fetcher := NewZipFetcher()

	// Create test zip with root directory
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZipWithRoot(t, zipPath, "my-template-main", map[string]string{
		"file.txt":          "content",
		"tag.template.json": `{"name": "test"}`,
	})

	ref := &Reference{
		Original: zipPath,
		Type:     ReferenceTypeZip,
		Provider: ProviderGeneric,
		URL:      zipPath,
	}

	path, err := fetcher.Fetch(context.Background(), ref)
	require.NoError(t, err)
	defer os.RemoveAll(filepath.Dir(path)) // Clean up parent temp dir

	// Path should point to the unwrapped content
	content, err := os.ReadFile(filepath.Join(path, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))

	// Verify we're in the unwrapped directory (my-template-main)
	assert.True(t, filepath.Base(path) == "my-template-main")
}

func TestUT_ZipFetcher_NoUnwrapMultipleRoots(t *testing.T) {
	fetcher := NewZipFetcher()

	// Create test zip with multiple root items
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
	})

	ref := &Reference{
		Original: zipPath,
		Type:     ReferenceTypeZip,
		Provider: ProviderGeneric,
		URL:      zipPath,
	}

	path, err := fetcher.Fetch(context.Background(), ref)
	require.NoError(t, err)
	defer os.RemoveAll(path)

	// Both files should be at root
	_, err = os.Stat(filepath.Join(path, "file1.txt"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(path, "file2.txt"))
	assert.NoError(t, err)
}

func TestUT_ZipFetcher_SubPath(t *testing.T) {
	fetcher := NewZipFetcher()

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	// Use a root directory like GitHub archives have (repo-main/)
	// After unwrap, content will be at the root, and we can select subpath
	createTestZipWithRoot(t, zipPath, "repo-main", map[string]string{
		"templates/go/main.go.tmpl":      "package main",
		"templates/go/tag.template.json": `{"name": "go"}`,
		"templates/python/main.py.tmpl":  "print('hello')",
		"README.md":                      "# Readme",
	})

	ref := &Reference{
		Original: zipPath,
		Type:     ReferenceTypeZip,
		Provider: ProviderGeneric,
		URL:      zipPath,
		SubPath:  "templates/go",
	}

	path, err := fetcher.Fetch(context.Background(), ref)
	require.NoError(t, err)
	defer os.RemoveAll(filepath.Dir(filepath.Dir(filepath.Dir(path)))) // Clean up root temp dir

	// Should be in the subpath
	content, err := os.ReadFile(filepath.Join(path, "main.go.tmpl"))
	require.NoError(t, err)
	assert.Equal(t, "package main", string(content))
}

func TestUT_ZipFetcher_InvalidSubPath(t *testing.T) {
	fetcher := NewZipFetcher()

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"file.txt": "content",
	})

	ref := &Reference{
		Original: zipPath,
		Type:     ReferenceTypeZip,
		Provider: ProviderGeneric,
		URL:      zipPath,
		SubPath:  "nonexistent",
	}

	_, err := fetcher.Fetch(context.Background(), ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subpath")
	assert.Contains(t, err.Error(), "not found")
}

func TestUT_ZipFetcher_PreventZipSlip(t *testing.T) {
	fetcher := NewZipFetcher()

	// Create a zip with path traversal attempt
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "malicious.zip")

	// Manually create zip with malicious path
	f, err := os.Create(zipPath)
	require.NoError(t, err)

	w := zip.NewWriter(f)
	// Try to write outside the extraction directory
	fw, err := w.Create("../../../etc/passwd")
	require.NoError(t, err)
	_, err = fw.Write([]byte("malicious content"))
	require.NoError(t, err)
	w.Close()
	f.Close()

	ref := &Reference{
		Original: zipPath,
		Type:     ReferenceTypeZip,
		Provider: ProviderGeneric,
		URL:      zipPath,
	}

	_, err = fetcher.Fetch(context.Background(), ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestUT_ZipFetcher_SanitizePath(t *testing.T) {
	fetcher := NewZipFetcher()
	destDir := "/tmp/extract"

	tests := []struct {
		name      string
		filePath  string
		expectErr bool
	}{
		{"valid path", "file.txt", false},
		{"valid nested", "dir/file.txt", false},
		{"path traversal", "../etc/passwd", true},
		{"hidden traversal", "foo/../../../etc/passwd", true},
		{"absolute path", "/etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fetcher.sanitizePath(destDir, tt.filePath)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUT_ZipFetcher_SkipsHiddenAndMacOS(t *testing.T) {
	fetcher := NewZipFetcher()

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"template/file.txt": "content",
		"__MACOSX/._file":   "mac metadata",
		".hidden":           "hidden file",
	})

	ref := &Reference{
		Original: zipPath,
		Type:     ReferenceTypeZip,
		Provider: ProviderGeneric,
		URL:      zipPath,
	}

	path, err := fetcher.Fetch(context.Background(), ref)
	require.NoError(t, err)
	defer os.RemoveAll(filepath.Dir(path))

	// Should unwrap to template/ since __MACOSX and .hidden are skipped
	assert.Equal(t, "template", filepath.Base(path))
}

func TestIT_ZipFetcher_RemoteDownload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create test zip
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"file.txt":          "remote content",
		"tag.template.json": `{"name": "remote-test"}`,
	})

	// Read zip content
	zipContent, err := os.ReadFile(zipPath)
	require.NoError(t, err)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(zipContent)
	}))
	defer server.Close()

	fetcher := NewZipFetcher()

	ref := &Reference{
		Original: server.URL + "/template.zip",
		Type:     ReferenceTypeZip,
		Provider: ProviderGeneric,
		URL:      server.URL + "/template.zip",
	}

	path, err := fetcher.Fetch(context.Background(), ref)
	require.NoError(t, err)
	defer os.RemoveAll(path)

	// Verify downloaded content
	content, err := os.ReadFile(filepath.Join(path, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "remote content", string(content))
}

func TestIT_ZipFetcher_RemoteDownloadError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	fetcher := NewZipFetcher()

	ref := &Reference{
		Original: server.URL + "/notfound.zip",
		Type:     ReferenceTypeZip,
		Provider: ProviderGeneric,
		URL:      server.URL + "/notfound.zip",
	}

	_, err := fetcher.Fetch(context.Background(), ref)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "download failed")
}

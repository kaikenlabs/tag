package update

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_New_CreatesUpdater(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	u := New("https://example.com", &buf)

	assert.NotNil(t, u)
	assert.NotNil(t, u.client)
	assert.Equal(t, "https://example.com", u.repoURL)
	assert.Equal(t, "tag", u.binaryName)
}

func TestUT_FileSHA256(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))

	hash, err := fileSHA256(path)
	require.NoError(t, err)
	// SHA256 of "hello"
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", hash)
}

func TestUT_FileSHA256_NotFound(t *testing.T) {
	t.Parallel()
	_, err := fileSHA256(filepath.Join(t.TempDir(), "nonexistent"))
	assert.Error(t, err)
}

func TestUT_FindChecksum_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums.txt")
	content := "abc123  file1.tar.gz\ndef456  file2.tar.gz\nghi789  file3.tar.gz\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	hash, err := findChecksum(path, "file2.tar.gz")
	require.NoError(t, err)
	assert.Equal(t, "def456", hash)
}

func TestUT_FindChecksum_FileNotInChecksums(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums.txt")
	require.NoError(t, os.WriteFile(path, []byte("abc123  other.tar.gz\n"), 0o644))

	_, err := findChecksum(path, "missing.tar.gz")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum not found")
}

func TestUT_CopyFile_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")

	require.NoError(t, os.WriteFile(src, []byte("binary content"), 0o755))
	require.NoError(t, copyFile(src, dst))

	content, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "binary content", string(content))

	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func TestUT_CopyFile_SrcNotFound(t *testing.T) {
	t.Parallel()
	err := copyFile(filepath.Join(t.TempDir(), "nonexistent"), filepath.Join(t.TempDir(), "dst"))
	assert.Error(t, err)
}

func TestUT_DownloadFile_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("downloaded content"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "downloaded.bin")

	var buf bytes.Buffer
	u := New(srv.URL, &buf)
	err := u.downloadFile(context.Background(), srv.URL+"/file", dest)
	require.NoError(t, err)

	content, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "downloaded content", string(content))
}

func TestUT_DownloadFile_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	u := New(srv.URL, &buf)
	err := u.downloadFile(context.Background(), srv.URL+"/file", filepath.Join(t.TempDir(), "out"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status 500")
}

func TestUT_UpdateError_Error(t *testing.T) {
	t.Parallel()
	err := &UpdateError{Op: "download", Err: assert.AnError}
	assert.Contains(t, err.Error(), "update download")
}

func TestUT_UpdateError_Unwrap(t *testing.T) {
	t.Parallel()
	inner := assert.AnError
	err := &UpdateError{Op: "verify", Err: inner}
	assert.Equal(t, inner, err.Unwrap())
}

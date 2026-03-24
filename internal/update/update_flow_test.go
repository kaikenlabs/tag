package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestUT_VerifyChecksum_FileNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	checksumsPath := filepath.Join(dir, "checksums.txt")
	require.NoError(t, os.WriteFile(checksumsPath, []byte("abc123  test.tar.gz\n"), 0o644))

	err := verifyChecksum(filepath.Join(dir, "nonexistent.tar.gz"), "test.tar.gz", checksumsPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compute checksum")
}

func TestUT_VerifyChecksum_ChecksumsFileNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "test.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, []byte("data"), 0o644))

	err := verifyChecksum(archivePath, "test.tar.gz", filepath.Join(dir, "nonexistent-checksums.txt"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open checksums")
}

func TestUT_FindChecksum_MultipleEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums.txt")
	content := "aaa111  file1.tar.gz\nbbb222  file2.tar.gz\nccc333  file3.tar.gz\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	hash, err := findChecksum(path, "file3.tar.gz")
	require.NoError(t, err)
	assert.Equal(t, "ccc333", hash)
}

func TestUT_FindChecksum_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums.txt")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o644))

	_, err := findChecksum(path, "test.tar.gz")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "checksum not found")
}

func TestUT_FindChecksum_MalformedLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums.txt")
	content := "malformed_no_spaces\nabc123  correct.tar.gz\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	hash, err := findChecksum(path, "correct.tar.gz")
	require.NoError(t, err)
	assert.Equal(t, "abc123", hash)
}

func TestUT_ReplaceBinary_NoDstFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dst := filepath.Join(dir, "nonexistent-binary")
	src := filepath.Join(dir, "new-binary")
	require.NoError(t, os.WriteFile(src, []byte("new"), 0o755))

	err := replaceBinary(dst, src)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup current binary")
}

func TestUT_CopyFile_DstDirNotExist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	require.NoError(t, os.WriteFile(src, []byte("content"), 0o644))

	dst := filepath.Join(dir, "nonexistent", "subdir", "dst")
	err := copyFile(src, dst)
	assert.Error(t, err)
}

func TestUT_Update_ChecksumsDownloadFailure(t *testing.T) {
	t.Parallel()
	binaryDir := t.TempDir()
	binaryPath := filepath.Join(binaryDir, "tag")
	require.NoError(t, os.WriteFile(binaryPath, []byte("old"), 0o755))

	archiveData := buildTestArchiveLocal(t, "tag", "new-content")

	platform, err := DetectPlatform()
	require.NoError(t, err)

	archiveName := fmt.Sprintf("tag_1.0.0_%s_%s.tar.gz", platform.OS, platform.Arch)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/download/v1.0.0/" + archiveName:
			_, _ = w.Write(archiveData)
		case "/releases/download/v1.0.0/checksums.txt":
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var buf bytes.Buffer
	u := New(srv.URL, &buf)
	err = u.Update(context.Background(), "v1.0.0", binaryPath)
	require.Error(t, err)

	var updateErr *UpdateError
	assert.ErrorAs(t, err, &updateErr)
	assert.Equal(t, "verify", updateErr.Op)
}

func TestUT_Update_VersionPrefixStripped(t *testing.T) {
	t.Parallel()
	binaryDir := t.TempDir()
	binaryPath := filepath.Join(binaryDir, "tag")
	require.NoError(t, os.WriteFile(binaryPath, []byte("old"), 0o755))

	archiveData := buildTestArchiveLocal(t, "tag", "new")
	h := sha256.Sum256(archiveData)
	checksum := hex.EncodeToString(h[:])

	platform, err := DetectPlatform()
	require.NoError(t, err)

	archiveName := fmt.Sprintf("tag_2.0.0_%s_%s.tar.gz", platform.OS, platform.Arch)

	var receivedPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPaths = append(receivedPaths, r.URL.Path)
		switch r.URL.Path {
		case "/releases/download/v2.0.0/" + archiveName:
			_, _ = w.Write(archiveData)
		case "/releases/download/v2.0.0/checksums.txt":
			fmt.Fprintf(w, "%s  %s\n", checksum, archiveName)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var buf bytes.Buffer
	u := New(srv.URL, &buf)
	err = u.Update(context.Background(), "v2.0.0", binaryPath)
	require.NoError(t, err)

	// Verify the archive name uses "2.0.0" (stripped v prefix) in one of the requests
	found := false
	for _, p := range receivedPaths {
		if p != "" && contains(p, "tag_2.0.0_") {
			found = true
			break
		}
	}
	assert.True(t, found, "archive request should contain stripped version")
}

func TestUT_DownloadFile_ContextCancellation(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("content"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var buf bytes.Buffer
	u := New(srv.URL, &buf)
	err := u.downloadFile(ctx, srv.URL+"/file", filepath.Join(t.TempDir(), "out"))
	assert.Error(t, err)
}

// buildTestArchiveLocal mirrors the existing buildTestArchive but avoids name conflict.
func buildTestArchiveLocal(t *testing.T, name, content string) []byte {
	t.Helper()
	return buildTestArchive(t, name, content)
}

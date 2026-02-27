package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Update_Success(t *testing.T) {
	// Create a fake binary to be "updated".
	binaryDir := t.TempDir()
	binaryPath := filepath.Join(binaryDir, "tag")
	require.NoError(t, os.WriteFile(binaryPath, []byte("old-binary"), 0o755))

	// Build a test archive.
	archiveData := buildTestArchive(t, "tag", "new-binary-content")
	checksum := sha256Hex(archiveData)

	platform, err := DetectPlatform()
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/releases/download/v1.0.0/checksums.txt":
			archiveName := fmt.Sprintf("tag_1.0.0_%s_%s.tar.gz", platform.OS, platform.Arch)
			fmt.Fprintf(w, "%s  %s\n", checksum, archiveName)
		case r.URL.Path == fmt.Sprintf("/releases/download/v1.0.0/tag_1.0.0_%s_%s.tar.gz", platform.OS, platform.Arch):
			w.Write(archiveData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var buf bytes.Buffer
	u := New(srv.URL, &buf)
	err = u.Update(context.Background(), "v1.0.0", binaryPath)
	require.NoError(t, err)

	// Verify the binary was replaced.
	content, err := os.ReadFile(binaryPath)
	require.NoError(t, err)
	assert.Equal(t, "new-binary-content", string(content))

	// Verify output messages.
	assert.Contains(t, buf.String(), "Downloading")
	assert.Contains(t, buf.String(), "Verifying checksum")
	assert.Contains(t, buf.String(), "Installing")
}

func TestUT_Update_ChecksumMismatch(t *testing.T) {
	binaryDir := t.TempDir()
	binaryPath := filepath.Join(binaryDir, "tag")
	require.NoError(t, os.WriteFile(binaryPath, []byte("old-binary"), 0o755))

	archiveData := buildTestArchive(t, "tag", "binary-content")

	platform, err := DetectPlatform()
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/releases/download/v1.0.0/checksums.txt":
			archiveName := fmt.Sprintf("tag_1.0.0_%s_%s.tar.gz", platform.OS, platform.Arch)
			fmt.Fprintf(w, "%s  %s\n", "0000000000000000000000000000000000000000000000000000000000000000", archiveName)
		case r.URL.Path == fmt.Sprintf("/releases/download/v1.0.0/tag_1.0.0_%s_%s.tar.gz", platform.OS, platform.Arch):
			w.Write(archiveData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var buf bytes.Buffer
	u := New(srv.URL, &buf)
	err = u.Update(context.Background(), "v1.0.0", binaryPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrChecksumMismatch)

	// Verify the original binary is still intact.
	content, err := os.ReadFile(binaryPath)
	require.NoError(t, err)
	assert.Equal(t, "old-binary", string(content))
}

func TestUT_Update_DownloadFailure(t *testing.T) {
	binaryDir := t.TempDir()
	binaryPath := filepath.Join(binaryDir, "tag")
	require.NoError(t, os.WriteFile(binaryPath, []byte("old-binary"), 0o755))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	u := New(srv.URL, &buf)
	err := u.Update(context.Background(), "v1.0.0", binaryPath)
	require.Error(t, err)

	var updateErr *UpdateError
	assert.ErrorAs(t, err, &updateErr)
	assert.Equal(t, "download", updateErr.Op)
}

func TestUT_Update_BinaryNotInArchive(t *testing.T) {
	binaryDir := t.TempDir()
	binaryPath := filepath.Join(binaryDir, "tag")
	require.NoError(t, os.WriteFile(binaryPath, []byte("old-binary"), 0o755))

	// Archive with wrong binary name.
	archiveData := buildTestArchive(t, "wrong-name", "content")
	checksum := sha256Hex(archiveData)

	platform, err := DetectPlatform()
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/releases/download/v1.0.0/checksums.txt":
			archiveName := fmt.Sprintf("tag_1.0.0_%s_%s.tar.gz", platform.OS, platform.Arch)
			fmt.Fprintf(w, "%s  %s\n", checksum, archiveName)
		case r.URL.Path == fmt.Sprintf("/releases/download/v1.0.0/tag_1.0.0_%s_%s.tar.gz", platform.OS, platform.Arch):
			w.Write(archiveData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var buf bytes.Buffer
	u := New(srv.URL, &buf)
	err = u.Update(context.Background(), "v1.0.0", binaryPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBinaryNotFound)
}

func TestUT_ReplaceBinary_Success(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "tag")
	src := filepath.Join(dir, "tag-new")

	require.NoError(t, os.WriteFile(dst, []byte("old"), 0o755))
	require.NoError(t, os.WriteFile(src, []byte("new"), 0o755))

	require.NoError(t, replaceBinary(dst, src))

	content, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "new", string(content))

	// Backup should be cleaned up.
	_, err = os.Stat(dst + ".old")
	assert.True(t, os.IsNotExist(err))
}

func TestUT_ReplaceBinary_RestoresOnFailure(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "tag")
	src := filepath.Join(dir, "nonexistent") // Will fail to open.

	require.NoError(t, os.WriteFile(dst, []byte("original"), 0o755))

	err := replaceBinary(dst, src)
	require.Error(t, err)

	// Original should be restored.
	content, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "original", string(content))
}

func TestUT_VerifyChecksum_Success(t *testing.T) {
	dir := t.TempDir()

	content := []byte("test content")
	archivePath := filepath.Join(dir, "archive.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, content, 0o644))

	h := sha256.Sum256(content)
	checksum := hex.EncodeToString(h[:])

	checksumsPath := filepath.Join(dir, "checksums.txt")
	require.NoError(t, os.WriteFile(checksumsPath, fmt.Appendf(nil, "%s  archive.tar.gz\n", checksum), 0o644))

	err := verifyChecksum(archivePath, "archive.tar.gz", checksumsPath)
	require.NoError(t, err)
}

func TestUT_VerifyChecksum_Mismatch(t *testing.T) {
	dir := t.TempDir()

	archivePath := filepath.Join(dir, "archive.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, []byte("content"), 0o644))

	checksumsPath := filepath.Join(dir, "checksums.txt")
	require.NoError(t, os.WriteFile(checksumsPath, []byte("0000000000000000000000000000000000000000000000000000000000000000  archive.tar.gz\n"), 0o644))

	err := verifyChecksum(archivePath, "archive.tar.gz", checksumsPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrChecksumMismatch)
}

func TestUT_FindChecksum_NotFound(t *testing.T) {
	dir := t.TempDir()
	checksumsPath := filepath.Join(dir, "checksums.txt")
	require.NoError(t, os.WriteFile(checksumsPath, []byte("abc123  other-file.tar.gz\n"), 0o644))

	_, err := findChecksum(checksumsPath, "missing.tar.gz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum not found")
}

// buildTestArchive creates a tar.gz archive in memory containing a single file.
func buildTestArchive(t *testing.T, name, content string) []byte {
	t.Helper()

	var buf bytes.Buffer

	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     name,
		Size:     int64(len(content)),
		Mode:     0o755,
		Typeflag: tar.TypeReg,
	}))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

package update

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ExtractFromTarGz_Success(t *testing.T) {
	archivePath := createTestTarGz(t, "tag", "hello-binary-content")
	destDir := t.TempDir()

	result, err := extractFromTarGz(archivePath, "tag", destDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(destDir, "tag"), result)

	content, err := os.ReadFile(result)
	require.NoError(t, err)
	assert.Equal(t, "hello-binary-content", string(content))
}

func TestUT_ExtractFromTarGz_NestedPath(t *testing.T) {
	// Binary inside a subdirectory in the archive (GoReleaser pattern).
	archivePath := createTestTarGzWithPath(t, "tag_1.0.0_Darwin_arm64/tag", "binary-data")
	destDir := t.TempDir()

	result, err := extractFromTarGz(archivePath, "tag", destDir)
	require.NoError(t, err)

	content, err := os.ReadFile(result)
	require.NoError(t, err)
	assert.Equal(t, "binary-data", string(content))
}

func TestUT_ExtractFromTarGz_BinaryNotFound(t *testing.T) {
	archivePath := createTestTarGz(t, "other-binary", "content")
	destDir := t.TempDir()

	_, err := extractFromTarGz(archivePath, "tag", destDir)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBinaryNotFound)
}

func TestUT_ExtractFromTarGz_InvalidArchive(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "bad.tar.gz")
	require.NoError(t, os.WriteFile(tmpFile, []byte("not a tar.gz"), 0o644))

	_, err := extractFromTarGz(tmpFile, "tag", t.TempDir())
	require.Error(t, err)
}

// createTestTarGz creates a tar.gz archive with a single file at the top level.
func createTestTarGz(t *testing.T, name, content string) string {
	t.Helper()
	return createTestTarGzWithPath(t, name, content)
}

// createTestTarGzWithPath creates a tar.gz archive with a file at the given path.
func createTestTarGzWithPath(t *testing.T, archivePath, content string) string {
	t.Helper()

	tarGzPath := filepath.Join(t.TempDir(), "test.tar.gz")

	f, err := os.Create(tarGzPath)
	require.NoError(t, err)
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     archivePath,
		Size:     int64(len(content)),
		Mode:     0o755,
		Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write([]byte(content))
	require.NoError(t, err)

	return tarGzPath
}

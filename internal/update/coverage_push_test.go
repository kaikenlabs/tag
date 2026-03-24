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

// ===========================================================================
// extract.go — tar read error (line 38-40)
// ===========================================================================

func TestUT_ExtractFromTarGz_CorruptTar(t *testing.T) {
	t.Parallel()

	// Create a valid gzip file but with corrupt tar content
	tarGzPath := filepath.Join(t.TempDir(), "corrupt.tar.gz")
	f, err := os.Create(tarGzPath)
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	// Write something that looks like tar but isn't valid
	_, err = gw.Write([]byte("not valid tar header data but long enough to cause read errors"))
	require.NoError(t, err)
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())

	_, err = extractFromTarGz(tarGzPath, "tag", t.TempDir())
	require.Error(t, err)
}

// ===========================================================================
// extract.go — create output file error (line 54-56)
// ===========================================================================

func TestUT_ExtractFromTarGz_CreateOutputError(t *testing.T) {
	t.Parallel()

	archivePath := createTestTarGz(t, "tag", "binary-content")

	// Use a non-existent directory with a file blocking the path
	destDir := filepath.Join(t.TempDir(), "blocked")
	require.NoError(t, os.WriteFile(destDir, []byte("blocker"), 0o644))

	_, err := extractFromTarGz(archivePath, "tag", destDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create output file")
}

// ===========================================================================
// extract.go — open archive error (line 20-22 is already covered)
// extract.go — typeflag non-regular skipped (line 47-49)
// ===========================================================================

func TestUT_ExtractFromTarGz_SkipsDirectoryEntries(t *testing.T) {
	t.Parallel()

	// Create archive with a directory entry named "tag" and a regular file named "other"
	tarGzPath := filepath.Join(t.TempDir(), "test.tar.gz")
	f, err := os.Create(tarGzPath)
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add a directory entry named "tag"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "tag",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	}))

	// Add a regular file with a different name
	content := "other-content"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "not-tag",
		Size:     int64(len(content)),
		Mode:     0o755,
		Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write([]byte(content))
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())

	_, err = extractFromTarGz(tarGzPath, "tag", t.TempDir())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBinaryNotFound)
}

// ===========================================================================
// extract.go — non-existent archive path
// ===========================================================================

func TestUT_ExtractFromTarGz_NonExistentArchive(t *testing.T) {
	t.Parallel()

	_, err := extractFromTarGz("/nonexistent/path.tar.gz", "tag", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open archive")
}

// ===========================================================================
// extract.go — multiple files in archive, binary found (exercises loop + match)
// ===========================================================================

func TestUT_ExtractFromTarGz_MultipleFiles_MatchesCorrectBinary(t *testing.T) {
	t.Parallel()

	tarGzPath := filepath.Join(t.TempDir(), "multi.tar.gz")
	f, err := os.Create(tarGzPath)
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add a non-matching file first
	readme := "README content"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "tag_v1/README.md",
		Size:     int64(len(readme)),
		Mode:     0o644,
		Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write([]byte(readme))
	require.NoError(t, err)

	// Add the matching binary
	binary := "binary-data-here"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "tag_v1/tag",
		Size:     int64(len(binary)),
		Mode:     0o755,
		Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write([]byte(binary))
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())

	destDir := t.TempDir()
	result, err := extractFromTarGz(tarGzPath, "tag", destDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(destDir, "tag"), result)

	data, err := os.ReadFile(result)
	require.NoError(t, err)
	assert.Equal(t, binary, string(data))
}

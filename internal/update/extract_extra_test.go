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

func TestUT_ExtractFromTarGz_SkipsNonRegularFiles(t *testing.T) {
	t.Parallel()
	// Create a tar.gz with a directory entry and a regular file
	tarGzPath := filepath.Join(t.TempDir(), "test.tar.gz")

	f, err := os.Create(tarGzPath)
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add a directory entry named "tag" (should be skipped)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "tag",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	}))

	// Add the actual file under a subdirectory
	content := "real binary"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "dist/tag",
		Size:     int64(len(content)),
		Mode:     0o755,
		Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write([]byte(content))
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())

	destDir := t.TempDir()
	result, err := extractFromTarGz(tarGzPath, "tag", destDir)
	require.NoError(t, err)

	data, err := os.ReadFile(result)
	require.NoError(t, err)
	assert.Equal(t, "real binary", string(data))
}

func TestUT_ExtractFromTarGz_EmptyArchive(t *testing.T) {
	t.Parallel()
	tarGzPath := filepath.Join(t.TempDir(), "empty.tar.gz")

	f, err := os.Create(tarGzPath)
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())

	_, err = extractFromTarGz(tarGzPath, "tag", t.TempDir())
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrBinaryNotFound)
}

func TestUT_ExtractFromTarGz_NonexistentFile(t *testing.T) {
	t.Parallel()
	_, err := extractFromTarGz(filepath.Join(t.TempDir(), "nope.tar.gz"), "tag", t.TempDir())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "open archive")
}

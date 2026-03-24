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

func TestUT_ExtractFromTarGz_SkipsSymlinks(t *testing.T) {
	t.Parallel()

	tarGzPath := filepath.Join(t.TempDir(), "test.tar.gz")

	f, err := os.Create(tarGzPath)
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add a symlink entry named "tag" (should be skipped since it's not TypeReg)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "tag",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
		Mode:     0o755,
	}))

	// Add the actual file under a different name
	content := "actual binary"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "real/tag",
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
	assert.Equal(t, "actual binary", string(data))
}

func TestUT_ExtractFromTarGz_MultipleFilesFindsCorrect(t *testing.T) {
	t.Parallel()

	tarGzPath := filepath.Join(t.TempDir(), "multi.tar.gz")

	f, err := os.Create(tarGzPath)
	require.NoError(t, err)

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add a README file
	readme := "# README"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "README.md",
		Size:     int64(len(readme)),
		Mode:     0o644,
		Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write([]byte(readme))
	require.NoError(t, err)

	// Add the target binary
	binary := "the-binary"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "tag",
		Size:     int64(len(binary)),
		Mode:     0o755,
		Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write([]byte(binary))
	require.NoError(t, err)

	// Add another file after
	license := "MIT License"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "LICENSE",
		Size:     int64(len(license)),
		Mode:     0o644,
		Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write([]byte(license))
	require.NoError(t, err)

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	require.NoError(t, f.Close())

	destDir := t.TempDir()
	result, err := extractFromTarGz(tarGzPath, "tag", destDir)
	require.NoError(t, err)

	data, err := os.ReadFile(result)
	require.NoError(t, err)
	assert.Equal(t, "the-binary", string(data))
}

func TestUT_ExtractFromTarGz_MatchesByBaseName(t *testing.T) {
	t.Parallel()

	// GoReleaser puts the binary inside a subdirectory
	tarGzPath := createTestTarGzWithPath(t, "tag_1.2.3_Linux_x86_64/tag", "linux-binary")
	destDir := t.TempDir()

	result, err := extractFromTarGz(tarGzPath, "tag", destDir)
	require.NoError(t, err)

	data, err := os.ReadFile(result)
	require.NoError(t, err)
	assert.Equal(t, "linux-binary", string(data))
}

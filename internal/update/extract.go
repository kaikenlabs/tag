package update

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// maxBinarySize is the maximum allowed binary size (256 MB) to guard against decompression bombs.
const maxBinarySize = 256 << 20

// extractFromTarGz extracts the named binary from a tar.gz archive and writes it to destDir.
// Returns the path to the extracted binary.
func extractFromTarGz(archivePath, binaryName, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar: %w", err)
		}

		// Match the binary name regardless of directory prefix in the archive.
		if filepath.Base(hdr.Name) != binaryName {
			continue
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		destPath := filepath.Join(destDir, binaryName)

		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", fmt.Errorf("create output file: %w", err)
		}

		if _, err := io.Copy(out, io.LimitReader(tr, maxBinarySize)); err != nil {
			out.Close()
			return "", fmt.Errorf("extract binary: %w", err)
		}

		if err := out.Sync(); err != nil {
			out.Close()
			return "", fmt.Errorf("sync binary: %w", err)
		}

		if err := out.Close(); err != nil {
			return "", fmt.Errorf("close binary: %w", err)
		}

		return destPath, nil
	}

	return "", fmt.Errorf("%w: %s", ErrBinaryNotFound, binaryName)
}

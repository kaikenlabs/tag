package history

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// HashFile computes a hex-encoded SHA256 hash of the file at path.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

// HashBytes computes a hex-encoded SHA256 hash of b.
func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("sha256:%x", sum)
}

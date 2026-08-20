package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const httpTimeout = 60 * time.Second

// Updater downloads and installs TAG binary updates from GitHub Releases.
type Updater struct {
	client     *http.Client
	out        io.Writer
	repoURL    string
	binaryName string
}

// New creates an Updater for the given GitHub repository URL.
func New(repoURL string, out io.Writer) *Updater {
	return &Updater{
		client: &http.Client{
			Timeout: httpTimeout,
		},
		out:        out,
		repoURL:    repoURL,
		binaryName: "tag",
	}
}

// Update downloads the specified version and replaces the binary at binaryPath.
func (u *Updater) Update(ctx context.Context, version, binaryPath string) error {
	platform, err := DetectPlatform()
	if err != nil {
		return err
	}

	fmt.Fprintf(u.out, "Downloading tag %s for %s/%s...\n", version, platform.OS, platform.Arch)

	tmpDir, err := os.MkdirTemp("", "tag-update-*")
	if err != nil {
		return &UpdateError{Op: "download", Err: fmt.Errorf("create temp dir: %w", err)}
	}
	defer os.RemoveAll(tmpDir)

	// Build asset URLs.
	ver := strings.TrimPrefix(version, "v")
	archiveName := fmt.Sprintf("tag_%s_%s_%s%s", ver, platform.OS, platform.Arch, platform.ArchiveExt)
	archiveURL := fmt.Sprintf("%s/releases/download/%s/%s", u.repoURL, version, archiveName)
	checksumsURL := fmt.Sprintf("%s/releases/download/%s/checksums.txt", u.repoURL, version)

	// Download archive.
	archivePath := filepath.Join(tmpDir, archiveName)
	err = u.downloadFile(ctx, archiveURL, archivePath)
	if err != nil {
		return &UpdateError{Op: "download", Err: err}
	}

	// Download and verify checksum.
	fmt.Fprintln(u.out, "Verifying checksum...")

	checksumsPath := filepath.Join(tmpDir, "checksums.txt")
	err = u.downloadFile(ctx, checksumsURL, checksumsPath)
	if err != nil {
		return &UpdateError{Op: "verify", Err: fmt.Errorf("download checksums: %w", err)}
	}

	err = verifyChecksum(archivePath, archiveName, checksumsPath)
	if err != nil {
		return &UpdateError{Op: "verify", Err: err}
	}

	// Extract binary.
	fmt.Fprintln(u.out, "Installing...")

	newBinaryPath, err := extractFromTarGz(archivePath, u.binaryName, tmpDir)
	if err != nil {
		return &UpdateError{Op: "extract", Err: err}
	}

	// Replace current binary.
	err = replaceBinary(binaryPath, newBinaryPath)
	if err != nil {
		return &UpdateError{Op: "replace", Err: err}
	}

	return nil
}

// downloadFile downloads a URL to a local file path.
func (u *Updater) downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := u.client.Do(req) //nolint:gosec // G704: URL is a fixed GitHub releases endpoint, not user-supplied
	if err != nil {
		return fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return fmt.Errorf("write file: %w", err)
	}

	if err := out.Sync(); err != nil {
		out.Close()
		return fmt.Errorf("sync file: %w", err)
	}

	return out.Close()
}

// verifyChecksum checks the SHA256 of archivePath against the entry in checksumsPath.
func verifyChecksum(archivePath, archiveName, checksumsPath string) error {
	expected, err := findChecksum(checksumsPath, archiveName)
	if err != nil {
		return err
	}

	actual, err := fileSHA256(archivePath)
	if err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}

	if actual != expected {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumMismatch, expected, actual)
	}

	return nil
}

// findChecksum parses a GoReleaser checksums.txt and returns the hash for the given filename.
func findChecksum(checksumsPath, filename string) (string, error) {
	f, err := os.Open(checksumsPath)
	if err != nil {
		return "", fmt.Errorf("open checksums: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// Format: "<hash>  <filename>" (two spaces between hash and filename).
		parts := strings.Fields(scanner.Text())
		if len(parts) == 2 && parts[1] == filename {
			return parts[0], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}

	return "", fmt.Errorf("checksum not found for %s", filename)
}

// fileSHA256 computes the hex-encoded SHA256 hash of a file.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// replaceBinary safely replaces the binary at dst with the one at src.
// Uses rename-to-backup strategy: rename current → .old, copy new, remove .old.
// Restores the original on failure.
func replaceBinary(dst, src string) error {
	backup := dst + ".old"

	// Remove any stale backup from a previous failed update.
	os.Remove(backup)

	if err := os.Rename(dst, backup); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	if err := copyFile(src, dst); err != nil {
		// Restore from backup.
		if restoreErr := os.Rename(backup, dst); restoreErr != nil {
			return fmt.Errorf("replace failed (%w) and restore failed (%w)", err, restoreErr)
		}
		return fmt.Errorf("copy new binary: %w", err)
	}

	os.Remove(backup)

	return nil
}

// copyFile copies src to dst, preserving executable permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// #nosec G302 -- dst is the replacement tag binary and must stay executable
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}

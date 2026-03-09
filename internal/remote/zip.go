package remote

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kaikenlabs/tag/internal/types"
)

// ZipFetcher fetches templates from zip files (remote or local).
type ZipFetcher struct {
	client      *http.Client
	out         io.Writer
	maxFileSize int64 // Maximum size for downloads (default 500MB)
	maxExtract  int64 // Maximum extracted size (default 1GB)
	maxFiles    int   // Maximum number of files to extract (default 10000)
}

// NewZipFetcher creates a new Zip fetcher.
func NewZipFetcher() *ZipFetcher {
	return &ZipFetcher{
		client: &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				ForceAttemptHTTP2:     true,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				IdleConnTimeout:       90 * time.Second,
				MaxIdleConns:          10,
				MaxIdleConnsPerHost:   5,
			},
		},
		out:         os.Stderr,
		maxFileSize: 500 * 1024 * 1024,  // 500MB
		maxExtract:  1024 * 1024 * 1024, // 1GB
		maxFiles:    10000,
	}
}

// Fetch downloads (if remote) and extracts the zip file.
// Returns the path to the extracted template directory.
// CommitSHA is always empty for zip sources.
func (f *ZipFetcher) Fetch(ctx context.Context, ref *Reference) (*FetchResult, error) {
	if ref.Type != ReferenceTypeZip {
		return nil, &FetchError{Ref: ref, Message: "not a Zip reference"}
	}

	// Determine if remote or local
	urlLower := strings.ToLower(ref.URL)
	isRemote := strings.HasPrefix(urlLower, "http://") || strings.HasPrefix(urlLower, "https://")

	// Security: reject insecure HTTP URLs for remote downloads
	if strings.HasPrefix(urlLower, "http://") {
		return nil, &FetchError{
			Ref:     ref,
			Message: "insecure HTTP URL rejected; use HTTPS instead",
		}
	}

	var zipPath string
	var tmpZip bool
	var err error

	if isRemote {
		// Download to temp file
		f.writeStatus("Downloading template...")
		zipPath, err = f.download(ctx, ref.URL)
		if err != nil {
			return nil, &FetchError{Ref: ref, Message: "download failed", Err: err}
		}
		tmpZip = true
	} else {
		// Use local path directly
		zipPath = ref.URL
	}

	// Create temp directory for extraction
	tmpDir, err := os.MkdirTemp("", "tag-zip-*")
	if err != nil {
		if tmpZip {
			os.Remove(zipPath)
		}
		return nil, &FetchError{Ref: ref, Message: "cannot create temp directory", Err: err}
	}

	// Clean up on error
	success := false
	defer func() {
		if tmpZip {
			os.Remove(zipPath)
		}
		if !success {
			os.RemoveAll(tmpDir)
		}
	}()

	// Extract zip
	f.writeStatus("Extracting template...")
	if err := f.extract(zipPath, tmpDir); err != nil { //nolint:govet // shadow in if-init is idiomatic
		return nil, &FetchError{Ref: ref, Message: "extraction failed", Err: err}
	}

	// Handle single root directory (unwrap if needed)
	resultPath, err := f.unwrapSingleRoot(tmpDir)
	if err != nil {
		return nil, &FetchError{Ref: ref, Message: "cannot unwrap root", Err: err}
	}

	// Apply subpath if specified
	if ref.SubPath != "" {
		resultPath = filepath.Join(resultPath, ref.SubPath)
		info, err := os.Stat(resultPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, &FetchError{
					Ref:     ref,
					Message: fmt.Sprintf("subpath %q not found in archive", ref.SubPath),
					Err:     ErrSubPathNotFound,
				}
			}
			return nil, &FetchError{Ref: ref, Message: "cannot access subpath", Err: err}
		}
		if !info.IsDir() {
			return nil, &FetchError{
				Ref:     ref,
				Message: fmt.Sprintf("subpath %q is not a directory", ref.SubPath),
			}
		}
	}

	f.writeStatus("") // Clear status line
	success = true
	return &FetchResult{
		Path:    resultPath,
		Version: ref.Version,
	}, nil
}

// download fetches a remote zip file to a temporary location.
func (f *ZipFetcher) download(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := f.client.Do(req) //nolint:gosec // G704: URL is validated by the caller's reference parser
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "tag-download-*.zip")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Limit download size
	limited := io.LimitReader(resp.Body, f.maxFileSize)

	_, err = io.Copy(tmpFile, limited)
	closeErr := tmpFile.Close()

	if err != nil {
		os.Remove(tmpPath) //nolint:gosec // G703: tmpPath is from os.CreateTemp, not user-controlled
		return "", fmt.Errorf("write file: %w", err)
	}
	if closeErr != nil {
		os.Remove(tmpPath) //nolint:gosec // G703: tmpPath is from os.CreateTemp, not user-controlled
		return "", fmt.Errorf("close temp file: %w", closeErr)
	}

	return tmpPath, nil
}

// extract extracts a zip file to the destination directory.
func (f *ZipFetcher) extract(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	// Track actual extracted bytes cumulatively (not from forged headers)
	counter := &countingWriter{max: f.maxExtract}
	fileCount := 0

	for _, file := range r.File {
		// Check file count limit
		fileCount++
		if fileCount > f.maxFiles {
			return fmt.Errorf("too many files in archive (max %d)", f.maxFiles)
		}

		// Security: prevent zip slip attacks
		destPath, err := f.sanitizePath(destDir, file.Name)
		if err != nil {
			return err
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, types.DirMode); err != nil {
				return fmt.Errorf("create directory: %w", err)
			}
			continue
		}

		// Extract file with cumulative size tracking
		if err := f.extractFile(file, destPath, counter); err != nil {
			return err
		}
	}

	return nil
}

// sanitizePath ensures the extracted path is within the destination directory.
func (f *ZipFetcher) sanitizePath(destDir, filePath string) (string, error) {
	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(filePath)

	// Reject absolute paths
	if filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("invalid path in zip: absolute path not allowed: %s", filePath)
	}

	// Reject paths that try to escape
	if strings.HasPrefix(cleanPath, "..") {
		return "", fmt.Errorf("invalid path in zip: path traversal not allowed: %s", filePath)
	}

	destPath := filepath.Join(destDir, cleanPath)

	// Double-check the result is within destDir
	if !strings.HasPrefix(filepath.Clean(destPath)+string(os.PathSeparator), filepath.Clean(destDir)+string(os.PathSeparator)) &&
		filepath.Clean(destPath) != filepath.Clean(destDir) {
		return "", fmt.Errorf("invalid path in zip: escapes destination: %s", filePath)
	}

	return destPath, nil
}

// extractFile extracts a single file from the zip.
// The counter tracks cumulative bytes across all files and enforces the global limit.
func (f *ZipFetcher) extractFile(file *zip.File, destPath string, counter *countingWriter) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), types.DirMode); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	// Open source file in zip
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("open zip entry: %w", err)
	}
	defer src.Close()

	// Sanitize file mode (remove setuid, setgid, sticky bits)
	mode := file.Mode() & 0o777

	// Create destination file
	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer dst.Close()

	// Copy through the counting writer to enforce cumulative size limit
	counter.dst = dst
	_, err = io.Copy(counter, src) //nolint:gosec // zip size is bounded by download limit and io.LimitReader earlier
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// countingWriter wraps a destination writer and tracks cumulative bytes written.
// It returns an error if the cumulative total exceeds the maximum.
type countingWriter struct {
	dst   io.Writer
	total int64
	max   int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.total += int64(len(p))
	if w.total > w.max {
		return 0, fmt.Errorf("extracted size exceeds limit (%d bytes)", w.max)
	}
	return w.dst.Write(p)
}

// unwrapSingleRoot checks if the extracted content has a single root directory
// and returns the path to it. If there are multiple items at root, returns the
// original directory.
func (f *ZipFetcher) unwrapSingleRoot(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	// Filter out hidden files and metadata
	var visibleEntries []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files and common metadata
		if strings.HasPrefix(name, ".") || name == "__MACOSX" {
			continue
		}
		visibleEntries = append(visibleEntries, entry)
	}

	// If single directory at root, return that
	if len(visibleEntries) == 1 && visibleEntries[0].IsDir() {
		return filepath.Join(dir, visibleEntries[0].Name()), nil
	}

	// Multiple items or single file, return original
	return dir, nil
}

// writeStatus writes a status message using carriage return for in-place updates.
func (f *ZipFetcher) writeStatus(msg string) {
	w := f.out
	if w == nil {
		return
	}
	fmt.Fprintf(w, "\r%-40s", msg)
}

// Ensure ZipFetcher implements Fetcher.
var _ Fetcher = (*ZipFetcher)(nil)

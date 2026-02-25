package fileutil

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// IsTextContent checks if content appears to be text rather than binary.
// It inspects the first 8KB for null bytes, invalid UTF-8, and non-printable characters.
func IsTextContent(content []byte) bool {
	if len(content) == 0 {
		return true
	}

	// Check first 8KB for binary indicators
	sample := content
	if len(sample) > 8192 {
		sample = sample[:8192]
	}

	// Null bytes are a strong binary indicator
	if bytes.Contains(sample, []byte{0}) {
		return false
	}

	// Must be valid UTF-8
	if !utf8.Valid(sample) {
		return false
	}

	// Count non-printable characters (excluding common whitespace)
	nonPrintable := 0
	for _, b := range sample {
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			nonPrintable++
		}
	}

	// If more than 10% non-printable, likely binary
	return float64(nonPrintable)/float64(len(sample)) < 0.1
}

// CopyFile copies a single file from src to dst, preserving permissions.
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Strip setuid, setgid, and sticky bits to prevent privilege escalation
	// from untrusted template sources.
	mode := srcInfo.Mode() &^ (os.ModeSetuid | os.ModeSetgid | os.ModeSticky)

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		return err
	}

	if err = dstFile.Sync(); err != nil {
		dstFile.Close()
		return err
	}

	return dstFile.Close()
}

// CopyDir recursively copies a directory from src to dst.
// Symlinks are skipped for security. A TOCTOU mitigation re-checks each file
// with Lstat before copying to detect symlinks created between walk and copy.
func CopyDir(src, dst string, dirMode os.FileMode) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks (detected at walk time via Lstat)
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, dirMode)
		}

		if mkdirErr := os.MkdirAll(filepath.Dir(destPath), dirMode); mkdirErr != nil {
			return mkdirErr
		}

		// TOCTOU mitigation: re-check with Lstat before copy to confirm
		// the file hasn't been replaced with a symlink since WalkDir's Lstat.
		info, lstatErr := os.Lstat(path)
		if lstatErr != nil {
			return lstatErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil // skip: became a symlink between walk and copy
		}

		return CopyFile(path, destPath)
	})
}

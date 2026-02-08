package fileutil

import (
	"bytes"
	"io"
	"os"
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

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

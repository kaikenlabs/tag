package update

import (
	"errors"
	"fmt"
)

// Sentinel errors for update operations.
var (
	// ErrUnsupportedPlatform indicates the current OS/architecture is not supported.
	ErrUnsupportedPlatform = errors.New("unsupported platform")

	// ErrChecksumMismatch indicates the downloaded archive failed SHA256 verification.
	ErrChecksumMismatch = errors.New("checksum mismatch")

	// ErrBinaryNotFound indicates the binary was not found inside the archive.
	ErrBinaryNotFound = errors.New("binary not found in archive")
)

// UpdateError represents an error during the update process.
type UpdateError struct {
	Op  string // "download", "verify", "extract", "replace"
	Err error
}

func (e *UpdateError) Error() string {
	return fmt.Sprintf("update %s: %v", e.Op, e.Err)
}

func (e *UpdateError) Unwrap() error {
	return e.Err
}

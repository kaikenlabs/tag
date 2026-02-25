package validate

import (
	"errors"
	"fmt"
	"strings"
)

// MaxNameLen is the maximum allowed length for a template name.
const MaxNameLen = 255

// ErrInvalidName indicates a name failed validation.
var ErrInvalidName = errors.New("invalid name")

// PathSegmentSafe checks that a string is safe to use as a single path segment.
// It rejects empty/whitespace-only values, "." and "..", path separators, and
// path traversal sequences.
func PathSegmentSafe(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: must not be empty", ErrInvalidName)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("%w: %q is a reserved name", ErrInvalidName, name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("%w: %q contains path traversal sequence", ErrInvalidName, name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%w: %q contains path separator", ErrInvalidName, name)
	}
	return nil
}

// TemplateName validates a name for use as a template/library directory name.
// It applies PathSegmentSafe checks plus additional constraints: max length,
// no dot prefix, and no control characters.
func TemplateName(name string) error {
	if err := PathSegmentSafe(name); err != nil {
		return err
	}
	if len(name) > MaxNameLen {
		return fmt.Errorf("%w: exceeds maximum length of %d", ErrInvalidName, MaxNameLen)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("%w: name cannot start with a dot", ErrInvalidName)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7F {
			return fmt.Errorf("%w: contains control character", ErrInvalidName)
		}
	}
	return nil
}

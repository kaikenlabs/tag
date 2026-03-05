package validate

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// MaxNameLen is the maximum allowed length for a template name.
const MaxNameLen = 255

// ErrInvalidName indicates a name failed validation.
var ErrInvalidName = errors.New("invalid name")

// ReservedGeneratorNames are names that conflict with "tag generate" subcommands
// and cannot be used as generator or bundle names.
var ReservedGeneratorNames = []string{"list", "ls", "info", "agent-file"}

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

// GeneratorName validates a name for use as a generator or bundle name.
// It applies PathSegmentSafe checks and rejects names that conflict with
// "tag generate" subcommands (list, ls, info, agent-file).
func GeneratorName(name string) error {
	if err := PathSegmentSafe(name); err != nil {
		return err
	}
	if slices.Contains(ReservedGeneratorNames, name) {
		return fmt.Errorf("%w: %q is a reserved name (conflicts with \"tag generate %s\" subcommand)", ErrInvalidName, name, name)
	}
	return nil
}

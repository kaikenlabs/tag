package commands

import (
	"github.com/kaikenlabs/tag/internal/validate"
)

// ValidateNameSafe checks that a CLI-provided name is safe to use as a path segment.
// It rejects names containing path traversal sequences, path separators, or that are empty.
func ValidateNameSafe(name string) error {
	return validate.PathSegmentSafe(name)
}

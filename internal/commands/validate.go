package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kaikenlabs/tag/internal/fileutil"
)

// ValidateNameSafe checks that a CLI-provided name is safe to use as a path segment.
// It rejects names containing path traversal sequences, path separators, or that are empty.
func ValidateNameSafe(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name must not be empty")
	}

	if strings.Contains(name, "..") {
		return fmt.Errorf("name %q contains path traversal sequence", name)
	}

	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("name %q contains path separator", name)
	}

	if name == "." {
		return fmt.Errorf("name %q is not a valid name", name)
	}

	return nil
}

// ValidatePathContainment checks that the resolved path stays within the base directory.
func ValidatePathContainment(basePath, fullPath string) error {
	return fileutil.ValidatePathContainment(basePath, fullPath)
}

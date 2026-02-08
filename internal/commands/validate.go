package commands

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateNameSafe checks that a CLI-provided name is safe to use as a path segment.
// It rejects names containing path traversal sequences, path separators, or that are empty.
func ValidateNameSafe(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name must not be empty")
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
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("failed to resolve base path: %w", err)
	}

	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	rel, err := filepath.Rel(absBase, absFull)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes base directory %q", fullPath, basePath)
	}

	return nil
}

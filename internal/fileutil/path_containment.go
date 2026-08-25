package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePathContainment checks that fullPath is contained within basePath.
// Both paths are resolved through filepath.EvalSymlinks (with fallback for
// non-existent targets) and made absolute before comparison. This prevents
// path traversal attacks including those using symlinks.
func ValidatePathContainment(basePath, fullPath string) error {
	absBase, err := resolveForContainment(basePath)
	if err != nil {
		return fmt.Errorf("failed to resolve base path: %w", err)
	}

	absTarget, err := resolveForContainment(fullPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if target is within base directory (or equals it)
	if absTarget != absBase && !strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes base directory %q", fullPath, basePath)
	}

	return nil
}

// resolveForContainment resolves a path for containment checking.
// The path is made absolute BEFORE symlink resolution: filepath.EvalSymlinks
// returns a relative result for a relative argument (EvalSymlinks(".") is "."),
// so resolving first and calling filepath.Abs afterwards would reintroduce the
// unresolved working directory reported by os.Getwd. That asymmetry made a
// relative target escape an absolute base whenever the working directory itself
// sat under a symlink (e.g. macOS /var -> /private/var).
// If the absolute path doesn't exist, it walks up the directory tree to find the
// nearest existing ancestor, resolves that through EvalSymlinks, and appends the
// remaining path segments.
func resolveForContainment(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to make path absolute for %q: %w", path, err)
	}

	// Try full resolution first
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}

	// If path doesn't exist, walk up to find existing ancestor
	if os.IsNotExist(err) {
		return resolveNonExistent(abs)
	}

	return "", fmt.Errorf("failed to evaluate symlinks for %q: %w", path, err)
}

// resolveNonExistent resolves a non-existent absolute path by walking up the directory
// tree to find the nearest existing ancestor, resolving it through EvalSymlinks,
// and appending the remaining segments.
func resolveNonExistent(cleaned string) (string, error) {
	// Collect non-existent segments from bottom up
	current := cleaned
	var segments []string

	for {
		_, err := os.Lstat(current)
		if err == nil {
			// Found existing ancestor; resolve it
			resolved, evalErr := filepath.EvalSymlinks(current)
			if evalErr != nil {
				return filepath.Abs(cleaned)
			}
			absResolved, absErr := filepath.Abs(resolved)
			if absErr != nil {
				return filepath.Abs(cleaned)
			}
			// Append the collected non-existent segments
			result := absResolved
			for i := len(segments) - 1; i >= 0; i-- {
				result = filepath.Join(result, segments[i])
			}
			return result, nil
		}

		if !os.IsNotExist(err) {
			// Some other error (permission, etc.)
			return filepath.Abs(cleaned)
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached root without finding existing ancestor
			return filepath.Abs(cleaned)
		}
		segments = append(segments, filepath.Base(current))
		current = parent
	}
}

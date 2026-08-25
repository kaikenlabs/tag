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
//
// This is a validation-time predicate over a PATHNAME, not a binding
// capability over an open file or directory handle. It protects against
// traversal and pre-existing symlink escapes at the moment it runs, but does
// NOT protect against a concurrent process replacing a directory in the
// resolved path with a symlink between this check and the write that follows
// it (TOCTOU). The accepted threat model is that tag's destination workspace
// is not concurrently writable by an attacker while tag runs; if that ever
// changes, callers must move to rooted-descriptor APIs (e.g. os.Root) instead
// of re-validating pathnames.
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

// resolveNonExistent resolves a non-existent absolute path by walking up the
// directory tree to find the nearest existing ancestor, resolving it through
// EvalSymlinks, and appending the remaining segments.
//
// This fails CLOSED: any ancestor that cannot be conclusively resolved (a
// stat error other than not-exist, or a symlink EvalSymlinks cannot follow —
// most commonly a dangling symlink) returns an error rather than the
// unresolved input path. A predicate that cannot resolve a path has no basis
// to grant permission, and every caller of ValidatePathContainment treats a
// nil error as authorization to write. Before this, an unresolvable ancestor
// silently fell back to the caller-supplied path, which — because that path
// is already prefixed with the base directory by construction — satisfied
// the containment check even when it pointed (via a dangling symlink)
// somewhere ValidatePathContainment could not actually verify.
func resolveNonExistent(cleaned string) (string, error) {
	current := cleaned
	var segments []string

	for {
		if _, err := os.Lstat(current); err != nil {
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("failed to stat ancestor %q of %q: %w", current, cleaned, err)
			}
			parent := filepath.Dir(current)
			if parent == current {
				return "", fmt.Errorf("no existing ancestor for %q", cleaned)
			}
			segments = append(segments, filepath.Base(current))
			current = parent
			continue
		}

		resolved, evalErr := filepath.EvalSymlinks(current)
		if evalErr != nil {
			return "", fmt.Errorf("failed to evaluate symlinks for ancestor %q of %q: %w", current, cleaned, evalErr)
		}

		result := resolved
		for i := len(segments) - 1; i >= 0; i-- {
			result = filepath.Join(result, segments[i])
		}
		return result, nil
	}
}

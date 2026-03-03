package lockfile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// HashTemplateDir computes a deterministic SHA-256 hash of a template directory's
// contents. The hash captures every regular file's relative path and content, in
// lexicographic order, so it is stable across OS, file-system traversal order, and
// directory modification times.
//
// Algorithm (format version "tag-lock-v1"):
//
//  1. Walk the tree and collect every regular file (symlinks are not followed).
//  2. Skip lock.json itself and any file matched by .tagignore patterns.
//  3. Sort entries by their normalized (forward-slash) relative path.
//  4. For each entry write a record:  "tag-lock-v1\t<relpath>\t<file-sha256>\n"
//  5. Return the hex-encoded SHA-256 of all records concatenated.
func HashTemplateDir(dir string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	type entry struct {
		rel  string // forward-slash normalized
		hash string // hex sha256 of file content
	}

	var entries []entry

	err = filepath.WalkDir(absDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		// Normalize relative path to forward slashes for cross-platform stability.
		rel, relErr := filepath.Rel(absDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)

		// Skip the lockfile itself.
		if rel == LockFileName {
			return nil
		}
		// Skip OS noise and editor artefacts that leak into template dirs.
		if isIgnoredFileName(rel) {
			return nil
		}

		h, hashErr := sha256File(path)
		if hashErr != nil {
			return fmt.Errorf("hash %s: %w", rel, hashErr)
		}
		entries = append(entries, entry{rel: rel, hash: h})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk %s: %w", absDir, err)
	}

	// Stable sort (WalkDir is already lexicographic on most platforms, but we
	// sort explicitly to guarantee cross-platform determinism).
	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })

	h := sha256.New()
	for _, e := range entries {
		fmt.Fprintf(h, "tag-lock-v1\t%s\t%s\n", e.rel, e.hash)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// sha256File returns the hex-encoded SHA-256 of a file's content.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// isIgnoredFileName returns true for common OS/editor noise files that should
// not affect a template's content hash.
func isIgnoredFileName(rel string) bool {
	base := filepath.Base(rel)
	ignored := []string{".DS_Store", "Thumbs.db", "desktop.ini"}
	if slices.Contains(ignored, base) {
		return true
	}
	// Skip .swp / .swo (vim swap files)
	return strings.HasSuffix(base, ".swp") || strings.HasSuffix(base, ".swo")
}

// Package lockfile provides template provenance tracking via a .tag/lock.json
// file. Each entry pins a remote template reference to a specific version and
// SHA-256 content hash, enabling reproducible scaffolding and tamper detection.
package lockfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// LockFileName is the name of the lockfile inside .tag/.
const LockFileName = "lock.json"

// File is the in-memory representation of .tag/lock.json.
type File struct {
	// Version is a format version for future evolution.
	Version int `json:"version"`
	// Templates maps a canonical template ref to its locked entry.
	Templates map[string]*Entry `json:"templates"`
}

// Entry records the locked version and content hash for one template reference.
type Entry struct {
	Ref        string    `json:"ref"`
	Version    string    `json:"version,omitempty"`
	SHA256     string    `json:"sha256"`
	ResolvedAt time.Time `json:"resolved_at"`
}

// VerifyOptions controls lockfile verification behaviour.
type VerifyOptions struct {
	// UpdateLock refreshes the lockfile entry even if it already exists.
	UpdateLock bool
	// IgnoreLock skips all verification (a warning is still emitted).
	IgnoreLock bool
	// DryRun suppresses the create/refresh write while leaving verification
	// intact — a mismatch still returns ErrChecksumMismatch.
	DryRun bool
}

// ErrChecksumMismatch is returned when the computed hash does not match the
// stored hash.
var ErrChecksumMismatch = errors.New("template checksum mismatch")

// lockfilePath returns the canonical path of the lock file for a project root.
func lockfilePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".tag", LockFileName)
}

// Load reads .tag/lock.json from projectRoot. If the file does not exist a new,
// empty File is returned without error.
func Load(projectRoot string) (*File, error) {
	path := lockfilePath(projectRoot)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &File{Version: 1, Templates: make(map[string]*Entry)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read lockfile: %w", err)
	}
	var lf File
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parse lockfile: %w", err)
	}
	if lf.Templates == nil {
		lf.Templates = make(map[string]*Entry)
	}
	return &lf, nil
}

// Save persists the lockfile to .tag/lock.json in projectRoot, creating
// intermediate directories as needed.
func Save(projectRoot string, lf *File) error {
	dir := filepath.Join(projectRoot, ".tag")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create .tag/: %w", err)
	}
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal lockfile: %w", err)
	}
	path := lockfilePath(projectRoot)
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write lockfile: %w", err)
	}
	return nil
}

// VerifyAndMaybeUpdate checks whether templateDir's content hash matches the
// lockfile entry for ref, creating or updating the entry as needed.
//
//   - If opts.IgnoreLock is true: returns nil without verification (with a
//     warning printed to stderr).
//   - If no entry exists: creates one (first use).
//   - If an entry exists and opts.UpdateLock: refreshes it.
//   - If an entry exists and hashes differ: returns ErrChecksumMismatch.
//   - If opts.DryRun is true and the create/refresh branch above would fire,
//     the write is skipped (a notice is printed to stderr instead); a
//     mismatch is still reported as an error either way.
func VerifyAndMaybeUpdate(projectRoot, ref, templateDir string, opts VerifyOptions) error {
	if opts.IgnoreLock {
		fmt.Fprintf(os.Stderr, "warning: lockfile verification skipped for %s (--ignore-lock)\n", ref)
		return nil
	}

	computed, err := HashTemplateDir(templateDir)
	if err != nil {
		return fmt.Errorf("compute template hash: %w", err)
	}

	lf, err := Load(projectRoot)
	if err != nil {
		return err
	}

	existing, exists := lf.Templates[ref]

	switch {
	case !exists || opts.UpdateLock:
		// Check DryRun before the in-memory assignment below: the mutation
		// boundary is the write, not the decision, so a preview must not
		// touch lf.Templates at all.
		if opts.DryRun {
			fmt.Fprintf(os.Stderr, "(dry-run) would pin %s in .tag/lock.json\n", ref)
			return nil
		}
		// First use or forced refresh — pin the current hash.
		lf.Templates[ref] = &Entry{
			Ref:        ref,
			SHA256:     computed,
			ResolvedAt: time.Now().UTC(),
		}
		return Save(projectRoot, lf)

	case existing.SHA256 != computed:
		return fmt.Errorf("%w: %s\n  stored:   %s\n  computed: %s\n  To accept the new version run with --update-lock",
			ErrChecksumMismatch, ref, existing.SHA256, computed)
	}

	return nil
}

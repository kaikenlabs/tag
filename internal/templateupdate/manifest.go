package templateupdate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileAction describes what happened to a file during an update.
type FileAction string

const (
	// FileModified means the file existed and was changed (back up, restore on rollback).
	FileModified FileAction = "modified"
	// FileDeleted means the file was removed by the merge (back up, restore on rollback).
	FileDeleted FileAction = "deleted"
	// FileAdded means the file was newly created by the merge (remove on rollback).
	FileAdded FileAction = "added"
)

// ManifestEntry records a single file's role in a backup.
type ManifestEntry struct {
	Path   string     `json:"path"`
	Action FileAction `json:"action"`
}

// BackupManifest records metadata about a backup for reliable rollback.
type BackupManifest struct {
	CreatedAt  time.Time       `json:"created_at"`
	FromCommit string          `json:"from_commit"`
	ToCommit   string          `json:"to_commit"`
	Files      []ManifestEntry `json:"files"`
}

const manifestFileName = "manifest.json"

// WriteManifest writes the manifest to the backup directory.
func WriteManifest(backupPath string, manifest *BackupManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	return os.WriteFile(filepath.Join(backupPath, manifestFileName), append(data, '\n'), 0o600)
}

// ReadManifest reads the manifest from a backup directory.
// Returns nil, nil if no manifest exists (legacy backup).
func ReadManifest(backupPath string) (*BackupManifest, error) {
	data, err := os.ReadFile(filepath.Join(backupPath, manifestFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // nil,nil is the documented API for "not found"
		}

		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest BackupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	return &manifest, nil
}

// BuildManifestEntries categorises merge results into manifest entries.
func BuildManifestEntries(results []MergeResult) []ManifestEntry {
	var entries []ManifestEntry

	for _, r := range results {
		switch r.Op {
		case MergeUpdate, MergeConflict:
			entries = append(entries, ManifestEntry{Path: r.Path, Action: FileModified})
		case MergeDelete:
			entries = append(entries, ManifestEntry{Path: r.Path, Action: FileDeleted})
		case MergeAdd:
			entries = append(entries, ManifestEntry{Path: r.Path, Action: FileAdded})
		case MergeKeep, MergeUserAdded, MergePrompt:
			// No backup action needed for these ops.
		}
	}

	return entries
}

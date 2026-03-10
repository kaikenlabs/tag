package templateupdate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/kaikenlabs/tag/internal/types"
)

const backupDir = ".tag"

// CreateBackup copies the specified files from projectDir into
// .tag/backup/{timestamp}/ for restoration on --abort.
// Returns the backup directory path.
func CreateBackup(projectDir string, filePaths []string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(projectDir, backupDir, "backup", timestamp)

	if err := os.MkdirAll(backupPath, types.DirMode); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}

	for _, relPath := range filePaths {
		srcPath := filepath.Join(projectDir, relPath)
		dstPath := filepath.Join(backupPath, relPath)

		// Skip files that don't exist (they're new files from template).
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}

		// Create parent directory in backup.
		if err := os.MkdirAll(filepath.Dir(dstPath), types.DirMode); err != nil {
			return "", fmt.Errorf("create backup dir for %s: %w", relPath, err)
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return "", fmt.Errorf("backup %s: %w", relPath, err)
		}
	}

	return backupPath, nil
}

// RestoreBackup restores files from a backup directory to the project.
func RestoreBackup(projectDir, backupPath string) error {
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup not found at %s", backupPath)
	}

	return filepath.WalkDir(backupPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(backupPath, path)
		if err != nil {
			return fmt.Errorf("get relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		dstPath := filepath.Join(projectDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, types.DirMode)
		}

		return copyFile(path, dstPath)
	})
}

// FindLatestBackup returns the path of the most recent backup directory,
// or empty string if none exists.
func FindLatestBackup(projectDir string) (string, error) {
	backupsDir := filepath.Join(projectDir, backupDir, "backup")
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read backup directory: %w", err)
	}

	if len(entries) == 0 {
		return "", nil
	}

	// Entries are sorted lexically; timestamps sort correctly.
	latest := entries[len(entries)-1]
	return filepath.Join(backupsDir, latest.Name()), nil
}

// RemoveBackup removes a backup directory.
func RemoveBackup(backupPath string) error {
	return os.RemoveAll(backupPath)
}

// CreateBackupFromResults creates a manifest-aware backup from merge results.
// Files being modified or deleted are copied to the backup directory; files
// being added are tracked in the manifest so they can be removed on rollback.
func CreateBackupFromResults(projectDir string, results []MergeResult, fromCommit, toCommit string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(projectDir, backupDir, "backup", timestamp)

	if err := os.MkdirAll(backupPath, types.DirMode); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}

	entries := BuildManifestEntries(results)

	// Copy existing files that will be modified or deleted.
	for _, entry := range entries {
		if entry.Action == FileAdded {
			continue // New files don't exist yet — nothing to copy.
		}

		if err := backupFile(projectDir, backupPath, entry.Path); err != nil {
			_ = os.RemoveAll(backupPath)
			return "", err
		}
	}

	manifest := &BackupManifest{
		CreatedAt:  time.Now(),
		FromCommit: fromCommit,
		ToCommit:   toCommit,
		Files:      entries,
	}

	if err := WriteManifest(backupPath, manifest); err != nil {
		_ = os.RemoveAll(backupPath)
		return "", fmt.Errorf("write manifest: %w", err)
	}

	return backupPath, nil
}

// RestoreFromManifest restores a project using the manifest for precise rollback.
// If no manifest exists (legacy backup), falls back to RestoreBackup.
func RestoreFromManifest(projectDir, backupPath string) error {
	manifest, err := ReadManifest(backupPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	if manifest == nil {
		return RestoreBackup(projectDir, backupPath)
	}

	for _, entry := range manifest.Files {
		filePath := filepath.Join(projectDir, entry.Path)

		switch entry.Action {
		case FileModified, FileDeleted:
			src := filepath.Join(backupPath, entry.Path)

			if err := os.MkdirAll(filepath.Dir(filePath), types.DirMode); err != nil {
				return fmt.Errorf("create dir for %s: %w", entry.Path, err)
			}

			if err := copyFile(src, filePath); err != nil {
				return fmt.Errorf("restore %s: %w", entry.Path, err)
			}
		case FileAdded:
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove added file %s: %w", entry.Path, err)
			}
		}
	}

	return nil
}

// defaultBackupMaxAge is the retention period for old backups.
const defaultBackupMaxAge = 30 * 24 * time.Hour

// CleanOldBackups removes backup directories older than maxAge.
func CleanOldBackups(projectDir string, maxAge time.Duration) error {
	backupsDir := filepath.Join(projectDir, backupDir, "backup")

	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("read backup directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		t, parseErr := time.Parse("20060102-150405", entry.Name())
		if parseErr != nil {
			continue // Skip non-timestamp directories.
		}

		if t.Before(cutoff) {
			path := filepath.Join(backupsDir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("remove old backup %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// backupFile copies a single file from the project into the backup directory.
func backupFile(projectDir, backupPath, relPath string) error {
	srcPath := filepath.Join(projectDir, relPath)
	dstPath := filepath.Join(backupPath, relPath)

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return nil // File doesn't exist — safe to skip.
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), types.DirMode); err != nil {
		return fmt.Errorf("create backup dir for %s: %w", relPath, err)
	}

	if err := copyFile(srcPath, dstPath); err != nil {
		return fmt.Errorf("backup %s: %w", relPath, err)
	}

	return nil
}

// copyFile copies src to dst preserving file mode.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, info.Mode())
}

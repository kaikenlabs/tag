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

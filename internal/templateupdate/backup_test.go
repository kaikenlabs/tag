package templateupdate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Backup_CreateAndRestore(t *testing.T) {
	dir := t.TempDir()

	// Create files to back up.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("original1"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "file2.txt"), []byte("original2"), 0o644))

	// Create backup.
	backupPath, err := CreateBackup(dir, []string{"file1.txt", "sub/file2.txt"})
	require.NoError(t, err)
	assert.NotEmpty(t, backupPath)

	// Modify original files.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("modified1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "file2.txt"), []byte("modified2"), 0o644))

	// Restore.
	require.NoError(t, RestoreBackup(dir, backupPath))

	// Verify restoration.
	content1, err := os.ReadFile(filepath.Join(dir, "file1.txt"))
	require.NoError(t, err)
	assert.Equal(t, "original1", string(content1))

	content2, err := os.ReadFile(filepath.Join(dir, "sub", "file2.txt"))
	require.NoError(t, err)
	assert.Equal(t, "original2", string(content2))
}

func TestUT_Backup_SkipsNonexistentFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("data"), 0o644))

	backupPath, err := CreateBackup(dir, []string{"existing.txt", "nonexistent.txt"})
	require.NoError(t, err)

	// Only existing file should be backed up.
	_, err = os.Stat(filepath.Join(backupPath, "existing.txt"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(backupPath, "nonexistent.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Backup_Restore_MissingBackup(t *testing.T) {
	dir := t.TempDir()
	err := RestoreBackup(dir, filepath.Join(dir, "nonexistent-backup"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup not found")
}

func TestUT_Backup_FindLatestBackup(t *testing.T) {
	dir := t.TempDir()

	// No backups yet.
	path, err := FindLatestBackup(dir)
	require.NoError(t, err)
	assert.Empty(t, path)

	// Create backup directories.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag", "backup", "20260101-100000"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag", "backup", "20260201-100000"), 0o755))

	path, err = FindLatestBackup(dir)
	require.NoError(t, err)
	assert.Contains(t, path, "20260201-100000")
}

func TestUT_Backup_RemoveBackup(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	require.NoError(t, os.MkdirAll(backupDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(backupDir, "file.txt"), []byte("x"), 0o644))

	require.NoError(t, RemoveBackup(backupDir))
	_, err := os.Stat(backupDir)
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Backup_CreateFromResults_AndRestoreFromManifest(t *testing.T) {
	dir := t.TempDir()

	// Set up existing project files.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "modified.txt"), []byte("original"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "deleted.txt"), []byte("will-delete"), 0o644))

	results := []MergeResult{
		{Path: "modified.txt", Op: MergeUpdate, Content: []byte("new content")},
		{Path: "deleted.txt", Op: MergeDelete},
		{Path: "added.txt", Op: MergeAdd, Content: []byte("brand new")},
		{Path: "kept.txt", Op: MergeKeep},
	}

	// Create backup.
	backupPath, err := CreateBackupFromResults(dir, results, "old123", "new456")
	require.NoError(t, err)
	assert.NotEmpty(t, backupPath)

	// Verify manifest was written.
	manifest, err := ReadManifest(backupPath)
	require.NoError(t, err)
	require.NotNil(t, manifest)
	assert.Equal(t, "old123", manifest.FromCommit)
	assert.Equal(t, "new456", manifest.ToCommit)
	assert.Len(t, manifest.Files, 3) // modified, deleted, added (not kept)

	// Simulate the update: modify files, delete one, add one.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "modified.txt"), []byte("new content"), 0o644))
	require.NoError(t, os.Remove(filepath.Join(dir, "deleted.txt")))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "added.txt"), []byte("brand new"), 0o644))

	// Restore from manifest.
	require.NoError(t, RestoreFromManifest(dir, backupPath))

	// Verify: modified file restored.
	content, err := os.ReadFile(filepath.Join(dir, "modified.txt"))
	require.NoError(t, err)
	assert.Equal(t, "original", string(content))

	// Verify: deleted file restored.
	content, err = os.ReadFile(filepath.Join(dir, "deleted.txt"))
	require.NoError(t, err)
	assert.Equal(t, "will-delete", string(content))

	// Verify: added file removed.
	_, err = os.Stat(filepath.Join(dir, "added.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Backup_RestoreFromManifest_LegacyFallback(t *testing.T) {
	dir := t.TempDir()

	// Create a legacy backup (no manifest).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("original"), 0o644))
	backupPath, err := CreateBackup(dir, []string{"file.txt"})
	require.NoError(t, err)

	// Modify the file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("changed"), 0o644))

	// RestoreFromManifest falls back to legacy restore.
	require.NoError(t, RestoreFromManifest(dir, backupPath))

	content, err := os.ReadFile(filepath.Join(dir, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "original", string(content))
}

func TestUT_Backup_CleanOldBackups(t *testing.T) {
	dir := t.TempDir()
	backupsDir := filepath.Join(dir, ".tag", "backup")

	// Create old and recent backup directories.
	require.NoError(t, os.MkdirAll(filepath.Join(backupsDir, "20240101-100000"), 0o755)) // Old
	require.NoError(t, os.MkdirAll(filepath.Join(backupsDir, "20260301-100000"), 0o755)) // Recent

	require.NoError(t, CleanOldBackups(dir, 30*24*time.Hour))

	// Old backup removed.
	_, err := os.Stat(filepath.Join(backupsDir, "20240101-100000"))
	assert.True(t, os.IsNotExist(err))

	// Recent backup kept.
	_, err = os.Stat(filepath.Join(backupsDir, "20260301-100000"))
	assert.NoError(t, err)
}

func TestUT_Backup_CleanOldBackups_NoDir(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, CleanOldBackups(dir, 30*24*time.Hour))
}

func TestUT_Backup_CreateFromResults_AddedOnly(t *testing.T) {
	dir := t.TempDir()

	results := []MergeResult{
		{Path: "added.txt", Op: MergeAdd},
	}

	backupPath, err := CreateBackupFromResults(dir, results, "a", "b")
	require.NoError(t, err)
	assert.NotEmpty(t, backupPath)
}

func TestUT_Backup_RestoreFromManifest_PathTraversal(t *testing.T) {
	dir := t.TempDir()

	// Create a backup directory with a tampered manifest containing path traversal.
	backupPath := filepath.Join(dir, ".tag", "backup", "20260310-120000")
	require.NoError(t, os.MkdirAll(backupPath, 0o755))

	manifest := &BackupManifest{
		Files: []ManifestEntry{
			{Path: "../../etc/evil.txt", Action: FileAdded},
		},
	}
	require.NoError(t, WriteManifest(backupPath, manifest))

	err := RestoreFromManifest(dir, backupPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestUT_Backup_BackupFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	backupPath := filepath.Join(dir, "backup")
	require.NoError(t, os.MkdirAll(backupPath, 0o755))

	err := backupFile(dir, backupPath, "../../etc/passwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

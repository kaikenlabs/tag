package templateupdate

import (
	"os"
	"path/filepath"
	"testing"

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

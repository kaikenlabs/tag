package templateupdate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// backup.go — coverage for validateContainedPath, backupFile,
// CleanOldBackups edge cases, RestoreBackup WalkDir, CreateBackup
// ===========================================================================

func TestUT_ValidateContainedPath_Valid(t *testing.T) {
	t.Parallel()

	err := validateContainedPath("/project", "src/main.go")
	assert.NoError(t, err)
}

func TestUT_ValidateContainedPath_DotPath(t *testing.T) {
	t.Parallel()

	err := validateContainedPath("/project", ".")
	assert.NoError(t, err)
}

func TestUT_ValidateContainedPath_Traversal(t *testing.T) {
	t.Parallel()

	err := validateContainedPath("/project", "../../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestUT_ValidateContainedPath_NestedTraversal(t *testing.T) {
	t.Parallel()

	err := validateContainedPath("/project", "sub/../../../etc/shadow")
	assert.Error(t, err)
}

func TestUT_BackupFile_NonexistentSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	require.NoError(t, os.MkdirAll(backupDir, 0o755))

	// File doesn't exist — should be silently skipped
	err := backupFile(dir, backupDir, "nonexistent.txt")
	assert.NoError(t, err)
}

func TestUT_BackupFile_ExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	require.NoError(t, os.MkdirAll(backupDir, 0o755))

	// Create a file to back up
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0o644))

	err := backupFile(dir, backupDir, "file.txt")
	require.NoError(t, err)

	// Verify backup copy exists
	data, err := os.ReadFile(filepath.Join(backupDir, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(data))
}

func TestUT_BackupFile_NestedFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	require.NoError(t, os.MkdirAll(backupDir, 0o755))

	// Create nested file
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "deep", "file.txt"), []byte("deep"), 0o644))

	err := backupFile(dir, backupDir, "sub/deep/file.txt")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(backupDir, "sub", "deep", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "deep", string(data))
}

func TestUT_BackupFile_PathTraversal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	require.NoError(t, os.MkdirAll(backupDir, 0o755))

	err := backupFile(dir, backupDir, "../../etc/shadow")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestUT_CleanOldBackups_SkipsNonTimestampDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backupsDir := filepath.Join(dir, ".tag", "backup")
	require.NoError(t, os.MkdirAll(filepath.Join(backupsDir, "not-a-timestamp"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(backupsDir, "20260301-100000"), 0o755))

	err := CleanOldBackups(dir, 30*24*time.Hour)
	require.NoError(t, err)

	// not-a-timestamp should still exist (skipped)
	_, err = os.Stat(filepath.Join(backupsDir, "not-a-timestamp"))
	assert.NoError(t, err)

	// Recent backup should still exist
	_, err = os.Stat(filepath.Join(backupsDir, "20260301-100000"))
	assert.NoError(t, err)
}

func TestUT_CleanOldBackups_SkipsFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backupsDir := filepath.Join(dir, ".tag", "backup")
	require.NoError(t, os.MkdirAll(backupsDir, 0o755))

	// Create a file (not a directory) — should be skipped
	require.NoError(t, os.WriteFile(filepath.Join(backupsDir, "20240101-100000"), []byte("x"), 0o644))

	err := CleanOldBackups(dir, 30*24*time.Hour)
	require.NoError(t, err)

	// File should still exist
	_, err = os.Stat(filepath.Join(backupsDir, "20240101-100000"))
	assert.NoError(t, err)
}

func TestUT_CreateBackup_EmptyList(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backupPath, err := CreateBackup(dir, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, backupPath)

	// Backup directory should exist even with no files
	_, err = os.Stat(backupPath)
	assert.NoError(t, err)
}

func TestUT_CreateBackup_NestedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "b", "c.txt"), []byte("nested"), 0o644))

	backupPath, err := CreateBackup(dir, []string{"a/b/c.txt"})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(backupPath, "a", "b", "c.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nested", string(data))
}

func TestUT_RestoreBackup_EmptyBackup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	require.NoError(t, os.MkdirAll(backupDir, 0o755))

	// Empty backup — should succeed
	err := RestoreBackup(dir, backupDir)
	assert.NoError(t, err)
}

func TestUT_RestoreBackup_NonExistent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := RestoreBackup(dir, filepath.Join(dir, "nonexistent"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup not found")
}

func TestUT_FindLatestBackup_NoBackupDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path, err := FindLatestBackup(dir)
	require.NoError(t, err)
	assert.Empty(t, path)
}

func TestUT_FindLatestBackup_EmptyBackupDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tag", "backup"), 0o755))

	path, err := FindLatestBackup(dir)
	require.NoError(t, err)
	assert.Empty(t, path)
}

func TestUT_FindLatestBackup_MultipleBackups(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	backupsDir := filepath.Join(dir, ".tag", "backup")
	require.NoError(t, os.MkdirAll(filepath.Join(backupsDir, "20260101-100000"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(backupsDir, "20260301-100000"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(backupsDir, "20260201-100000"), 0o755))

	path, err := FindLatestBackup(dir)
	require.NoError(t, err)
	assert.Contains(t, path, "20260301-100000")
}

func TestUT_RemoveBackup_NonExistent(t *testing.T) {
	t.Parallel()

	err := RemoveBackup(filepath.Join(t.TempDir(), "nonexistent"))
	assert.NoError(t, err) // os.RemoveAll returns nil for nonexistent
}

func TestUT_CreateBackupFromResults_ModifiedAndDeleted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mod.txt"), []byte("original"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "del.txt"), []byte("to-delete"), 0o644))

	results := []MergeResult{
		{Path: "mod.txt", Op: MergeUpdate, Content: []byte("new")},
		{Path: "del.txt", Op: MergeDelete},
	}

	backupPath, err := CreateBackupFromResults(dir, results, "sha1", "sha2")
	require.NoError(t, err)

	// Verify backed up files
	data, err := os.ReadFile(filepath.Join(backupPath, "mod.txt"))
	require.NoError(t, err)
	assert.Equal(t, "original", string(data))

	data, err = os.ReadFile(filepath.Join(backupPath, "del.txt"))
	require.NoError(t, err)
	assert.Equal(t, "to-delete", string(data))
}

func TestUT_RestoreFromManifest_AddedFileRemoved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create backup with manifest tracking an added file
	backupPath := filepath.Join(dir, ".tag", "backup", "20260310-120000")
	require.NoError(t, os.MkdirAll(backupPath, 0o755))

	manifest := &BackupManifest{
		Files: []ManifestEntry{
			{Path: "new-file.txt", Action: FileAdded},
		},
	}
	require.NoError(t, WriteManifest(backupPath, manifest))

	// Create the file that was "added" by the update
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new-file.txt"), []byte("added"), 0o644))

	// Restore should remove the added file
	err := RestoreFromManifest(dir, backupPath)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, "new-file.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_RestoreFromManifest_NoManifest_LegacyFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("original"), 0o644))

	// Create backup without manifest (legacy)
	backupPath, err := CreateBackup(dir, []string{"file.txt"})
	require.NoError(t, err)

	// Modify the file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("changed"), 0o644))

	// Restore from manifest should fall back to legacy
	require.NoError(t, RestoreFromManifest(dir, backupPath))

	data, err := os.ReadFile(filepath.Join(dir, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "original", string(data))
}

// ===========================================================================
// manifest.go — coverage for BuildManifestEntries, WriteManifest, ReadManifest
// ===========================================================================

func TestUT_BuildManifestEntries_AllOps(t *testing.T) {
	t.Parallel()

	results := []MergeResult{
		{Path: "updated.go", Op: MergeUpdate},
		{Path: "conflicted.go", Op: MergeConflict},
		{Path: "deleted.go", Op: MergeDelete},
		{Path: "added.go", Op: MergeAdd},
		{Path: "kept.go", Op: MergeKeep},
		{Path: "user.go", Op: MergeUserAdded},
	}

	entries := BuildManifestEntries(results)
	require.Len(t, entries, 4) // update, conflict, delete, add

	actions := make(map[string]FileAction)
	for _, e := range entries {
		actions[e.Path] = e.Action
	}

	assert.Equal(t, FileModified, actions["updated.go"])
	assert.Equal(t, FileModified, actions["conflicted.go"])
	assert.Equal(t, FileDeleted, actions["deleted.go"])
	assert.Equal(t, FileAdded, actions["added.go"])
}

func TestUT_BuildManifestEntries_NilInput(t *testing.T) {
	t.Parallel()

	entries := BuildManifestEntries(nil)
	assert.Empty(t, entries)
}

func TestUT_WriteAndReadManifest_Roundtrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	manifest := &BackupManifest{
		CreatedAt:  time.Now().Truncate(time.Second),
		FromCommit: "abc123",
		ToCommit:   "def456",
		Files: []ManifestEntry{
			{Path: "file.go", Action: FileModified},
		},
	}

	err := WriteManifest(dir, manifest)
	require.NoError(t, err)

	read, err := ReadManifest(dir)
	require.NoError(t, err)
	require.NotNil(t, read)

	assert.Equal(t, "abc123", read.FromCommit)
	assert.Equal(t, "def456", read.ToCommit)
	require.Len(t, read.Files, 1)
	assert.Equal(t, "file.go", read.Files[0].Path)
	assert.Equal(t, FileModified, read.Files[0].Action)
}

func TestUT_ReadManifest_NotFound_ReturnsNil(t *testing.T) {
	t.Parallel()

	m, err := ReadManifest(t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, m)
}

func TestUT_ReadManifest_InvalidJSON_Error(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, manifestFileName), []byte("{invalid"), 0o644))

	_, err := ReadManifest(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse manifest")
}

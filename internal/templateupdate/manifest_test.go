package templateupdate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_WriteAndReadManifest(t *testing.T) {
	dir := t.TempDir()

	manifest := &BackupManifest{
		CreatedAt:  time.Date(2026, 3, 9, 14, 30, 22, 0, time.UTC),
		FromCommit: "abc1234",
		ToCommit:   "def5678",
		Files: []ManifestEntry{
			{Path: "Makefile", Action: FileModified},
			{Path: "go.mod", Action: FileModified},
			{Path: "old-file.go", Action: FileDeleted},
			{Path: "new-file.go", Action: FileAdded},
		},
	}

	require.NoError(t, WriteManifest(dir, manifest))

	got, err := ReadManifest(dir)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "abc1234", got.FromCommit)
	assert.Equal(t, "def5678", got.ToCommit)
	assert.Len(t, got.Files, 4)
	assert.Equal(t, FileModified, got.Files[0].Action)
	assert.Equal(t, FileDeleted, got.Files[2].Action)
	assert.Equal(t, FileAdded, got.Files[3].Action)
}

func TestUT_ReadManifest_NotFound(t *testing.T) {
	dir := t.TempDir()

	got, err := ReadManifest(dir)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUT_ReadManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, manifestFileName), []byte("{bad json"), 0o644))

	_, err := ReadManifest(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse manifest")
}

func TestUT_BuildManifestEntries(t *testing.T) {
	results := []MergeResult{
		{Path: "updated.txt", Op: MergeUpdate},
		{Path: "conflict.txt", Op: MergeConflict},
		{Path: "deleted.txt", Op: MergeDelete},
		{Path: "added.txt", Op: MergeAdd},
		{Path: "kept.txt", Op: MergeKeep},
		{Path: "user.txt", Op: MergeUserAdded},
		{Path: "prompt.txt", Op: MergePrompt},
	}

	entries := BuildManifestEntries(results)
	assert.Len(t, entries, 4)
	assert.Equal(t, ManifestEntry{Path: "updated.txt", Action: FileModified}, entries[0])
	assert.Equal(t, ManifestEntry{Path: "conflict.txt", Action: FileModified}, entries[1])
	assert.Equal(t, ManifestEntry{Path: "deleted.txt", Action: FileDeleted}, entries[2])
	assert.Equal(t, ManifestEntry{Path: "added.txt", Action: FileAdded}, entries[3])
}

func TestUT_BuildManifestEntries_Empty(t *testing.T) {
	results := []MergeResult{
		{Path: "kept.txt", Op: MergeKeep},
	}

	entries := BuildManifestEntries(results)
	assert.Empty(t, entries)
}

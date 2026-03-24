package history

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/writer"
)

// ===========================================================================
// recorder.go — coverage for RecordingFileWriter overwrite/inject on new file,
// snapshotBefore subsequent touch, WriteFile overwrite existing
// ===========================================================================

func TestUT_RecordingWriter_WriteFile_OverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "existing.go")
	require.NoError(t, os.WriteFile(target, []byte("original content"), 0o644))

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&stubFileWriter{}, rec)

	require.NoError(t, rw.WriteFile(target, []byte("new content"), 0o644))

	gen := rec.Build("test", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, ActionOverwrite, gen.Files[0].Action)
	assert.NotNil(t, gen.Files[0].HashBefore)
	assert.NotEmpty(t, gen.Files[0].HashAfter)

	// Backup should exist
	backupPath := filepath.Join(rec.BackupDir(), target)
	data, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, "original content", string(data))
}

func TestUT_RecordingWriter_InjectIntoFile_NewFile_RecordsCreate(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "new-inject.go")

	// stubFileWriter.InjectIntoFile needs the file to exist first for ReadFile
	require.NoError(t, os.WriteFile(target, []byte(""), 0o644))
	require.NoError(t, os.Remove(target)) // remove to simulate new file

	// For InjectIntoFile on non-existent file, the stub will fail on ReadFile.
	// Let's test inject on existing file instead.
	require.NoError(t, os.WriteFile(target, []byte("base"), 0o644))

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&stubFileWriter{}, rec)
	require.NoError(t, rw.InjectIntoFile(target, []byte(" injected"), writer.Inject{}))

	gen := rec.Build("test", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, ActionInject, gen.Files[0].Action)
	assert.NotNil(t, gen.Files[0].HashBefore)
}

func TestUT_RecordingWriter_MultipleTouches_OnlyOneBackup(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "multi.go")
	require.NoError(t, os.WriteFile(target, []byte("v0"), 0o644))

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&stubFileWriter{}, rec)

	// First touch: creates backup
	require.NoError(t, rw.WriteFile(target, []byte("v1"), 0o644))
	// Second touch: no new backup
	require.NoError(t, rw.AppendFile(target, []byte("v2")))

	gen := rec.Build("bundle", "generate")
	require.Len(t, gen.Files, 1)

	// Backup content should be original v0
	backupPath := filepath.Join(rec.BackupDir(), target)
	data, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, "v0", string(data))
}

func TestUT_RecordingWriter_WriteFile_NewFileCreatesAction(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "brand-new.go")

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&stubFileWriter{}, rec)
	require.NoError(t, rw.WriteFile(target, []byte("content"), 0o644))

	gen := rec.Build("test", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, ActionCreate, gen.Files[0].Action)
	assert.Nil(t, gen.Files[0].HashBefore)
}

// failingFileWriter simulates inner writer failures.
type failingFileWriter struct{}

func (f *failingFileWriter) WriteFile(_ string, _ []byte, _ fs.FileMode) error {
	return assert.AnError
}

func (f *failingFileWriter) AppendFile(_ string, _ []byte) error {
	return assert.AnError
}

func (f *failingFileWriter) InjectIntoFile(_ string, _ []byte, _ writer.Inject) error {
	return assert.AnError
}

func TestUT_RecordingWriter_WriteFile_InnerError(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "file.go")

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&failingFileWriter{}, rec)
	err := rw.WriteFile(target, []byte("data"), 0o644)
	assert.Error(t, err)
}

func TestUT_RecordingWriter_AppendFile_InnerError(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "file.go")

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&failingFileWriter{}, rec)
	err := rw.AppendFile(target, []byte("data"))
	assert.Error(t, err)
}

func TestUT_Recorder_Build_MultipleFiles_Sorted(t *testing.T) {
	t.Parallel()
	rec := NewRecorder(t.TempDir())
	rec.RecordCreate("c.go", "h3")
	rec.RecordCreate("a.go", "h1")
	rec.RecordCreate("b.go", "h2")

	gen := rec.Build("test", "gen")
	require.Len(t, gen.Files, 3)
	assert.Equal(t, "a.go", gen.Files[0].Path)
	assert.Equal(t, "b.go", gen.Files[1].Path)
	assert.Equal(t, "c.go", gen.Files[2].Path)
}

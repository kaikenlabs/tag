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

// stubFileWriter is a minimal FileWriter backed by the real filesystem.
type stubFileWriter struct{}

func (s *stubFileWriter) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return os.WriteFile(name, data, perm)
}

func (s *stubFileWriter) AppendFile(name string, data []byte) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (s *stubFileWriter) InjectIntoFile(name string, data []byte, _ writer.Inject) error {
	existing, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	return os.WriteFile(name, append(existing, data...), 0o644)
}

func (s *stubFileWriter) MergeOpenAPIFile(_ string, _ []byte, _ writer.OpenAPIMergeOptions) (writer.OpenAPIMergeResult, error) {
	return writer.OpenAPIMergeResult{}, nil
}

func TestUT_Recorder_RecordCreate_NilHashBefore(t *testing.T) {
	rec := NewRecorder(t.TempDir())
	rec.RecordCreate("handler.go", "sha256:abc")

	gen := rec.Build("model", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, ActionCreate, gen.Files[0].Action)
	assert.Nil(t, gen.Files[0].HashBefore)
	assert.Equal(t, "sha256:abc", gen.Files[0].HashAfter)
}

func TestUT_Recorder_RecordModify_HasHashBefore(t *testing.T) {
	rec := NewRecorder(t.TempDir())
	rec.RecordModify("router.go", ActionInject, "sha256:before", "sha256:after")

	gen := rec.Build("model", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, ActionInject, gen.Files[0].Action)
	require.NotNil(t, gen.Files[0].HashBefore)
	assert.Equal(t, "sha256:before", *gen.Files[0].HashBefore)
	assert.Equal(t, "sha256:after", gen.Files[0].HashAfter)
}

func TestUT_Recorder_Build_ReturnsGeneration(t *testing.T) {
	rec := NewRecorder(t.TempDir())
	rec.RecordCreate("a.go", "sha256:aaa")
	rec.RecordCreate("b.go", "sha256:bbb")

	gen := rec.Build("crud", "generate")
	assert.Equal(t, "crud", gen.Template)
	assert.Equal(t, "generate", gen.Command)
	assert.NotEmpty(t, gen.ID)
	assert.False(t, gen.Timestamp.IsZero())
	assert.Len(t, gen.Files, 2)
}

func TestUT_RecordingWriter_WriteFile_RecordsCreate(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&stubFileWriter{}, rec)

	target := filepath.Join(dir, "handler.go")
	require.NoError(t, rw.WriteFile(target, []byte("package main"), 0o644))

	gen := rec.Build("model", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, ActionCreate, gen.Files[0].Action)
	assert.Equal(t, target, gen.Files[0].Path)
	assert.Nil(t, gen.Files[0].HashBefore)
	assert.NotEmpty(t, gen.Files[0].HashAfter)
}

func TestUT_RecordingWriter_AppendFile_RecordsModify_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	// Pre-create the target file.
	target := filepath.Join(dir, "router.go")
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))
	originalHash, err := HashFile(target)
	require.NoError(t, err)

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&stubFileWriter{}, rec)
	require.NoError(t, rw.AppendFile(target, []byte("\nappended")))

	gen := rec.Build("routes", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, ActionAppend, gen.Files[0].Action)
	require.NotNil(t, gen.Files[0].HashBefore)
	assert.Equal(t, originalHash, *gen.Files[0].HashBefore)

	// Backup should exist.
	backupPath := filepath.Join(rec.BackupDir(), target)
	_, statErr := os.Stat(backupPath)
	assert.NoError(t, statErr, "backup file should exist")
}

func TestUT_RecordingWriter_InjectIntoFile_RecordsModify_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "app.go")
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&stubFileWriter{}, rec)
	require.NoError(t, rw.InjectIntoFile(target, []byte("\ninjected"), writer.Inject{}))

	gen := rec.Build("routes", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, ActionInject, gen.Files[0].Action)
	assert.NotNil(t, gen.Files[0].HashBefore)
}

func TestUT_RecordingWriter_AppendFile_TargetNotExist_RecordsCreate(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")
	target := filepath.Join(dir, "new.go")

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&stubFileWriter{}, rec)
	require.NoError(t, rw.AppendFile(target, []byte("content")))

	gen := rec.Build("model", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, ActionCreate, gen.Files[0].Action)
	assert.Nil(t, gen.Files[0].HashBefore)
}

func TestUT_RecordingWriter_FirstTouchSemantics_SameFileTwice(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "router.go")
	require.NoError(t, os.WriteFile(target, []byte("v0"), 0o644))
	v0Hash, err := HashFile(target)
	require.NoError(t, err)

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&stubFileWriter{}, rec)

	// Two appends to the same file in one generation (simulates a bundle).
	require.NoError(t, rw.AppendFile(target, []byte("v1")))
	require.NoError(t, rw.AppendFile(target, []byte("v2")))

	gen := rec.Build("bundle", "generate")
	// Should have exactly one entry for the file.
	require.Len(t, gen.Files, 1)
	entry := gen.Files[0]
	// hash_before must be v0 (pre-generation state).
	require.NotNil(t, entry.HashBefore)
	assert.Equal(t, v0Hash, *entry.HashBefore)
	// hash_after must reflect the state after both appends.
	currentHash, err := HashFile(target)
	require.NoError(t, err)
	assert.Equal(t, currentHash, entry.HashAfter)
}

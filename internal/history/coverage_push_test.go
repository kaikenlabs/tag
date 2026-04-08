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
// recorder.go — InjectIntoFile on non-existent file records Create (line 199-201)
// ===========================================================================

// stubInjectNewFile creates the file when InjectIntoFile is called on a new file.
type stubInjectNewFile struct{}

func (s *stubInjectNewFile) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return os.WriteFile(name, data, perm)
}

func (s *stubInjectNewFile) AppendFile(name string, data []byte) error {
	return os.WriteFile(name, data, 0o644)
}

func (s *stubInjectNewFile) InjectIntoFile(name string, data []byte, _ writer.Inject) error {
	// If file doesn't exist, create it; otherwise append
	if _, err := os.Stat(name); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(filepath.Dir(name), 0o755); mkErr != nil {
			return mkErr
		}
		return os.WriteFile(name, data, 0o644)
	}
	existing, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	return os.WriteFile(name, append(existing, data...), 0o644)
}

func (s *stubInjectNewFile) MergeOpenAPIFile(_ string, _ []byte, _ writer.OpenAPIMergeOptions) (writer.OpenAPIMergeResult, error) {
	return writer.OpenAPIMergeResult{}, nil
}

func TestUT_RecordingWriter_InjectIntoFile_NonExistent_RecordsCreate(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "injected-new.go")

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&stubInjectNewFile{}, rec)
	require.NoError(t, rw.InjectIntoFile(target, []byte("injected content"), writer.Inject{}))

	gen := rec.Build("test", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, ActionCreate, gen.Files[0].Action)
	assert.Nil(t, gen.Files[0].HashBefore)
	assert.NotEmpty(t, gen.Files[0].HashAfter)
}

// ===========================================================================
// recorder.go — snapshotBefore stat error (line 215-217)
// ===========================================================================

func TestUT_RecordingWriter_SnapshotBefore_StatError(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	// Create a file, then make its parent directory unreadable
	subdir := filepath.Join(dir, "restricted")
	require.NoError(t, os.MkdirAll(subdir, 0o755))
	target := filepath.Join(subdir, "file.go")
	require.NoError(t, os.WriteFile(target, []byte("data"), 0o644))

	// Make parent unreadable so stat fails
	require.NoError(t, os.Chmod(subdir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(subdir, 0o755) })

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&stubInjectNewFile{}, rec)
	err := rw.WriteFile(target, []byte("new"), 0o644)
	assert.Error(t, err, "should fail due to stat error on restricted directory")
}

// ===========================================================================
// recorder.go — backupFile MkdirAll error (line 241-243)
// ===========================================================================

func TestUT_RecordingWriter_BackupFile_MkdirError(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")
	require.NoError(t, os.MkdirAll(tagDir, 0o755))

	target := filepath.Join(dir, "existing.go")
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))

	rec := NewRecorder(tagDir)
	// Block the backup path by creating a file where "history" directory needs to be.
	// BackupDir = tagDir + "history/backups/" + genID
	// Create a regular file at tagDir/history to block MkdirAll.
	require.NoError(t, os.WriteFile(filepath.Join(tagDir, "history"), []byte("block"), 0o644))

	rw := NewRecordingFileWriter(&stubInjectNewFile{}, rec)
	err := rw.WriteFile(target, []byte("new"), 0o644)
	assert.Error(t, err, "should fail when backup dir cannot be created")
}

// ===========================================================================
// recorder.go — Build sort edge case (lines 92: equal paths)
// ===========================================================================

func TestUT_Recorder_Build_SamePath_HandledCorrectly(t *testing.T) {
	t.Parallel()
	rec := NewRecorder(t.TempDir())
	rec.RecordCreate("same.go", "sha256:first")
	rec.RecordCreate("same.go", "sha256:second") // update

	gen := rec.Build("test", "gen")
	require.Len(t, gen.Files, 1)
	// Should have the updated hash
	assert.Equal(t, "sha256:second", gen.Files[0].HashAfter)
}

// ===========================================================================
// recorder.go — InjectIntoFile inner error (line 190-192)
// ===========================================================================

func TestUT_RecordingWriter_InjectIntoFile_InnerError(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "file.go")

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&failingFileWriter{}, rec)
	err := rw.InjectIntoFile(target, []byte("data"), writer.Inject{})
	assert.Error(t, err)
}

// ===========================================================================
// recorder.go — WriteFile existing + inner write error (line 137-139)
// ===========================================================================

func TestUT_RecordingWriter_WriteFile_ExistingFile_InnerError(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "existing.go")
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&failingFileWriter{}, rec)
	err := rw.WriteFile(target, []byte("new"), 0o644)
	assert.Error(t, err)
}

// ===========================================================================
// recorder.go — AppendFile existing + inner append error (line 162-164)
// ===========================================================================

func TestUT_RecordingWriter_AppendFile_ExistingFile_InnerError(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "existing.go")
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&failingFileWriter{}, rec)
	err := rw.AppendFile(target, []byte("appended"))
	assert.Error(t, err)
}

// ===========================================================================
// recorder.go — InjectIntoFile existing + inner inject error (line 186-188)
// ===========================================================================

func TestUT_RecordingWriter_InjectIntoFile_ExistingFile_InnerError(t *testing.T) {
	dir := t.TempDir()
	tagDir := filepath.Join(dir, ".tag")

	target := filepath.Join(dir, "existing.go")
	require.NoError(t, os.WriteFile(target, []byte("original"), 0o644))

	rec := NewRecorder(tagDir)
	rw := NewRecordingFileWriter(&failingFileWriter{}, rec)
	err := rw.InjectIntoFile(target, []byte("inject"), writer.Inject{})
	assert.Error(t, err)
}

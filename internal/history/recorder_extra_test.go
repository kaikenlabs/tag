package history

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Recorder_GenID_Format(t *testing.T) {
	t.Parallel()
	rec := NewRecorder(t.TempDir())
	genID := rec.GenID()

	assert.NotEmpty(t, genID)
	assert.Contains(t, genID, "gen_")
}

func TestUT_Recorder_BackupDir_ContainsGenID(t *testing.T) {
	t.Parallel()
	tagDir := t.TempDir()
	rec := NewRecorder(tagDir)

	backupDir := rec.BackupDir()
	assert.Contains(t, backupDir, rec.GenID())
	assert.Contains(t, backupDir, tagDir)
}

func TestUT_Recorder_RecordCreate_DuplicateUpdatesHashAfter(t *testing.T) {
	t.Parallel()
	rec := NewRecorder(t.TempDir())

	rec.RecordCreate("file.go", "sha256:first")
	rec.RecordCreate("file.go", "sha256:second")

	gen := rec.Build("test", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, "sha256:second", gen.Files[0].HashAfter)
	assert.Equal(t, ActionCreate, gen.Files[0].Action)
}

func TestUT_Recorder_RecordModify_DuplicateUpdatesHashAfter(t *testing.T) {
	t.Parallel()
	rec := NewRecorder(t.TempDir())

	rec.RecordModify("file.go", ActionInject, "sha256:before", "sha256:mid")
	rec.RecordModify("file.go", ActionAppend, "", "sha256:final")

	gen := rec.Build("test", "generate")
	require.Len(t, gen.Files, 1)
	assert.Equal(t, "sha256:final", gen.Files[0].HashAfter)
	assert.Equal(t, ActionAppend, gen.Files[0].Action)
	// hashBefore from first call is preserved
	require.NotNil(t, gen.Files[0].HashBefore)
	assert.Equal(t, "sha256:before", *gen.Files[0].HashBefore)
}

func TestUT_Recorder_Build_SortsByPath(t *testing.T) {
	t.Parallel()
	rec := NewRecorder(t.TempDir())

	rec.RecordCreate("z_file.go", "sha256:z")
	rec.RecordCreate("a_file.go", "sha256:a")
	rec.RecordCreate("m_file.go", "sha256:m")

	gen := rec.Build("test", "generate")
	require.Len(t, gen.Files, 3)
	assert.Equal(t, "a_file.go", gen.Files[0].Path)
	assert.Equal(t, "m_file.go", gen.Files[1].Path)
	assert.Equal(t, "z_file.go", gen.Files[2].Path)
}

func TestUT_Recorder_Build_EmptyRecorder(t *testing.T) {
	t.Parallel()
	rec := NewRecorder(t.TempDir())

	gen := rec.Build("empty", "scaffold")
	assert.Empty(t, gen.Files)
	assert.Equal(t, "empty", gen.Template)
	assert.Equal(t, "scaffold", gen.Command)
	assert.NotEmpty(t, gen.ID)
	assert.False(t, gen.Timestamp.IsZero())
}

func TestUT_NewGenID_Uniqueness(t *testing.T) {
	t.Parallel()
	ids := make(map[string]bool, 100)
	for range 100 {
		id := newGenID()
		ids[id] = true
	}
	// With crypto/rand + unix timestamp, all IDs should be unique
	assert.GreaterOrEqual(t, len(ids), 95, "expected mostly unique IDs")
}

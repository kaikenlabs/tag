package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Manifest_LoadNonExistent_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	m, err := Load(dir)
	require.NoError(t, err)
	assert.Empty(t, m.Generations)
}

func TestUT_Manifest_SaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	hb := "sha256:aabbcc"
	original := Manifest{
		Generations: []Generation{
			{
				ID:        "gen_1_abc",
				Timestamp: time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
				Template:  "model",
				Command:   "generate",
				Files: []FileEntry{
					{Path: "handler.go", Action: ActionCreate, HashBefore: nil, HashAfter: "sha256:deadbeef"},
					{Path: "router.go", Action: ActionInject, HashBefore: &hb, HashAfter: "sha256:cafebabe"},
				},
			},
		},
	}

	require.NoError(t, Save(dir, original))
	loaded, err := Load(dir)
	require.NoError(t, err)

	require.Len(t, loaded.Generations, 1)
	g := loaded.Generations[0]
	assert.Equal(t, "gen_1_abc", g.ID)
	assert.Equal(t, "model", g.Template)
	assert.Len(t, g.Files, 2)
	assert.Nil(t, g.Files[0].HashBefore)
	assert.NotNil(t, g.Files[1].HashBefore)
	assert.Equal(t, "sha256:aabbcc", *g.Files[1].HashBefore)
}

func TestUT_Manifest_AtomicWrite_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()

	first := Manifest{Generations: []Generation{{ID: "gen_1_aaa", Command: "generate"}}}
	require.NoError(t, Save(dir, first))

	second := Manifest{Generations: []Generation{{ID: "gen_2_bbb", Command: "generate"}}}
	require.NoError(t, Save(dir, second))

	loaded, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, loaded.Generations, 1)
	assert.Equal(t, "gen_2_bbb", loaded.Generations[0].ID)
}

func TestUT_Manifest_CorruptJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	require.NoError(t, os.WriteFile(path, []byte("{not valid json"), 0o644))

	_, err := Load(dir)
	assert.Error(t, err)
}

func TestUT_Manifest_Append_AddsGeneration(t *testing.T) {
	dir := t.TempDir()
	g1 := Generation{ID: "gen_1_aaa", Command: "generate"}
	g2 := Generation{ID: "gen_2_bbb", Command: "generate"}

	require.NoError(t, Append(dir, g1))
	require.NoError(t, Append(dir, g2))

	m, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, m.Generations, 2)
	assert.Equal(t, "gen_1_aaa", m.Generations[0].ID)
	assert.Equal(t, "gen_2_bbb", m.Generations[1].ID)
}

func TestUT_Manifest_Remove_RemovesGeneration(t *testing.T) {
	dir := t.TempDir()
	g1 := Generation{ID: "gen_1_aaa", Command: "generate"}
	g2 := Generation{ID: "gen_2_bbb", Command: "generate"}
	require.NoError(t, Append(dir, g1))
	require.NoError(t, Append(dir, g2))

	require.NoError(t, Remove(dir, "gen_1_aaa"))

	m, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, m.Generations, 1)
	assert.Equal(t, "gen_2_bbb", m.Generations[0].ID)
}

func TestUT_Manifest_Remove_UnknownID_ReturnsErrNotFound(t *testing.T) {
	dir := t.TempDir()
	err := Remove(dir, "gen_unknown")
	assert.ErrorIs(t, err, ErrNotFound)
}

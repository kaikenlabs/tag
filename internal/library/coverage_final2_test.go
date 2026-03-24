package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// registry.go — coverage for load with newer version, save MkdirAll error
// ===========================================================================

func TestUT_LoadRegistry_NewerVersion_ReturnsError(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	// Write a registry with a version higher than supported
	content := `{"version": 999, "entries": {}}`
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, registryFile), []byte(content), 0o600))

	_, err := newStore(dataDir).load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "newer than supported version")
}

func TestUT_LoadRegistry_ValidWithEntries(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	content := `{"version": 1, "entries": {"my-lib": {"name": "my-lib", "source": "gh:test/lib"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, registryFile), []byte(content), 0o600))

	reg, err := newStore(dataDir).load()
	require.NoError(t, err)
	require.Len(t, reg.Entries, 1)
	assert.Equal(t, "my-lib", reg.Entries["my-lib"].Name)
}

func TestUT_SaveLoad_EmptyRegistry(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	store := newStore(dataDir)

	reg := &Registry{Entries: make(map[string]*Entry)}
	require.NoError(t, store.save(reg))

	loaded, err := store.load()
	require.NoError(t, err)
	assert.Empty(t, loaded.Entries)
	assert.Equal(t, registryVersion, loaded.Version)
}

func TestUT_SaveRegistry_MultipleEntries(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	store := newStore(dataDir)

	now := time.Now().UTC().Truncate(time.Second)
	reg := &Registry{
		Entries: map[string]*Entry{
			"a": {Name: "a", Source: "gh:test/a", AddedAt: now},
			"b": {Name: "b", Source: "gl:test/b", AddedAt: now},
			"c": {Name: "c", Source: "bb:test/c", AddedAt: now},
		},
	}

	require.NoError(t, store.save(reg))
	loaded, err := store.load()
	require.NoError(t, err)
	assert.Len(t, loaded.Entries, 3)
}

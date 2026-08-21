package library

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_LoadRegistry_NonExistentFile(t *testing.T) {
	dataDir := t.TempDir()

	reg, err := newStore(dataDir).load()
	require.NoError(t, err)
	assert.NotNil(t, reg.Entries)
	assert.Empty(t, reg.Entries)
}

func TestUT_LoadRegistry_CorruptJSON(t *testing.T) {
	dataDir := t.TempDir()
	err := os.WriteFile(filepath.Join(dataDir, registryFile), []byte("{not valid json"), 0o600)
	require.NoError(t, err)

	_, err = newStore(dataDir).load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse registry")
}

func TestUT_LoadRegistry_NullEntries(t *testing.T) {
	// JSON with entries: null should still work
	dataDir := t.TempDir()
	err := os.WriteFile(filepath.Join(dataDir, registryFile), []byte(`{"entries": null}`), 0o600)
	require.NoError(t, err)

	reg, err := newStore(dataDir).load()
	require.NoError(t, err)
	assert.NotNil(t, reg.Entries)
	assert.Empty(t, reg.Entries)
}

func TestUT_SaveRegistry_CreatesDirectory(t *testing.T) {
	// dataDir doesn't exist yet — store.save should create it
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "nested", "path")

	reg := &Registry{Entries: map[string]*Entry{
		"test": {Name: "test", Source: "gh:user/test", AddedAt: time.Now()},
	}}

	err := newStore(dataDir).save(reg)
	require.NoError(t, err)

	// Verify file was written
	assert.FileExists(t, filepath.Join(dataDir, registryFile))
}

func TestUT_SaveLoad_RoundTrip(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second) // JSON loses sub-second on some systems

	original := &Registry{Entries: map[string]*Entry{
		"go-api": {
			Name:          "go-api",
			Source:        "gh:company/go-api",
			AddedAt:       now,
			UpdatedAt:     now,
			Version:       "1.0.0",
			Description:   "A Go API template",
			ConvertedFrom: "",
		},
		"django": {
			Name:          "django",
			Source:        "gh:user/cookiecutter-django",
			AddedAt:       now,
			UpdatedAt:     now,
			Version:       "",
			Description:   "Django project",
			ConvertedFrom: "cookiecutter",
		},
	}}

	store := newStore(dataDir)
	err := store.save(original)
	require.NoError(t, err)

	loaded, err := store.load()
	require.NoError(t, err)

	require.Len(t, loaded.Entries, 2)

	goAPI := loaded.Entries["go-api"]
	require.NotNil(t, goAPI)
	assert.Equal(t, "gh:company/go-api", goAPI.Source)
	assert.Equal(t, "1.0.0", goAPI.Version)
	assert.Equal(t, "A Go API template", goAPI.Description)

	django := loaded.Entries["django"]
	require.NotNil(t, django)
	assert.Equal(t, "cookiecutter", django.ConvertedFrom)
}

func TestUT_SaveRegistry_FilePermissions(t *testing.T) {
	dataDir := t.TempDir()
	reg := &Registry{Entries: make(map[string]*Entry)}

	err := newStore(dataDir).save(reg)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dataDir, registryFile))
	require.NoError(t, err)
	// File should be owner-only readable (0600)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestUT_SaveRegistry_ConcurrentWrites(t *testing.T) {
	dataDir := t.TempDir()
	store := newStore(dataDir)

	const numWriters = 4
	const rounds = 5

	for round := range rounds {
		var wg sync.WaitGroup
		errs := make([]error, numWriters)
		candidates := make([]*Registry, numWriters)
		for w := range numWriters {
			entryName := fmt.Sprintf("entry-%d-%d", round, w)
			reg := &Registry{
				Version: registryVersion,
				Entries: map[string]*Entry{
					entryName: {Name: entryName, Source: "local"},
				},
			}
			candidates[w] = reg
			wg.Go(func() {
				errs[w] = store.save(reg)
			})
		}
		wg.Wait()

		for _, err := range errs {
			require.NoError(t, err)
		}

		data, err := os.ReadFile(filepath.Join(dataDir, registryFile))
		require.NoError(t, err)

		var parsed Registry
		require.NoError(t, json.Unmarshal(data, &parsed), "round %d: final registry file must be valid JSON", round)

		matched := false
		for _, c := range candidates {
			if reflect.DeepEqual(parsed, *c) {
				matched = true
				break
			}
		}
		assert.True(t, matched, "round %d: surviving registry content must equal exactly one writer's payload, not a mix", round)
	}
}

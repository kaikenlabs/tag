package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_LoadRegistry_NonExistentFile(t *testing.T) {
	dataDir := t.TempDir()

	reg, err := loadRegistry(dataDir)
	require.NoError(t, err)
	assert.NotNil(t, reg.Entries)
	assert.Empty(t, reg.Entries)
}

func TestUT_LoadRegistry_CorruptJSON(t *testing.T) {
	dataDir := t.TempDir()
	err := os.WriteFile(filepath.Join(dataDir, registryFile), []byte("{not valid json"), 0o600)
	require.NoError(t, err)

	_, err = loadRegistry(dataDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse registry")
}

func TestUT_LoadRegistry_NullEntries(t *testing.T) {
	// JSON with entries: null should still work
	dataDir := t.TempDir()
	err := os.WriteFile(filepath.Join(dataDir, registryFile), []byte(`{"entries": null}`), 0o600)
	require.NoError(t, err)

	reg, err := loadRegistry(dataDir)
	require.NoError(t, err)
	assert.NotNil(t, reg.Entries)
	assert.Empty(t, reg.Entries)
}

func TestUT_SaveRegistry_CreatesDirectory(t *testing.T) {
	// dataDir doesn't exist yet — saveRegistry should create it
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "nested", "path")

	reg := &Registry{Entries: map[string]*Entry{
		"test": {Name: "test", Source: "gh:user/test", AddedAt: time.Now()},
	}}

	err := saveRegistry(dataDir, reg)
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

	err := saveRegistry(dataDir, original)
	require.NoError(t, err)

	loaded, err := loadRegistry(dataDir)
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

func TestUT_SaveRegistry_AtomicWrite(t *testing.T) {
	// Verify no .tmp file is left behind after successful save
	dataDir := t.TempDir()
	reg := &Registry{Entries: map[string]*Entry{
		"test": {Name: "test", Source: "local"},
	}}

	err := saveRegistry(dataDir, reg)
	require.NoError(t, err)

	tmpPath := filepath.Join(dataDir, registryFile+".tmp")
	_, err = os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), "temp file should not exist after successful save")
}

func TestUT_SaveRegistry_FilePermissions(t *testing.T) {
	dataDir := t.TempDir()
	reg := &Registry{Entries: make(map[string]*Entry)}

	err := saveRegistry(dataDir, reg)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dataDir, registryFile))
	require.NoError(t, err)
	// File should be owner-only readable (0600)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

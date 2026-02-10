package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_LoadGlobalConfig_MissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, &GlobalConfig{}, cfg)
}

func TestUT_SaveAndLoadGlobalConfig_RoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	original := &GlobalConfig{Editor: "code --wait"}
	require.NoError(t, SaveGlobalConfig(original))

	loaded, err := LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, original, loaded)
}

func TestUT_SaveGlobalConfig_CreatesDirectories(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	require.NoError(t, SaveGlobalConfig(&GlobalConfig{Editor: "vim"}))

	path := filepath.Join(tmp, "tag", "config.json")
	_, err := os.Stat(path)
	assert.NoError(t, err)
}

func TestUT_LoadGlobalConfig_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir := filepath.Join(tmp, "tag")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte("{invalid"), 0o644))

	_, err := LoadGlobalConfig()
	assert.Error(t, err)
}

func TestUT_LoadGlobalConfig_EmptyEditor(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	require.NoError(t, SaveGlobalConfig(&GlobalConfig{}))

	loaded, err := LoadGlobalConfig()
	require.NoError(t, err)
	assert.Empty(t, loaded.Editor)
}

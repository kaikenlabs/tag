package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// global.go — coverage for LoadGlobalConfig, SaveGlobalConfig, GlobalConfigPath
// ===========================================================================

func TestUT_GlobalConfigPath_ContainsConfigJSON(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := GlobalConfigPath()
	require.NoError(t, err)
	assert.Contains(t, path, "config.json")
	assert.Contains(t, path, "tag")
}

func TestUT_LoadGlobalConfig_NoFile_ReturnsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := LoadGlobalConfig()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.Editor)
}

func TestUT_LoadGlobalConfig_ValidFile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	tagDir := filepath.Join(configDir, "tag")
	require.NoError(t, os.MkdirAll(tagDir, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(tagDir, "config.json"),
		[]byte(`{"editor":"vim"}`),
		0o600,
	))

	cfg, err := LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "vim", cfg.Editor)
}

func TestUT_LoadGlobalConfig_MalformedJSON(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	tagDir := filepath.Join(configDir, "tag")
	require.NoError(t, os.MkdirAll(tagDir, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(tagDir, "config.json"),
		[]byte(`{bad json content`),
		0o600,
	))

	_, err := LoadGlobalConfig()
	assert.Error(t, err)
}

func TestUT_SaveGlobalConfig_CreatesFile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	cfg := &GlobalConfig{Editor: "code"}
	err := SaveGlobalConfig(cfg)
	require.NoError(t, err)

	// Verify file exists and is readable
	loaded, err := LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "code", loaded.Editor)
}

func TestUT_SaveGlobalConfig_OverwritesExisting(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	require.NoError(t, SaveGlobalConfig(&GlobalConfig{Editor: "vim"}))
	require.NoError(t, SaveGlobalConfig(&GlobalConfig{Editor: "nano"}))

	loaded, err := LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, "nano", loaded.Editor)
}

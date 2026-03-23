package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_GlobalConfigPath_ReturnsPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	path, err := GlobalConfigPath()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, "tag", "config.json"), path)
}

func TestUT_SaveGlobalConfig_PermissionsOnFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	require.NoError(t, SaveGlobalConfig(&GlobalConfig{Editor: "nano"}))

	path := filepath.Join(tmp, "tag", "config.json")
	info, err := os.Stat(path)
	require.NoError(t, err)
	// FileModePrivate is 0o600
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestUT_LoadGlobalConfig_RoundTrip_EmptyConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	original := &GlobalConfig{}
	require.NoError(t, SaveGlobalConfig(original))

	loaded, err := LoadGlobalConfig()
	require.NoError(t, err)
	assert.Equal(t, original, loaded)
}

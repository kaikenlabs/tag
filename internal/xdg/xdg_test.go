package xdg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_DataHome_Default(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	got, err := DataHome()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".local", "share", "tag"), got)
}

func TestUT_DataHome_CustomXDG(t *testing.T) {
	custom := t.TempDir()
	t.Setenv("XDG_DATA_HOME", custom)

	got, err := DataHome()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(custom, "tag"), got)
}

func TestUT_DataHome_RelativePathRejected(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "relative/path")

	_, err := DataHome()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "absolute path")
}

func TestUT_ConfigHome_Default(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	got, err := ConfigHome()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "tag"), got)
}

func TestUT_ConfigHome_CustomXDG(t *testing.T) {
	custom := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", custom)

	got, err := ConfigHome()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(custom, "tag"), got)
}

func TestUT_ConfigHome_RelativePathRejected(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "relative/path")

	_, err := ConfigHome()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "absolute path")
}

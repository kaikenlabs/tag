package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ReplayDir_Resolution(t *testing.T) {
	tests := []struct {
		name      string
		env       string
		envSet    bool
		home      string
		homeSet   bool
		wantErr   string
		wantEqual func(home string) string
	}{
		{
			name:      "env absolute wins",
			env:       "/env/absolute/replay",
			envSet:    true,
			wantEqual: func(string) string { return "/env/absolute/replay" },
		},
		{
			name:    "env relative errors naming TAG_REPLAY_DIR",
			env:     "relative/replay",
			envSet:  true,
			wantErr: "TAG_REPLAY_DIR",
		},
		{
			name:      "env empty string treated as unset falls back to home default",
			env:       "",
			envSet:    true,
			home:      "/home/user",
			homeSet:   true,
			wantEqual: func(home string) string { return filepath.Join(home, ".tag", DefaultReplayDir) },
		},
		{
			name:      "unset falls back to home default",
			home:      "/home/user",
			homeSet:   true,
			wantEqual: func(home string) string { return filepath.Join(home, ".tag", DefaultReplayDir) },
		},
		{
			name:      "env set and HOME empty still succeeds",
			env:       "/env/only/replay",
			envSet:    true,
			home:      "",
			homeSet:   true,
			wantEqual: func(string) string { return "/env/only/replay" },
		},
		{
			name:    "unset and HOME empty errors",
			home:    "",
			homeSet: true,
			wantErr: "home directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				t.Setenv("TAG_REPLAY_DIR", tt.env)
			} else {
				t.Setenv("TAG_REPLAY_DIR", "")
				require.NoError(t, os.Unsetenv("TAG_REPLAY_DIR"))
			}
			if tt.homeSet {
				t.Setenv("HOME", tt.home)
			}

			got, err := getReplayDir()

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantEqual(tt.home), got)
		})
	}
}

func TestUT_ReplayDir_SaveLoadRoundTripUnderEnvDir(t *testing.T) {
	replayDir := t.TempDir()
	t.Setenv("TAG_REPLAY_DIR", replayDir)
	t.Setenv("HOME", t.TempDir())

	err := Save("gh:user/repo", "v1.0.0", map[string]any{"name": "widget"}, nil)
	require.NoError(t, err)

	entries, err := os.ReadDir(replayDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, filepath.Ext(entries[0].Name()) == ".json")

	loaded, err := Load("gh:user/repo")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "gh:user/repo", loaded.Template)
	assert.Equal(t, "widget", loaded.Values["name"])
}

func TestUT_ReplayDir_SecretsFilteredUnderEnvDir(t *testing.T) {
	replayDir := t.TempDir()
	t.Setenv("TAG_REPLAY_DIR", replayDir)
	t.Setenv("HOME", t.TempDir())

	err := Save("gh:user/repo", "v1.0.0",
		map[string]any{"api_key": "super-secret-value", "name": "widget-name"},
		map[string]bool{"api_key": true},
	)
	require.NoError(t, err)

	entries, err := os.ReadDir(replayDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	raw, err := os.ReadFile(filepath.Join(replayDir, entries[0].Name()))
	require.NoError(t, err)

	assert.NotContains(t, string(raw), "super-secret-value")
	assert.Contains(t, string(raw), "widget-name")

	var parsed ReplayData
	require.NoError(t, json.Unmarshal(raw, &parsed))
	_, hasSecret := parsed.Values["api_key"]
	assert.False(t, hasSecret)
}

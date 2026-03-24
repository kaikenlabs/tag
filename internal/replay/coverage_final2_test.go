package replay

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// save.go — coverage for Save with renamed temp file, getReplayDir
// ===========================================================================

func TestUT_Save_EmptyTemplateSource(t *testing.T) {
	t.Parallel()
	err := Save("", "v1", map[string]any{"k": "v"}, nil)
	assert.ErrorIs(t, err, ErrEmptyTemplateSource)
}

func TestUT_Save_AtomicWrite(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	err := Save("gh:org/repo", "v2.0.0", map[string]any{"name": "test"}, nil)
	require.NoError(t, err)

	// Verify no temp file left behind
	replayDir := filepath.Join(tempHome, ".tag", "replay")
	entries, err := os.ReadDir(replayDir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, filepath.Ext(e.Name()) == ".tmp", "temp file should not remain: %s", e.Name())
	}
}

func TestUT_Save_WithSecrets_ExcludesFromDisk(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	values := map[string]any{"token": "secret123", "name": "myproject"}
	secrets := map[string]bool{"token": true}

	err := Save("gh:org/repo", "", values, secrets)
	require.NoError(t, err)

	loaded, err := Load("gh:org/repo")
	require.NoError(t, err)
	assert.NotContains(t, loaded.Values, "token")
	assert.Equal(t, "myproject", loaded.Values["name"])
}

// ===========================================================================
// load.go — coverage for Load with permission error, nil values initialization
// ===========================================================================

func TestUT_Load_EmptyTemplateSource(t *testing.T) {
	t.Parallel()
	_, err := Load("")
	assert.ErrorIs(t, err, ErrEmptyTemplateSource)
}

func TestUT_Load_ValidJSON_NilValues_InitializesMap(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	replayDir := filepath.Join(tempHome, ".tag", "replay")
	require.NoError(t, os.MkdirAll(replayDir, 0o700))

	// Write JSON with null values
	require.NoError(t, os.WriteFile(
		filepath.Join(replayDir, "gh_org_repo.json"),
		[]byte(`{"template":"gh:org/repo","values":null}`),
		0o600,
	))

	loaded, err := Load("gh:org/repo")
	require.NoError(t, err)
	assert.NotNil(t, loaded.Values)
	assert.Empty(t, loaded.Values)
}

// ===========================================================================
// replay_test.go (test-only helpers) — coverage for Delete, Exists, GetReplayFilePath
// ===========================================================================

func TestUT_Delete_WithExistingFile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	require.NoError(t, Save("bb:team/repo", "", map[string]any{"x": 1}, nil))
	assert.True(t, Exists("bb:team/repo"))

	require.NoError(t, Delete("bb:team/repo"))
	assert.False(t, Exists("bb:team/repo"))
}

func TestUT_Delete_EmptySource_Validation(t *testing.T) {
	t.Parallel()
	assert.ErrorIs(t, Delete(""), ErrEmptyTemplateSource)
}

func TestUT_Exists_EmptySource(t *testing.T) {
	t.Parallel()
	assert.False(t, Exists(""))
}

func TestUT_GetReplayFilePath_ValidSource(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	path, err := GetReplayFilePath("gl:user/repo")
	require.NoError(t, err)
	assert.Contains(t, path, "gl_user_repo.json")
}

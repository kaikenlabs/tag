package replay

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test-only helpers (moved from load.go — only used by tests)
// =============================================================================

// Exists checks if a replay file exists for the given template source.
func Exists(templateSource string) bool {
	templateSource = strings.TrimSpace(templateSource)
	if templateSource == "" {
		return false
	}

	templateID := GenerateTemplateID(templateSource)
	if templateID == "" {
		return false
	}

	filePath, err := getReplayFilePath(templateID)
	if err != nil {
		return false
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

// GetReplayFilePath returns the full path to the replay file for the given template source.
// This is useful for displaying to users or for cleanup operations.
func GetReplayFilePath(templateSource string) (string, error) {
	templateSource = strings.TrimSpace(templateSource)
	if templateSource == "" {
		return "", ErrEmptyTemplateSource
	}

	templateID := GenerateTemplateID(templateSource)
	if templateID == "" {
		return "", ErrEmptyTemplateSource
	}

	return getReplayFilePath(templateID)
}

// Delete removes the replay file for the given template source.
// Returns nil if the file doesn't exist.
func Delete(templateSource string) error {
	templateSource = strings.TrimSpace(templateSource)
	if templateSource == "" {
		return ErrEmptyTemplateSource
	}

	templateID := GenerateTemplateID(templateSource)
	if templateID == "" {
		return ErrEmptyTemplateSource
	}

	filePath, err := getReplayFilePath(templateID)
	if err != nil {
		return NewReplayError(templateID, "delete", err)
	}

	err = os.Remove(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return NewReplayError(templateID, "delete", err)
	}

	return nil
}

// =============================================================================
// TestUT_GenerateTemplateID - Template ID generation tests
// =============================================================================

func TestUT_GenerateTemplateID_GitHubShorthand(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected string
	}{
		{
			name:     "simple repo",
			source:   "gh:user/repo",
			expected: "gh_user_repo",
		},
		{
			name:     "with version",
			source:   "gh:user/repo@v1.0.0",
			expected: "gh_user_repo_v1_0_0", // dots sanitized to underscores
		},
		{
			name:     "with subpath",
			source:   "gh:user/repo/subdir",
			expected: "gh_user_repo_subdir",
		},
		{
			name:     "with version and subpath",
			source:   "gh:user/repo@v1.0.0/subdir/nested",
			expected: "gh_user_repo_v1_0_0_subdir_nested", // dots sanitized to underscores
		},
		{
			name:     "org name with hyphen",
			source:   "gh:my-org/my-repo",
			expected: "gh_my-org_my-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTemplateID(tt.source)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_GenerateTemplateID_GitLabShorthand(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected string
	}{
		{
			name:     "simple repo",
			source:   "gl:user/repo",
			expected: "gl_user_repo",
		},
		{
			name:     "with version",
			source:   "gl:user/repo@main",
			expected: "gl_user_repo_main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTemplateID(tt.source)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_GenerateTemplateID_BitbucketShorthand(t *testing.T) {
	result := GenerateTemplateID("bb:team/project")
	assert.Equal(t, "bb_team_project", result)
}

func TestUT_GenerateTemplateID_HTTPSUrl(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "github https",
			source: "https://github.com/user/repo.git",
		},
		{
			name:   "generic https",
			source: "https://example.com/template.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTemplateID(tt.source)
			assert.True(t, strings.HasPrefix(result, "url_"), "expected url_ prefix, got: %s", result)
			assert.Len(t, result, 16) // "url_" + 12 char hash
		})
	}
}

func TestUT_GenerateTemplateID_LocalPath(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "relative path",
			source: "./my-template",
		},
		{
			name:   "absolute path",
			source: "/home/user/templates/my-template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTemplateID(tt.source)
			assert.True(t, strings.HasPrefix(result, "local_"), "expected local_ prefix, got: %s", result)
			assert.Len(t, result, 18) // "local_" + 12 char hash
		})
	}
}

func TestUT_GenerateTemplateID_GitSSH(t *testing.T) {
	tests := []struct {
		name   string
		source string
		prefix string
	}{
		{
			name:   "git+ssh url",
			source: "git+ssh://git@github.com/user/repo.git",
			prefix: "git_",
		},
		{
			name:   "ssh style",
			source: "git@github.com:user/repo.git",
			prefix: "ssh_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTemplateID(tt.source)
			assert.True(t, strings.HasPrefix(result, tt.prefix), "expected %s prefix, got: %s", tt.prefix, result)
		})
	}
}

func TestUT_GenerateTemplateID_EmptySource(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "empty string", source: ""},
		{name: "only spaces", source: "   "},
		{name: "only tabs", source: "\t\t"},
		{name: "only newlines", source: "\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTemplateID(tt.source)
			assert.Equal(t, "", result)
		})
	}
}

func TestUT_GenerateTemplateID_SpecialCharacters(t *testing.T) {
	// IDs with special characters should be sanitized
	result := GenerateTemplateID("gh:user/repo@v1.0.0-beta.1")
	// Dots are sanitized to underscores for filesystem safety
	assert.Equal(t, "gh_user_repo_v1_0_0-beta_1", result)
	assert.NotContains(t, result, "//")
	assert.NotContains(t, result, ":")
	assert.NotContains(t, result, ".") // dots should be sanitized
}

func TestUT_GenerateTemplateID_Consistency(t *testing.T) {
	// Same source should always produce same ID
	source := "gh:user/my-awesome-template@v2.0.0"
	id1 := GenerateTemplateID(source)
	id2 := GenerateTemplateID(source)
	assert.Equal(t, id1, id2)
}

// =============================================================================
// TestUT_Save - Save replay data tests
// =============================================================================

func TestUT_Save_Success(t *testing.T) {
	// Create a temporary home directory
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	values := map[string]any{
		"project_name": "test-project",
		"author":       "Test Author",
		"use_docker":   true,
		"port":         8080.0,
	}

	err := Save("gh:user/repo", "v1.0.0", values, nil)
	require.NoError(t, err)

	// Verify file was created
	replayDir := filepath.Join(tempHome, ".tag", "replay")
	filePath := filepath.Join(replayDir, "gh_user_repo.json")
	assert.FileExists(t, filePath)

	// Verify content
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var saved ReplayData
	err = json.Unmarshal(data, &saved)
	require.NoError(t, err)

	assert.Equal(t, "gh:user/repo", saved.Template)
	assert.Equal(t, "v1.0.0", saved.Version)
	assert.Equal(t, "test-project", saved.Values["project_name"])
	assert.Equal(t, "Test Author", saved.Values["author"])
	assert.Equal(t, true, saved.Values["use_docker"])
	assert.Equal(t, 8080.0, saved.Values["port"])
	assert.False(t, saved.Timestamp.IsZero())
}

func TestUT_Save_DirectoryCreation(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Verify directory doesn't exist yet
	replayDir := filepath.Join(tempHome, ".tag", "replay")
	assert.NoDirExists(t, replayDir)

	err := Save("gh:user/repo", "", map[string]any{"key": "value"}, nil)
	require.NoError(t, err)

	// Verify directory was created
	assert.DirExists(t, replayDir)

	// Verify directory permissions (0700)
	info, err := os.Stat(replayDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestUT_Save_FilePermissions(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	err := Save("gh:user/repo", "", map[string]any{"key": "value"}, nil)
	require.NoError(t, err)

	filePath := filepath.Join(tempHome, ".tag", "replay", "gh_user_repo.json")
	info, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestUT_Save_SecretsExcluded(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	values := map[string]any{
		"project_name": "test-project",
		"api_key":      "super-secret-key",
		"password":     "my-password",
		"public_data":  "visible",
	}

	secrets := map[string]bool{
		"api_key":  true,
		"password": true,
	}

	err := Save("gh:user/repo", "", values, secrets)
	require.NoError(t, err)

	// Load and verify secrets are excluded
	loaded, err := Load("gh:user/repo")
	require.NoError(t, err)

	assert.Equal(t, "test-project", loaded.Values["project_name"])
	assert.Equal(t, "visible", loaded.Values["public_data"])
	assert.NotContains(t, loaded.Values, "api_key")
	assert.NotContains(t, loaded.Values, "password")
}

func TestUT_Save_OverwritesExisting(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// First save
	err := Save("gh:user/repo", "v1.0.0", map[string]any{"key": "old-value"}, nil)
	require.NoError(t, err)

	// Second save with different values
	err = Save("gh:user/repo", "v2.0.0", map[string]any{"key": "new-value"}, nil)
	require.NoError(t, err)

	// Verify new values
	loaded, err := Load("gh:user/repo")
	require.NoError(t, err)

	assert.Equal(t, "v2.0.0", loaded.Version)
	assert.Equal(t, "new-value", loaded.Values["key"])
}

func TestUT_Save_EmptySource(t *testing.T) {
	err := Save("", "", map[string]any{}, nil)
	assert.ErrorIs(t, err, ErrEmptyTemplateSource)
}

// =============================================================================
// TestUT_Load - Load replay data tests
// =============================================================================

func TestUT_Load_Success(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Create replay directory and file
	replayDir := filepath.Join(tempHome, ".tag", "replay")
	err := os.MkdirAll(replayDir, 0o700)
	require.NoError(t, err)

	timestamp := time.Date(2026, 2, 5, 10, 30, 0, 0, time.UTC)
	data := ReplayData{
		Template:  "gh:user/repo",
		Version:   "v1.0.0",
		Timestamp: timestamp,
		Values: map[string]any{
			"project_name": "loaded-project",
			"count":        42.0,
		},
	}

	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	filePath := filepath.Join(replayDir, "gh_user_repo.json")
	err = os.WriteFile(filePath, jsonData, 0o600)
	require.NoError(t, err)

	// Load and verify
	loaded, err := Load("gh:user/repo")
	require.NoError(t, err)

	assert.Equal(t, "gh:user/repo", loaded.Template)
	assert.Equal(t, "v1.0.0", loaded.Version)
	assert.Equal(t, timestamp, loaded.Timestamp)
	assert.Equal(t, "loaded-project", loaded.Values["project_name"])
	assert.Equal(t, 42.0, loaded.Values["count"])
}

func TestUT_Load_NotFound(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	_, err := Load("gh:nonexistent/repo")
	assert.ErrorIs(t, err, ErrReplayNotFound)
}

func TestUT_Load_CorruptJSON(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Create replay directory and file with invalid JSON
	replayDir := filepath.Join(tempHome, ".tag", "replay")
	err := os.MkdirAll(replayDir, 0o700)
	require.NoError(t, err)

	filePath := filepath.Join(replayDir, "gh_user_repo.json")
	err = os.WriteFile(filePath, []byte("{ invalid json"), 0o600)
	require.NoError(t, err)

	_, err = Load("gh:user/repo")
	assert.ErrorIs(t, err, ErrReplayCorrupt)
}

func TestUT_Load_EmptyValues(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Create replay directory and file with empty values
	replayDir := filepath.Join(tempHome, ".tag", "replay")
	err := os.MkdirAll(replayDir, 0o700)
	require.NoError(t, err)

	data := ReplayData{
		Template: "gh:user/repo",
		// Values is nil
	}

	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	filePath := filepath.Join(replayDir, "gh_user_repo.json")
	err = os.WriteFile(filePath, jsonData, 0o600)
	require.NoError(t, err)

	// Load should succeed and initialize empty map
	loaded, err := Load("gh:user/repo")
	require.NoError(t, err)
	assert.NotNil(t, loaded.Values)
	assert.Empty(t, loaded.Values)
}

func TestUT_Load_EmptySource(t *testing.T) {
	_, err := Load("")
	assert.ErrorIs(t, err, ErrEmptyTemplateSource)
}

// =============================================================================
// TestUT_Exists - Check replay existence tests
// =============================================================================

func TestUT_Exists_True(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Save some data
	err := Save("gh:user/repo", "", map[string]any{"key": "value"}, nil)
	require.NoError(t, err)

	assert.True(t, Exists("gh:user/repo"))
}

func TestUT_Exists_False_NoFile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	assert.False(t, Exists("gh:nonexistent/repo"))
}

func TestUT_Exists_False_NoDirectory(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Don't create the replay directory
	assert.False(t, Exists("gh:user/repo"))
}

func TestUT_Exists_False_EmptySource(t *testing.T) {
	assert.False(t, Exists(""))
	assert.False(t, Exists("   "))
}

// =============================================================================
// TestUT_Delete - Delete replay data tests
// =============================================================================

func TestUT_Delete_Success(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Save then delete
	err := Save("gh:user/repo", "", map[string]any{"key": "value"}, nil)
	require.NoError(t, err)
	assert.True(t, Exists("gh:user/repo"))

	err = Delete("gh:user/repo")
	require.NoError(t, err)
	assert.False(t, Exists("gh:user/repo"))
}

func TestUT_Delete_NotFound(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Delete non-existent file should succeed
	err := Delete("gh:nonexistent/repo")
	assert.NoError(t, err)
}

func TestUT_Delete_EmptySource(t *testing.T) {
	err := Delete("")
	assert.ErrorIs(t, err, ErrEmptyTemplateSource)
}

// =============================================================================
// TestUT_GetReplayFilePath - Get file path tests
// =============================================================================

func TestUT_GetReplayFilePath_Success(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	path, err := GetReplayFilePath("gh:user/repo")
	require.NoError(t, err)

	expected := filepath.Join(tempHome, ".tag", "replay", "gh_user_repo.json")
	assert.Equal(t, expected, path)
}

func TestUT_GetReplayFilePath_EmptySource(t *testing.T) {
	_, err := GetReplayFilePath("")
	assert.ErrorIs(t, err, ErrEmptyTemplateSource)
}

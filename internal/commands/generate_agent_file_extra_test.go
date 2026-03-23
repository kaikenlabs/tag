package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ReplaceMarkerSection_MissingEndMarker_AppendsNewSection(t *testing.T) {
	t.Parallel()

	existing := "header\n" + agentMarkerStart + "\nold"
	newSection := agentMarkerStart + "\nnew\n" + agentMarkerEnd + "\n"

	result := replaceMarkerSection(existing, newSection)
	assert.Contains(t, result, "header")
	assert.Contains(t, result, "new")
	assert.Contains(t, result, agentMarkerEnd)
}

func TestUT_WriteAgentFile_CreatesNewFileAndParentDirectory(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	path := filepath.Join(base, "docs", "CLAUDE.md")
	content := agentMarkerStart + "\nline\n" + agentMarkerEnd + "\n"

	err := writeAgentFile(path, content)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestUT_WriteAgentFile_ReplacesExistingMarkerSection(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	existing := "# Intro\n\n" + agentMarkerStart + "\nold\n" + agentMarkerEnd + "\n\nfooter\n"
	require.NoError(t, os.WriteFile(path, []byte(existing), 0o644))

	newSection := agentMarkerStart + "\nnew body\n" + agentMarkerEnd + "\n"
	require.NoError(t, writeAgentFile(path, newSection))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	result := string(data)
	assert.Contains(t, result, "# Intro")
	assert.Contains(t, result, "new body")
	assert.NotContains(t, result, "old")
	assert.Contains(t, result, "footer")
}

func TestUT_WriteAgentFile_AppendsWhenNoMarkers(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "AGENTS.md")
	require.NoError(t, os.WriteFile(path, []byte("existing"), 0o644))

	newSection := agentMarkerStart + "\ncontent\n" + agentMarkerEnd + "\n"
	require.NoError(t, writeAgentFile(path, newSection))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	result := string(data)
	assert.Contains(t, result, "existing\n\n")
	assert.Contains(t, result, newSection)
}

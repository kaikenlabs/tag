package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_GenerateAgentFile_NewFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmplContent := "---\nto: {{ name }}.go\ndesc: test gen\n---\npackage main\n"
	createGenerator(t, tmpDir, "mygen", tmplContent)

	cfg := createTestConfig(t, tmpDir)
	outPath := filepath.Join(tmpDir, "CLAUDE.md")

	var buf bytes.Buffer
	err := generateAgentFile(cfg, "claude", outPath, &buf)
	require.NoError(t, err)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, agentMarkerStart)
	assert.Contains(t, content, agentMarkerEnd)
	assert.Contains(t, content, "mygen")
	assert.Contains(t, content, "generator")
	assert.Contains(t, buf.String(), "Wrote agent file")
}

func TestUT_GenerateAgentFile_ReplaceExisting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmplContent := "---\nto: {{ name }}.go\ndesc: test gen\n---\npackage main\n"
	createGenerator(t, tmpDir, "mygen", tmplContent)

	cfg := createTestConfig(t, tmpDir)
	outPath := filepath.Join(tmpDir, "CLAUDE.md")

	// Write first time.
	var buf bytes.Buffer
	require.NoError(t, generateAgentFile(cfg, "claude", outPath, &buf))

	first, err := os.ReadFile(outPath)
	require.NoError(t, err)

	// Write second time — should be idempotent.
	buf.Reset()
	require.NoError(t, generateAgentFile(cfg, "claude", outPath, &buf))

	second, err := os.ReadFile(outPath)
	require.NoError(t, err)

	assert.Equal(t, string(first), string(second))
}

func TestUT_GenerateAgentFile_AppendNoMarkers(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmplContent := "---\nto: {{ name }}.go\n---\npackage main\n"
	createGenerator(t, tmpDir, "mygen", tmplContent)

	cfg := createTestConfig(t, tmpDir)
	outPath := filepath.Join(tmpDir, "existing.md")

	// Create existing file without markers.
	existing := "# My Project\n\nSome existing content.\n"
	require.NoError(t, os.WriteFile(outPath, []byte(existing), 0o644))

	var buf bytes.Buffer
	require.NoError(t, generateAgentFile(cfg, "claude", outPath, &buf))

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "# My Project")
	assert.Contains(t, content, agentMarkerStart)
	assert.Contains(t, content, agentMarkerEnd)
}

func TestUT_GenerateAgentFile_AllFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format   string
		expected string
	}{
		{"claude", "CLAUDE.md"},
		{"cursor", ".cursorrules"},
		{"windsurf", ".windsurfrules"},
		{"copilot", ".github/copilot-instructions.md"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()

			path, ok := agentFileDefaults[tt.format]
			require.True(t, ok)
			assert.Equal(t, tt.expected, path)
		})
	}
}

func TestUT_GenerateAgentFile_CustomOutput(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmplContent := "---\nto: {{ name }}.go\n---\npackage main\n"
	createGenerator(t, tmpDir, "mygen", tmplContent)

	cfg := createTestConfig(t, tmpDir)
	customPath := filepath.Join(tmpDir, "custom", "agent.md")

	var buf bytes.Buffer
	require.NoError(t, generateAgentFile(cfg, "claude", customPath, &buf))

	_, err := os.Stat(customPath)
	require.NoError(t, err)
}

func TestUT_GenerateAgentFile_InvalidFormat(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	err := generateAgentFile(cfg, "invalid", "", &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format")
}

func TestUT_BuildAgentContent(t *testing.T) {
	t.Parallel()

	lists := generatorLists{
		templateGens: []generatorInfo{
			{Name: "api", Description: "API handler"},
		},
		localGens: []generatorInfo{
			{Name: "model", Description: "Data model"},
		},
		templateBundles: []generatorInfo{
			{Name: "crud", Description: "Full CRUD"},
		},
	}

	content := buildAgentContent(lists)

	assert.Contains(t, content, agentMarkerStart)
	assert.Contains(t, content, agentMarkerEnd)
	assert.Contains(t, content, "| api | generator | API handler |")
	assert.Contains(t, content, "| model | generator | Data model |")
	assert.Contains(t, content, "| crud | bundle | Full CRUD |")
	assert.Contains(t, content, "## Code Generators")
	assert.Contains(t, content, "tag generate info")
}

func TestUT_ReplaceMarkerSection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		existing string
		content  string
		check    func(t *testing.T, result string)
	}{
		{
			name:     "replaces between markers",
			existing: "before\n" + agentMarkerStart + "\nold content\n" + agentMarkerEnd + "\nafter\n",
			content:  agentMarkerStart + "\nnew content\n" + agentMarkerEnd + "\n",
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Contains(t, result, "before\n")
				assert.Contains(t, result, "new content")
				assert.NotContains(t, result, "old content")
				assert.Contains(t, result, "after")
			},
		},
		{
			name:     "no markers appends",
			existing: "existing content\n",
			content:  "new section\n",
			check: func(t *testing.T, result string) {
				t.Helper()
				assert.Contains(t, result, "existing content")
				assert.Contains(t, result, "new section")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := replaceMarkerSection(tt.existing, tt.content)
			tt.check(t, result)
		})
	}
}

func TestUT_GenerateAgentFile_CopilotCreatesDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmplContent := "---\nto: {{ name }}.go\n---\npackage main\n"
	createGenerator(t, tmpDir, "mygen", tmplContent)

	cfg := createTestConfig(t, tmpDir)
	outPath := filepath.Join(tmpDir, ".github", "copilot-instructions.md")

	var buf bytes.Buffer
	require.NoError(t, generateAgentFile(cfg, "copilot", outPath, &buf))

	_, err := os.Stat(filepath.Join(tmpDir, ".github"))
	require.NoError(t, err, ".github directory should be created")

	_, err = os.Stat(outPath)
	require.NoError(t, err, "copilot instructions file should exist")
}

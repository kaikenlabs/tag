package scaffold

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
)

// =============================================================================
// Test-only helpers (moved from output.go — only used by tests)
// =============================================================================

// openAndReadRegularFile opens a file with TOCTOU-safe symlink verification and reads its content.
// It performs: Lstat → Open → f.Stat → os.SameFile verification → Read.
// Returns the file content and sanitized file mode, or an error if the file
// is a symlink or was swapped between check and open.
func openAndReadRegularFile(path string) ([]byte, fs.FileMode, error) {
	f, mode, err := openRegularFile(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return nil, 0, fmt.Errorf("read: %w", err)
	}

	return content, mode, nil
}

// --- validatePathWithinDir ---

func TestUT_ValidatePathWithinDir_ValidPaths(t *testing.T) {
	base := t.TempDir()

	tests := []struct {
		name string
		path string
	}{
		{"simple file", filepath.Join(base, "file.txt")},
		{"nested file", filepath.Join(base, "sub", "dir", "file.txt")},
		{"dot in name", filepath.Join(base, ".hidden", "file.txt")},
		{"cleaned relative", filepath.Join(base, "sub", "..", "sub", "file.txt")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathWithinDir(tt.path, base)
			assert.NoError(t, err)
		})
	}
}

func TestUT_ValidatePathWithinDir_PathTraversal(t *testing.T) {
	base := t.TempDir()

	tests := []struct {
		name string
		path string
	}{
		{"parent traversal", filepath.Join(base, "..", "escape.txt")},
		{"double parent traversal", filepath.Join(base, "..", "..", "escape.txt")},
		{"absolute path outside", "/tmp/evil.txt"},
		{"prefix collision", base + "X/file.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathWithinDir(tt.path, base)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "escapes base directory")
		})
	}
}

func TestUT_ValidatePathWithinDir_PathEqualsBase(t *testing.T) {
	base := t.TempDir()
	// Path equal to base should be allowed (the `absPath != absBase` check)
	err := validatePathWithinDir(base, base)
	assert.NoError(t, err)
}

func TestUT_ValidatePathWithinDir_EmptyPath(t *testing.T) {
	base := t.TempDir()
	// Empty path resolves to cwd via filepath.Abs, which is outside base
	err := validatePathWithinDir("", base)
	assert.Error(t, err)
}

// --- sanitizeFileMode ---

func TestUT_SanitizeFileMode(t *testing.T) {
	tests := []struct {
		name     string
		input    fs.FileMode
		expected fs.FileMode
	}{
		{"normal file permissions", 0o644, 0o644},
		{"normal dir permissions", 0o755, 0o755},
		{"setuid bit removed", fs.ModeSetuid | 0o755, 0o755},
		{"setgid bit removed", fs.ModeSetgid | 0o755, 0o755},
		{"sticky bit removed", fs.ModeSticky | 0o755, 0o755},
		{"all dangerous bits removed", fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky | 0o755, 0o755},
		{"zero permissions", 0, 0},
		{"only setuid no perms", fs.ModeSetuid, 0},
		{"executable preserved", 0o755, 0o755},
		{"read only preserved", 0o444, 0o444},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFileMode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Write (end-to-end integration) ---

func mustNewEngine(t *testing.T) *template.Engine {
	t.Helper()
	engine, err := template.NewEngine()
	require.NoError(t, err)
	return engine
}

func mustNewOutputWriter(t *testing.T) *DefaultOutputWriter {
	t.Helper()
	engine := mustNewEngine(t)
	pathProcessor := NewPathProcessor(engine)
	return NewOutputWriter(engine, pathProcessor)
}

func TestUT_Write_TemplateRendering(t *testing.T) {
	writer := mustNewOutputWriter(t)

	// Setup template directory
	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create a template file
	err := os.WriteFile(
		filepath.Join(templateDir, "hello.txt"),
		[]byte("Hello, {{ vars.name }}!"),
		0o644,
	)
	require.NoError(t, err)

	vars := map[string]any{"name": "World"}
	err = writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// Verify rendered content
	content, err := os.ReadFile(filepath.Join(outputDir, "hello.txt"))
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", string(content))
}

func TestUT_Write_BinaryFilePassthrough(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create binary file with null bytes and {{ sequences (should not be processed as template)
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00, '{', '{', ' ', 'v', 'a', 'r', 's', '.', 'x', ' ', '}', '}'}
	err := os.WriteFile(filepath.Join(templateDir, "image.png"), binaryContent, 0o644)
	require.NoError(t, err)

	vars := map[string]any{"x": "should_not_appear"}
	err = writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// Binary file should be copied as-is, not template-processed
	content, err := os.ReadFile(filepath.Join(outputDir, "image.png"))
	require.NoError(t, err)
	assert.Equal(t, binaryContent, content)
}

func TestUT_Write_DirectoryCreation(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create nested directory structure
	nestedDir := filepath.Join(templateDir, "sub", "dir")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nestedDir, "file.txt"), []byte("content"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// Verify directory was created and file exists
	info, err := os.Stat(filepath.Join(outputDir, "sub", "dir"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	content, err := os.ReadFile(filepath.Join(outputDir, "sub", "dir", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}

func TestUT_Write_SkipsTagTemplateJSON(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create tag.template.json (should be skipped)
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, types.TemplateConfigFile),
		[]byte(`{"vars":{}}`),
		0o644,
	))
	// Create a normal file
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "keep.txt"),
		[]byte("kept"),
		0o644,
	))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// tag.template.json should NOT exist in output
	_, err = os.Stat(filepath.Join(outputDir, types.TemplateConfigFile))
	assert.True(t, os.IsNotExist(err))

	// Normal file should exist
	_, err = os.Stat(filepath.Join(outputDir, "keep.txt"))
	assert.NoError(t, err)
}

func TestUT_Write_SkipsGeneratorsDir(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create _generators directory with content
	genDir := filepath.Join(templateDir, types.GeneratorsDir)
	require.NoError(t, os.MkdirAll(genDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "gen.txt"), []byte("gen"), 0o644))

	// Create normal file
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "keep.txt"), []byte("kept"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// _generators should NOT exist in output
	_, err = os.Stat(filepath.Join(outputDir, types.GeneratorsDir))
	assert.True(t, os.IsNotExist(err))

	// Normal file should exist
	_, err = os.Stat(filepath.Join(outputDir, "keep.txt"))
	assert.NoError(t, err)
}

func TestUT_Write_SkipsTagTemplatesDir(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create .tag directory with generator content
	tmplDir := filepath.Join(templateDir, types.TemplatesDir)
	require.NoError(t, os.MkdirAll(filepath.Join(tmplDir, "component"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "component", "template.go"), []byte("component"), 0o644))

	// Create normal file
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "keep.txt"), []byte("kept"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// .tag should NOT exist in output
	_, err = os.Stat(filepath.Join(outputDir, types.TemplatesDir))
	assert.True(t, os.IsNotExist(err))

	// Normal file should exist
	_, err = os.Stat(filepath.Join(outputDir, "keep.txt"))
	assert.NoError(t, err)
}

func TestUT_Write_SkipsCacheMetaFile(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create _meta.json at root (should be skipped)
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, types.CacheMetaFile),
		[]byte(`{"source":"gh:user/repo"}`),
		0o644,
	))
	// Create _meta.json in subdirectory (should be kept)
	subDir := filepath.Join(templateDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(subDir, types.CacheMetaFile),
		[]byte(`{"nested":true}`),
		0o644,
	))
	// Create normal file
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "keep.txt"), []byte("kept"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// Root _meta.json should NOT exist in output
	_, err = os.Stat(filepath.Join(outputDir, types.CacheMetaFile))
	assert.True(t, os.IsNotExist(err))

	// Subdirectory _meta.json SHOULD exist in output
	_, err = os.Stat(filepath.Join(outputDir, "subdir", types.CacheMetaFile))
	assert.NoError(t, err)

	// Normal file should exist
	_, err = os.Stat(filepath.Join(outputDir, "keep.txt"))
	assert.NoError(t, err)
}

func TestUT_Write_PathPlaceholders(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create file with placeholder in directory name
	placeholderDir := filepath.Join(templateDir, "{{ vars.project_name }}")
	require.NoError(t, os.MkdirAll(placeholderDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(placeholderDir, "main.go"),
		[]byte("package {{ vars.project_name }}"),
		0o644,
	))

	vars := map[string]any{"project_name": "myapp"}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// Verify placeholder was resolved in path and content
	content, err := os.ReadFile(filepath.Join(outputDir, "myapp", "main.go"))
	require.NoError(t, err)
	assert.Equal(t, "package myapp", string(content))
}

func TestUT_Write_SymlinkSkipping(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create a real file
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "real.txt"), []byte("real"), 0o644))

	// Create a symlink (should be skipped)
	targetFile := filepath.Join(t.TempDir(), "outside.txt")
	require.NoError(t, os.WriteFile(targetFile, []byte("outside"), 0o644))
	require.NoError(t, os.Symlink(targetFile, filepath.Join(templateDir, "link.txt")))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// Real file should exist
	_, err = os.Stat(filepath.Join(outputDir, "real.txt"))
	assert.NoError(t, err)

	// Symlink should NOT be in output
	_, err = os.Stat(filepath.Join(outputDir, "link.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Write_ConditionalFileExclusion(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create file with conditional filename (false condition -> excluded)
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, `{% if vars.include == "yes" %}optional.txt{% endif %}`),
		[]byte("optional content"),
		0o644,
	))
	// Create normal file
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "always.txt"), []byte("always"), 0o644))

	vars := map[string]any{"include": "no"}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// Conditional file should NOT exist
	_, err = os.Stat(filepath.Join(outputDir, "optional.txt"))
	assert.True(t, os.IsNotExist(err))

	// Normal file should exist
	_, err = os.Stat(filepath.Join(outputDir, "always.txt"))
	assert.NoError(t, err)
}

func TestUT_Write_PathTraversalViaPlaceholder(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create a file whose path placeholder resolves to a traversal attempt
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "{{ vars.out }}"),
		[]byte("escaped"),
		0o644,
	))

	vars := map[string]any{"out": "../escape.txt"}
	err := writer.Write(templateDir, outputDir, vars)
	require.Error(t, err)

	var pathErr *PathError
	assert.ErrorAs(t, err, &pathErr)
	assert.Contains(t, err.Error(), "path traversal detected")

	// Escaped file should NOT exist outside output dir
	_, statErr := os.Stat(filepath.Join(outputDir, "..", "escape.txt"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestUT_Write_SymlinkDirSkipping(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create a real file
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "real.txt"), []byte("real"), 0o644))

	// Create a symlink to a directory outside the template
	externalDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(externalDir, "secret.txt"), []byte("secret"), 0o644))
	require.NoError(t, os.Symlink(externalDir, filepath.Join(templateDir, "linked_dir")))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// Real file should exist
	_, err = os.Stat(filepath.Join(outputDir, "real.txt"))
	assert.NoError(t, err)

	// Symlinked directory and its contents should NOT be in output
	_, err = os.Stat(filepath.Join(outputDir, "linked_dir"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Write_EmptyTemplate(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create empty file
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "empty.txt"), []byte(""), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "empty.txt"))
	require.NoError(t, err)
	assert.Equal(t, "", string(content))
}

// --- processFile ---

// mockTemplate implements template.Template for testing.
type mockTemplate struct {
	result string
	err    error
}

func (m *mockTemplate) Execute(_ template.Context) (string, error) {
	return m.result, m.err
}

// mockRenderer implements template.TemplateRenderer for testing.
type mockRenderer struct {
	parseStringTemplate template.Template
	parseStringErr      error
	executeResult       string
	executeErr          error
}

func (m *mockRenderer) ParseString(_ string) (template.Template, error) {
	return m.parseStringTemplate, m.parseStringErr
}

func (m *mockRenderer) ParseStringNamed(_, _ string) (template.Template, error) {
	return m.parseStringTemplate, m.parseStringErr
}

func (m *mockRenderer) ExecuteToString(_ string, _ template.Context) (string, error) {
	return m.executeResult, m.executeErr
}

var _ template.TemplateRenderer = (*mockRenderer)(nil)

func TestUT_ProcessFile_TextTemplate(t *testing.T) {
	mock := &mockRenderer{
		parseStringTemplate: &mockTemplate{result: "rendered output", err: nil},
	}
	writer := NewOutputWriter(mock, nil)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	srcPath := filepath.Join(templateDir, "file.txt")
	require.NoError(t, os.WriteFile(srcPath, []byte("{{ vars.name }}"), 0o644))

	destPath := filepath.Join(outputDir, "file.txt")
	info, err := os.Stat(srcPath)
	require.NoError(t, err)

	// Create a dirEntry from the file info
	ctx := template.Context{"vars": map[string]any{"name": "test"}}
	err = writer.processFile(srcPath, destPath, ctx, newTestDirEntry(info))
	require.NoError(t, err)

	content, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, "rendered output", string(content))
}

func TestUT_ProcessFile_BinaryFile(t *testing.T) {
	// Binary files should be copied as-is, mock should NOT be called for parsing
	mock := &mockRenderer{
		parseStringTemplate: &mockTemplate{result: "SHOULD NOT APPEAR", err: nil},
	}
	writer := NewOutputWriter(mock, nil)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF}
	srcPath := filepath.Join(templateDir, "binary.bin")
	require.NoError(t, os.WriteFile(srcPath, binaryContent, 0o644))

	destPath := filepath.Join(outputDir, "binary.bin")
	info, err := os.Stat(srcPath)
	require.NoError(t, err)

	ctx := template.Context{}
	err = writer.processFile(srcPath, destPath, ctx, newTestDirEntry(info))
	require.NoError(t, err)

	content, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, binaryContent, content)
}

func TestUT_ProcessFile_SanitizesFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode tests not reliable on Windows")
	}

	mock := &mockRenderer{
		parseStringTemplate: &mockTemplate{result: "content", err: nil},
	}
	writer := NewOutputWriter(mock, nil)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	srcPath := filepath.Join(templateDir, "exec.sh")
	require.NoError(t, os.WriteFile(srcPath, []byte("#!/bin/sh"), 0o755))
	// Set setuid bit on source file
	require.NoError(t, os.Chmod(srcPath, fs.ModeSetuid|0o755))

	destPath := filepath.Join(outputDir, "exec.sh")
	info, err := os.Stat(srcPath)
	require.NoError(t, err)

	ctx := template.Context{}
	err = writer.processFile(srcPath, destPath, ctx, newTestDirEntry(info))
	require.NoError(t, err)

	destInfo, err := os.Stat(destPath)
	require.NoError(t, err)
	// Setuid bit should be stripped
	assert.Equal(t, fs.FileMode(0), destInfo.Mode()&fs.ModeSetuid)
	// Execute bit should be preserved
	assert.NotEqual(t, fs.FileMode(0), destInfo.Mode()&0o111)
}

func TestUT_ProcessFile_TemplateParseError(t *testing.T) {
	mock := &mockRenderer{
		parseStringErr: errors.New("parse error: invalid syntax"),
	}
	writer := NewOutputWriter(mock, nil)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	srcPath := filepath.Join(templateDir, "bad.txt")
	require.NoError(t, os.WriteFile(srcPath, []byte("valid text"), 0o644))

	destPath := filepath.Join(outputDir, "bad.txt")
	info, err := os.Stat(srcPath)
	require.NoError(t, err)

	ctx := template.Context{}
	err = writer.processFile(srcPath, destPath, ctx, newTestDirEntry(info))
	require.Error(t, err)

	var tmplErr *FileProcessingError
	assert.ErrorAs(t, err, &tmplErr)
}

func TestUT_ProcessFile_TemplateExecuteError(t *testing.T) {
	mock := &mockRenderer{
		parseStringTemplate: &mockTemplate{result: "", err: errors.New("execute failed")},
	}
	writer := NewOutputWriter(mock, nil)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	srcPath := filepath.Join(templateDir, "bad.txt")
	require.NoError(t, os.WriteFile(srcPath, []byte("valid text"), 0o644))

	destPath := filepath.Join(outputDir, "bad.txt")
	info, err := os.Stat(srcPath)
	require.NoError(t, err)

	ctx := template.Context{}
	err = writer.processFile(srcPath, destPath, ctx, newTestDirEntry(info))
	require.Error(t, err)

	var tmplErr *FileProcessingError
	assert.ErrorAs(t, err, &tmplErr)
}

// --- GenerateTagConfig ---

func TestUT_GenerateTagConfig_DefaultOptions(t *testing.T) {
	outputDir := t.TempDir()

	err := GenerateTagConfig(outputDir, TagConfigOptions{})
	require.NoError(t, err)

	configPath := filepath.Join(outputDir, ".tagconfig.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config))

	// Schema version always present
	assert.Equal(t, float64(types.TagConfigSchemaVersion), config["schema_version"])

	// Template section always present with type
	tmpl, ok := config["template"].(map[string]any)
	require.True(t, ok, "template should always be a map")
	assert.Equal(t, string(types.TemplateTypeLocal), tmpl["type"])

	// Skip patterns always present as empty array
	skipPatterns, ok := config["skip_patterns"].([]any)
	require.True(t, ok, "skip_patterns should be an array")
	assert.Empty(t, skipPatterns)

	// Verify env section
	env, ok := config["env"].(map[string]any)
	require.True(t, ok, "env should be a map")
	assert.Equal(t, types.TemplatesDir, env["TAG_PATH"])
	assert.Equal(t, types.SharedDir, env["TAG_SHARED_PATH"])
	assert.Equal(t, types.BundlesDir, env["TAG_BUNDLE_PATH"])

	// Verify hooks section
	hooks, ok := config["hooks"].(map[string]any)
	require.True(t, ok, "hooks should be a map")
	assert.NotNil(t, hooks["pre"])
	assert.NotNil(t, hooks["post"])

	// No variables when not provided
	assert.Nil(t, config["variables"])
}

func TestUT_GenerateTagConfig_RemoteTemplate(t *testing.T) {
	outputDir := t.TempDir()

	err := GenerateTagConfig(outputDir, TagConfigOptions{
		TemplateType:    types.TemplateTypeRemote,
		TemplateSource:  "gh:acme/nextjs-starter",
		TemplateName:    "nextjs-starter",
		TemplateVersion: "1.2.0",
		TemplateRef:     "v1.2.0",
		CommitSHA:       "abc123def456",
		Variables: map[string]any{
			"project_name": "my-app",
			"use_docker":   true,
		},
	})
	require.NoError(t, err)

	configPath := filepath.Join(outputDir, ".tagconfig.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config))

	// Schema version
	assert.Equal(t, float64(types.TagConfigSchemaVersion), config["schema_version"])

	// Verify template origin
	tmpl, ok := config["template"].(map[string]any)
	require.True(t, ok, "template should be a map")
	assert.Equal(t, "remote", tmpl["type"])
	assert.Equal(t, "gh:acme/nextjs-starter", tmpl["source"])
	assert.Equal(t, "nextjs-starter", tmpl["name"])
	assert.Equal(t, "1.2.0", tmpl["version"])
	assert.Equal(t, "v1.2.0", tmpl["ref"])
	assert.Equal(t, "abc123def456", tmpl["commit"])

	// Verify variables
	vars, ok := config["variables"].(map[string]any)
	require.True(t, ok, "variables should be a map")
	assert.Equal(t, "my-app", vars["project_name"])
	assert.Equal(t, true, vars["use_docker"])
}

func TestUT_GenerateTagConfig_WithoutVersion(t *testing.T) {
	outputDir := t.TempDir()

	err := GenerateTagConfig(outputDir, TagConfigOptions{
		TemplateType:   types.TemplateTypeRemote,
		TemplateSource: "gh:acme/repo",
		TemplateName:   "repo",
	})
	require.NoError(t, err)

	configPath := filepath.Join(outputDir, ".tagconfig.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config))

	tmpl, ok := config["template"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "remote", tmpl["type"])
	assert.Equal(t, "gh:acme/repo", tmpl["source"])
	assert.Equal(t, "repo", tmpl["name"])
	assert.Nil(t, tmpl["version"]) // version omitted when empty
}

func TestUT_GenerateTagConfig_SkipPatterns(t *testing.T) {
	outputDir := t.TempDir()

	err := GenerateTagConfig(outputDir, TagConfigOptions{
		TemplateType:   types.TemplateTypeLocal,
		TemplateSource: "./my-template",
		SkipPatterns:   []string{"*.log", "temp/**"},
	})
	require.NoError(t, err)

	configPath := filepath.Join(outputDir, ".tagconfig.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config))

	skipPatterns, ok := config["skip_patterns"].([]any)
	require.True(t, ok)
	assert.Len(t, skipPatterns, 2)
	assert.Equal(t, "*.log", skipPatterns[0])
	assert.Equal(t, "temp/**", skipPatterns[1])
}

func TestUT_LoadTagConfig_V1(t *testing.T) {
	dir := t.TempDir()
	configJSON := `{
  "schema_version": 1,
  "template": {
    "type": "remote",
    "source": "gh:acme/starter",
    "name": "starter",
    "version": "2.0.0",
    "ref": "main",
    "commit": "deadbeef1234"
  },
  "variables": {"project_name": "my-proj"},
  "skip_patterns": ["*.bak"],
  "env": {},
  "hooks": {}
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tagconfig.json"), []byte(configJSON), 0o644))

	cfg, err := LoadTagConfig(dir)
	require.NoError(t, err)

	assert.Equal(t, 1, cfg.SchemaVersion)
	require.NotNil(t, cfg.Template)
	assert.Equal(t, types.TemplateTypeRemote, cfg.Template.Type)
	assert.Equal(t, "gh:acme/starter", cfg.Template.Source)
	assert.Equal(t, "starter", cfg.Template.Name)
	assert.Equal(t, "2.0.0", cfg.Template.Version)
	assert.Equal(t, "main", cfg.Template.Ref)
	assert.Equal(t, "deadbeef1234", cfg.Template.CommitSHA)
	assert.Equal(t, []string{"*.bak"}, cfg.SkipPatterns)
	assert.True(t, cfg.HasTemplateOrigin())
}

func TestUT_LoadTagConfig_LegacyV0(t *testing.T) {
	dir := t.TempDir()
	// Legacy format: no schema_version, no type, no skip_patterns
	configJSON := `{
  "template": {
    "source": "gh:acme/old-template",
    "name": "old-template"
  },
  "variables": {"name": "legacy"},
  "env": {},
  "hooks": {}
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tagconfig.json"), []byte(configJSON), 0o644))

	cfg, err := LoadTagConfig(dir)
	require.NoError(t, err)

	// Normalize fills in defaults
	assert.Equal(t, types.TagConfigSchemaVersion, cfg.SchemaVersion)
	assert.Equal(t, types.TemplateTypeRemote, cfg.Template.Type) // inferred from "gh:" prefix
	assert.Equal(t, []string{}, cfg.SkipPatterns)                // nil → empty slice
	assert.True(t, cfg.HasTemplateOrigin())
}

func TestUT_LoadTagConfig_LegacyNoTemplate(t *testing.T) {
	dir := t.TempDir()
	// Legacy format: no template section at all
	configJSON := `{
  "env": {},
  "hooks": {}
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tagconfig.json"), []byte(configJSON), 0o644))

	cfg, err := LoadTagConfig(dir)
	require.NoError(t, err)

	assert.Nil(t, cfg.Template)
	assert.False(t, cfg.HasTemplateOrigin())
}

func TestUT_InferTemplateType(t *testing.T) {
	tests := []struct {
		source   string
		expected types.TemplateType
	}{
		{"gh:acme/repo", types.TemplateTypeRemote},
		{"gl:acme/repo", types.TemplateTypeRemote},
		{"bb:acme/repo", types.TemplateTypeRemote},
		{"https://github.com/acme/repo", types.TemplateTypeRemote},
		{"git@github.com:acme/repo.git", types.TemplateTypeRemote},
		{"git://github.com/acme/repo.git", types.TemplateTypeRemote},
		{"git+ssh://git@github.com/acme/repo.git", types.TemplateTypeRemote},
		{"./local/path", types.TemplateTypeLocal},
		{"/absolute/path", types.TemplateTypeLocal},
		{"", types.TemplateTypeLocal},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			assert.Equal(t, tt.expected, inferTemplateType(tt.source))
		})
	}
}

// --- buildTemplateContext ---

func TestUT_BuildTemplateContext(t *testing.T) {
	vars := map[string]any{
		"name":    "test",
		"version": "1.0",
	}

	ctx := buildTemplateContext(vars)

	// Context should have vars namespace
	ctxVars, ok := ctx["vars"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "test", ctxVars["name"])
	assert.Equal(t, "1.0", ctxVars["version"])
}

// --- openAndReadRegularFile (TOCTOU protection) ---

func TestUT_OpenAndReadRegularFile_RegularFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "regular.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0o644))

	content, mode, err := openAndReadRegularFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
	assert.Equal(t, fs.FileMode(0o644), mode&0o777)
}

func TestUT_OpenAndReadRegularFile_Symlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("secret"), 0o644))

	link := filepath.Join(dir, "link.txt")
	require.NoError(t, os.Symlink(target, link))

	_, _, err := openAndReadRegularFile(link)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink detected")
}

func TestUT_OpenAndReadRegularFile_NonExistent(t *testing.T) {
	_, _, err := openAndReadRegularFile("/nonexistent/path/file.txt")
	require.Error(t, err)
}

func TestUT_OpenAndReadRegularFile_SanitizesMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode tests not reliable on Windows")
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "setuid.sh")
	require.NoError(t, os.WriteFile(filePath, []byte("#!/bin/sh"), 0o755))
	require.NoError(t, os.Chmod(filePath, fs.ModeSetuid|0o755))

	_, mode, err := openAndReadRegularFile(filePath)
	require.NoError(t, err)
	// Setuid should be stripped
	assert.Equal(t, fs.FileMode(0), mode&fs.ModeSetuid)
	// Execute should be preserved
	assert.NotEqual(t, fs.FileMode(0), mode&0o111)
}

// --- openRegularFile ---

func TestUT_OpenRegularFile_RegularFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "regular.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0o644))

	f, mode, err := openRegularFile(filePath)
	require.NoError(t, err)
	defer f.Close()

	assert.Equal(t, fs.FileMode(0o644), mode&0o777)

	content, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(content))
}

func TestUT_OpenRegularFile_Symlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("secret"), 0o644))

	link := filepath.Join(dir, "link.txt")
	require.NoError(t, os.Symlink(target, link))

	_, _, err := openRegularFile(link)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink detected")
}

// --- streamBinaryFile ---

func TestUT_StreamBinaryFile(t *testing.T) {
	dir := t.TempDir()

	// Create source binary file (1MB)
	srcPath := filepath.Join(dir, "source.bin")
	binaryContent := make([]byte, 1024*1024)
	for i := range binaryContent {
		binaryContent[i] = byte(i % 256)
	}
	require.NoError(t, os.WriteFile(srcPath, binaryContent, 0o644))

	// Open source file and read sample (same flow as processFile)
	f, mode, err := openRegularFile(srcPath)
	require.NoError(t, err)
	defer f.Close()

	sample := make([]byte, 8192)
	n, err := f.Read(sample)
	require.NoError(t, err)
	sample = sample[:n]

	// Stream to destination
	destPath := filepath.Join(dir, "dest.bin")
	err = streamBinaryFile(f, destPath, sample, mode)
	require.NoError(t, err)

	// Verify content matches
	destContent, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, binaryContent, destContent)
}

func TestUT_StreamBinaryFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()

	// Create empty source file
	srcPath := filepath.Join(dir, "empty.bin")
	require.NoError(t, os.WriteFile(srcPath, []byte{}, 0o644))

	f, mode, err := openRegularFile(srcPath)
	require.NoError(t, err)
	defer f.Close()

	sample := make([]byte, 8192)
	n, readErr := f.Read(sample)
	sample = sample[:n]
	assert.Equal(t, io.EOF, readErr)
	assert.Equal(t, 0, n)

	destPath := filepath.Join(dir, "dest.bin")
	err = streamBinaryFile(f, destPath, sample, mode)
	require.NoError(t, err)

	destContent, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Empty(t, destContent)
}

func TestUT_StreamBinaryFile_SmallFile(t *testing.T) {
	dir := t.TempDir()

	// Create small binary file (fits entirely in sample)
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x01, 0x02, 0x03}
	srcPath := filepath.Join(dir, "small.bin")
	require.NoError(t, os.WriteFile(srcPath, binaryContent, 0o644))

	f, mode, err := openRegularFile(srcPath)
	require.NoError(t, err)
	defer f.Close()

	sample := make([]byte, 8192)
	n, _ := f.Read(sample)
	sample = sample[:n]

	destPath := filepath.Join(dir, "dest.bin")
	err = streamBinaryFile(f, destPath, sample, mode)
	require.NoError(t, err)

	destContent, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, binaryContent, destContent)
}

// --- Large binary file end-to-end ---

func TestUT_Write_LargeBinaryFileStreaming(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create a 1MB binary file with null bytes
	binaryContent := make([]byte, 1024*1024)
	binaryContent[0] = 0x89 // PNG header signature
	binaryContent[1] = 0x50
	binaryContent[2] = 0x4E
	binaryContent[3] = 0x47
	binaryContent[4] = 0x00 // null byte makes it binary
	for i := 5; i < len(binaryContent); i++ {
		binaryContent[i] = byte(i % 256)
	}
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "large.bin"), binaryContent, 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// Verify content matches exactly
	outputContent, err := os.ReadFile(filepath.Join(outputDir, "large.bin"))
	require.NoError(t, err)
	assert.Equal(t, binaryContent, outputContent)
}

// --- isSkippedEntry ---

func TestUT_IsSkippedEntry(t *testing.T) {
	tests := []struct {
		name    string
		relPath string
		entName string
		want    bool
	}{
		// Root-only files (should be skipped)
		{"tag.template.json at root", "tag.template.json", "tag.template.json", true},
		{"_meta.json at root", "_meta.json", "_meta.json", true},
		{".tagignore at root", ".tagignore", ".tagignore", true},

		// Same files in subdirectories (should NOT be skipped)
		{"tag.template.json in subdir", "sub/tag.template.json", "tag.template.json", false},
		{"_meta.json in subdir", "sub/_meta.json", "_meta.json", false},
		{".tagignore in subdir", "sub/.tagignore", ".tagignore", false},

		// _generators directory tree
		{"_generators root", "_generators", "_generators", true},
		{"nested in _generators", "_generators/handler/handler.go", "handler.go", true},
		{"_generators deep nested", "_generators/a/b/c.go", "c.go", true},
		{"_generatorsx no match", "_generatorsx", "_generatorsx", false},
		{"_generators- no match", "_generators-old", "_generators-old", false},

		// .tag directory tree
		{".tag root", ".tag", ".tag", true},
		{"nested in .tag", ".tag/service/service.go", "service.go", true},
		{".tag deep nested", ".tag/a/b/c.go", "c.go", true},
		{".tagx no match", ".tagx", ".tagx", false},
		{".tag-old no match", ".tag-old", ".tag-old", false},

		// Regular files (should NOT be skipped)
		{"regular file at root", "main.go", "main.go", false},
		{"regular file in subdir", "cmd/server/main.go", "main.go", false},
		{"README at root", "README.md", "README.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSkippedEntry(tt.relPath, tt.entName)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- .tagignore ---

func TestUT_Write_SkipsTagIgnoreFile(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create .tagignore (should itself be skipped)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, ".tagignore"), []byte("*.log\n"), 0o644))
	// Create a normal file
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "keep.txt"), []byte("kept"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// .tagignore should NOT exist in output
	_, err = os.Stat(filepath.Join(outputDir, ".tagignore"))
	assert.True(t, os.IsNotExist(err))

	// Normal file should exist
	_, err = os.Stat(filepath.Join(outputDir, "keep.txt"))
	assert.NoError(t, err)
}

func TestUT_Write_TagIgnoreExcludesFiles(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, ".tagignore"), []byte("*.log\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "app.log"), []byte("log"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "keep.txt"), []byte("kept"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "app.log"))
	assert.True(t, os.IsNotExist(err), "*.log pattern should exclude app.log")

	_, err = os.Stat(filepath.Join(outputDir, "keep.txt"))
	assert.NoError(t, err)
}

func TestUT_Write_TagIgnoreExcludesDirectory(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, ".tagignore"), []byte("temp/\n"), 0o644))
	tempDir := filepath.Join(templateDir, "temp")
	require.NoError(t, os.MkdirAll(tempDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "data.txt"), []byte("tmp"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "keep.txt"), []byte("kept"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "temp"))
	assert.True(t, os.IsNotExist(err), "temp/ pattern should exclude directory")

	_, err = os.Stat(filepath.Join(outputDir, "temp", "data.txt"))
	assert.True(t, os.IsNotExist(err), "files inside ignored dir should be excluded")

	_, err = os.Stat(filepath.Join(outputDir, "keep.txt"))
	assert.NoError(t, err)
}

func TestUT_Write_TagIgnoreGlobPattern(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, ".tagignore"), []byte("**/*.tmp\n"), 0o644))
	nested := filepath.Join(templateDir, "sub", "deep")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "file.tmp"), []byte("tmp"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "file.txt"), []byte("kept"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "sub", "deep", "file.tmp"))
	assert.True(t, os.IsNotExist(err), "**/*.tmp should exclude nested .tmp files")

	_, err = os.Stat(filepath.Join(outputDir, "sub", "deep", "file.txt"))
	assert.NoError(t, err)
}

func TestUT_Write_TagIgnoreNegation(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, ".tagignore"), []byte("*.log\n!keep.log\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "app.log"), []byte("excluded"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "keep.log"), []byte("kept"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "app.log"))
	assert.True(t, os.IsNotExist(err), "*.log should exclude app.log")

	_, err = os.Stat(filepath.Join(outputDir, "keep.log"))
	assert.NoError(t, err, "!keep.log negation should re-include keep.log")
}

func TestUT_Write_TagIgnoreCommentsAndBlanks(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	ignoreContent := "# This is a comment\n\n*.log\n  # indented comment\n  \n"
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, ".tagignore"), []byte(ignoreContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "app.log"), []byte("excluded"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "keep.txt"), []byte("kept"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "app.log"))
	assert.True(t, os.IsNotExist(err), "*.log should still work with comments/blanks in file")

	_, err = os.Stat(filepath.Join(outputDir, "keep.txt"))
	assert.NoError(t, err)
}

func TestUT_Write_TagIgnoreMissing(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// No .tagignore file — all files should be copied normally
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "file.txt"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "file.log"), []byte("log"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "file.txt"))
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "file.log"))
	assert.NoError(t, err)
}

func TestUT_Write_TagIgnoreEmpty(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Empty .tagignore — all files should be copied normally
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, ".tagignore"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "file.txt"), []byte("content"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "file.log"), []byte("log"), 0o644))

	vars := map[string]any{}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	// .tagignore itself should not be in output
	_, err = os.Stat(filepath.Join(outputDir, ".tagignore"))
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(outputDir, "file.txt"))
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(outputDir, "file.log"))
	assert.NoError(t, err)
}

// --- SSTI protection ---

func TestUT_Write_SSTIBlocked_TemplateSyntaxInVarIsLiteral(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Template file uses {{ vars.name }}
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "readme.txt"),
		[]byte("Project: {{ vars.name }}"),
		0o644,
	))

	// User provides template syntax as variable value (SSTI attack)
	vars := map[string]any{"name": "{{ range(999) }}x{{ endfor }}"}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "readme.txt"))
	require.NoError(t, err)
	// Template syntax should appear literally, NOT executed
	assert.Equal(t, "Project: {{ range(999) }}x{{ endfor }}", string(content))
}

func TestUT_Write_SSTIBlocked_NormalVarsStillWork(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "file.txt"),
		[]byte("Hello {{ vars.name }}, version {{ vars.version }}"),
		0o644,
	))

	vars := map[string]any{"name": "World", "version": "1.0"}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "Hello World, version 1.0", string(content))
}

func TestUT_Write_SSTIBlocked_StmtAndCommentSyntax(t *testing.T) {
	writer := mustNewOutputWriter(t)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "file.txt"),
		[]byte("Value: {{ vars.input }}"),
		0o644,
	))

	// User tries {% %} and {# #} injection
	vars := map[string]any{"input": "{% if true %}pwned{% endif %}{# comment #}"}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "Value: {% if true %}pwned{% endif %}{# comment #}", string(content))
}

func TestUT_Write_SSTIAllowRecursiveRender(t *testing.T) {
	engine := mustNewEngine(t)
	pathProcessor := NewPathProcessor(engine)
	writer := NewOutputWriter(engine, pathProcessor)
	writer.SetAllowRecursiveRender(true)

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "file.txt"),
		[]byte("{{ vars.expr }}"),
		0o644,
	))

	// With allowRecursiveRender=true, values are NOT escaped (no sentinel tokens
	// appear in the output). Gonja does not re-evaluate string values, so the
	// template syntax in the value is output literally — but without any
	// escape/unescape round-trip.
	vars := map[string]any{"expr": "{{ 1 + 1 }}"}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "{{ 1 + 1 }}", string(content))
}

func TestUT_Write_SSTIDerivedVarsNotEscaped(t *testing.T) {
	engine := mustNewEngine(t)
	pathProcessor := NewPathProcessor(engine)
	writer := NewOutputWriter(engine, pathProcessor)
	// Mark "greeting" as a derived variable — its value should not be escaped
	writer.SetDerivedVarNames(map[string]bool{"greeting": true})

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "file.txt"),
		[]byte("{{ vars.greeting }}\nInput: {{ vars.input }}"),
		0o644,
	))

	// "greeting" is derived (not escaped), "input" is user-provided (escaped)
	vars := map[string]any{
		"greeting": "Hello!",
		"input":    "{{ malicious }}",
	}
	err := writer.Write(templateDir, outputDir, vars)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(outputDir, "file.txt"))
	require.NoError(t, err)
	// greeting rendered normally, input appears literally (escaped then unescaped)
	assert.Equal(t, "Hello!\nInput: {{ malicious }}", string(content))
}

// --- unescapeTemplateSyntax ---

func TestUT_UnescapeTemplateSyntax_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"expression delimiters", "{{ vars.name }}"},
		{"statement delimiters", "{% if true %}yes{% endif %}"},
		{"comment delimiters", "{# a comment #}"},
		{"mixed", "{{ x }} {% if y %}{# z #}{% endif %}"},
		{"no delimiters", "plain text"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			escaped := escapeTemplateSyntax(tt.input)
			unescaped := unescapeTemplateSyntax(escaped)
			assert.Equal(t, tt.input, unescaped, "round-trip should restore original")
		})
	}
}

// --- test helpers ---

// testDirEntry wraps os.FileInfo to implement fs.DirEntry for tests.
type testDirEntry struct {
	info os.FileInfo
}

func newTestDirEntry(info os.FileInfo) fs.DirEntry {
	return &testDirEntry{info: info}
}

func (d *testDirEntry) Name() string               { return d.info.Name() }
func (d *testDirEntry) IsDir() bool                { return d.info.IsDir() }
func (d *testDirEntry) Type() fs.FileMode          { return d.info.Mode().Type() }
func (d *testDirEntry) Info() (fs.FileInfo, error) { return d.info, nil }

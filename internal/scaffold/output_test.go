package scaffold

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		parseStringErr: fmt.Errorf("parse error: invalid syntax"),
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
		parseStringTemplate: &mockTemplate{result: "", err: fmt.Errorf("execute failed")},
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

// --- CopyGenerators ---

func TestUT_CopyGenerators_WithGenerators(t *testing.T) {
	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// Create _generators with content
	genDir := filepath.Join(templateDir, types.GeneratorsDir)
	require.NoError(t, os.MkdirAll(filepath.Join(genDir, "mygen"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "mygen", "template.txt"), []byte("gen content"), 0o644))

	err := CopyGenerators(templateDir, outputDir)
	require.NoError(t, err)

	// Should be copied to .tag.templates
	content, err := os.ReadFile(filepath.Join(outputDir, types.TemplatesDir, "mygen", "template.txt"))
	require.NoError(t, err)
	assert.Equal(t, "gen content", string(content))
}

func TestUT_CopyGenerators_NoGenerators(t *testing.T) {
	templateDir := t.TempDir()
	outputDir := t.TempDir()

	// No _generators directory exists
	err := CopyGenerators(templateDir, outputDir)
	require.NoError(t, err)

	// Should create empty .tag.templates
	info, err := os.Stat(filepath.Join(outputDir, types.TemplatesDir))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestUT_CopyGenerators_SkipsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	templateDir := t.TempDir()
	outputDir := t.TempDir()

	genDir := filepath.Join(templateDir, types.GeneratorsDir)
	require.NoError(t, os.MkdirAll(genDir, 0o755))

	// Create real file
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "real.txt"), []byte("real"), 0o644))

	// Create symlink pointing outside
	target := filepath.Join(t.TempDir(), "outside.txt")
	require.NoError(t, os.WriteFile(target, []byte("outside"), 0o644))
	require.NoError(t, os.Symlink(target, filepath.Join(genDir, "link.txt")))

	err := CopyGenerators(templateDir, outputDir)
	require.NoError(t, err)

	// Real file should be copied
	_, err = os.Stat(filepath.Join(outputDir, types.TemplatesDir, "real.txt"))
	assert.NoError(t, err)

	// Symlink should NOT be copied
	_, err = os.Stat(filepath.Join(outputDir, types.TemplatesDir, "link.txt"))
	assert.True(t, os.IsNotExist(err))
}

// --- GenerateTagConfig ---

func TestUT_GenerateTagConfig(t *testing.T) {
	outputDir := t.TempDir()

	err := GenerateTagConfig(outputDir)
	require.NoError(t, err)

	configPath := filepath.Join(outputDir, ".tagconfig.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	// Parse and validate JSON structure
	var config map[string]any
	require.NoError(t, json.Unmarshal(data, &config))

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

	// Context should also have root-level vars (from WithRootVars)
	// This makes {{ name }} work in addition to {{ vars.name }}
	rootName, ok := ctx["name"]
	assert.True(t, ok, "root-level vars should be set via WithRootVars")
	assert.Equal(t, "test", rootName)
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

package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestTemplate creates a test template structure in a temporary directory.
func createTestTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create tag.template.json
	config := map[string]any{
		"name":        "test-template",
		"description": "A test template",
		"vars": map[string]any{
			"project_name": "my_project",
			"author": map[string]any{
				"type":     "string",
				"prompt":   "Author name",
				"default":  "Test Author",
				"required": true,
			},
			"use_docker": map[string]any{
				"type":    "boolean",
				"default": false,
			},
			"port": map[string]any{
				"type":    "number",
				"default": 8080,
			},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), configData, 0o644))

	// Create template structure: {{ vars.project_name }}/
	projectDir := filepath.Join(dir, "{{ vars.project_name }}")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Create a template file
	mainTmpl := `package main

import "fmt"

// Project: {{ vars.project_name }}
// Author: {{ vars.author }}
// Port: {{ vars.port }}

func main() {
    fmt.Println("Hello from {{ vars.project_name }}")
}
`
	cmdDir := filepath.Join(projectDir, "cmd")
	require.NoError(t, os.MkdirAll(cmdDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainTmpl), 0o644))

	// Create README template
	readmeTmpl := `# {{ vars.project_name }}

By {{ vars.author }}

{% if vars.use_docker %}
## Docker

Run with Docker on port {{ vars.port }}
{% endif %}
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "README.md"), []byte(readmeTmpl), 0o644))

	// Create a non-template file (binary-like)
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ".gitignore"), []byte("*.exe\n*.o\n"), 0o644))

	// Create _generators directory
	generatorsDir := filepath.Join(dir, "_generators", "handler")
	require.NoError(t, os.MkdirAll(generatorsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(generatorsDir, "handler.tmpl"), []byte("// handler template\n"), 0o644))

	return dir
}

func TestIT_Scaffold_LocalTemplate(t *testing.T) {
	templateDir := createTestTemplate(t)
	outputDir := filepath.Join(t.TempDir(), "output")

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		Meta: map[string]string{
			"project_name": "awesome_project",
			"author":       "John Doe",
		},
		NoInput: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify output structure
	assert.DirExists(t, outputDir)
	assert.DirExists(t, filepath.Join(outputDir, "awesome_project"))
	assert.DirExists(t, filepath.Join(outputDir, "awesome_project", "cmd"))
	assert.FileExists(t, filepath.Join(outputDir, "awesome_project", "cmd", "main.go"))
	assert.FileExists(t, filepath.Join(outputDir, "awesome_project", "README.md"))
	assert.FileExists(t, filepath.Join(outputDir, "awesome_project", ".gitignore"))
	assert.FileExists(t, filepath.Join(outputDir, ".tagconfig.json"))
	assert.DirExists(t, filepath.Join(outputDir, ".tag.templates", "handler"))

	// Verify template was processed
	mainContent, err := os.ReadFile(filepath.Join(outputDir, "awesome_project", "cmd", "main.go"))
	require.NoError(t, err)
	assert.Contains(t, string(mainContent), "Project: awesome_project")
	assert.Contains(t, string(mainContent), "Author: John Doe")
	assert.Contains(t, string(mainContent), "Port: 8080")
	assert.NotContains(t, string(mainContent), "{{")

	// Verify non-template file was copied as-is
	gitignoreContent, err := os.ReadFile(filepath.Join(outputDir, "awesome_project", ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, "*.exe\n*.o\n", string(gitignoreContent))
}

func TestIT_Scaffold_AllVariableTypes(t *testing.T) {
	dir := t.TempDir()

	// Create tag.template.json with all variable types
	config := map[string]any{
		"vars": map[string]any{
			"string_var": "default_string",
			"bool_var":   true,
			"number_var": 42,
			"choice_var": map[string]any{
				"type":    "choice",
				"options": []string{"option1", "option2", "option3"},
				"default": "option2",
			},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), configData, 0o644))

	// Create a template that uses all variable types
	tmpl := `string: {{ vars.string_var }}
bool: {{ vars.bool_var }}
number: {{ vars.number_var }}
choice: {{ vars.choice_var }}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "output.txt"), []byte(tmpl), 0o644))

	outputDir := filepath.Join(t.TempDir(), "output")
	opts := Options{
		TemplateDir: dir,
		OutputDir:   outputDir,
		Meta: map[string]string{
			"string_var": "custom_string",
			"bool_var":   "false",
			"number_var": "99",
			"choice_var": "option3",
		},
		NoInput: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify output
	content, err := os.ReadFile(filepath.Join(outputDir, "output.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "string: custom_string")
	// Gonja outputs "False" (Python-style) for false booleans
	assert.Contains(t, string(content), "bool: False")
	assert.Contains(t, string(content), "number: 99")
	assert.Contains(t, string(content), "choice: option3")
}

func TestIT_Scaffold_PathPlaceholders(t *testing.T) {
	dir := t.TempDir()

	// Create tag.template.json
	config := map[string]any{
		"vars": map[string]any{
			"project_name": "my_project",
			"module_name":  "user",
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), configData, 0o644))

	// Create directory structure with placeholders
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "{{ vars.project_name | snake }}", "internal", "{{ vars.module_name | plural }}"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "{{ vars.project_name | snake }}", "internal", "{{ vars.module_name | plural }}", "{{ vars.module_name }}.go"),
		[]byte("package {{ vars.module_name | plural }}\n"),
		0o644,
	))

	outputDir := filepath.Join(t.TempDir(), "output")
	opts := Options{
		TemplateDir: dir,
		OutputDir:   outputDir,
		Meta: map[string]string{
			"project_name": "MyProject",
			"module_name":  "user",
		},
		NoInput: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify path placeholders were processed
	assert.DirExists(t, filepath.Join(outputDir, "my_project"))
	assert.DirExists(t, filepath.Join(outputDir, "my_project", "internal", "users"))
	assert.FileExists(t, filepath.Join(outputDir, "my_project", "internal", "users", "user.go"))
}

func TestIT_Scaffold_GeneratesTagconfig(t *testing.T) {
	dir := t.TempDir()

	// Create minimal tag.template.json
	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test",
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), configData, 0o644))

	outputDir := filepath.Join(t.TempDir(), "output")
	opts := Options{
		TemplateDir: dir,
		OutputDir:   outputDir,
		NoInput:     true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify .tagconfig.json was generated
	tagconfigPath := filepath.Join(outputDir, ".tagconfig.json")
	assert.FileExists(t, tagconfigPath)

	// Parse and verify content
	data, err := os.ReadFile(tagconfigPath)
	require.NoError(t, err)

	var tagconfig map[string]any
	require.NoError(t, json.Unmarshal(data, &tagconfig))

	env, ok := tagconfig["env"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, ".tag.templates", env["TAG_PATH"])
	assert.Nil(t, env["TAG_EXTENSION"], "TAG_EXTENSION should not be in config")
}

func TestIT_Scaffold_OutputExists_Error(t *testing.T) {
	templateDir := createTestTemplate(t)
	outputDir := filepath.Join(t.TempDir(), "output")

	// Create output directory first
	require.NoError(t, os.MkdirAll(outputDir, 0o755))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		Force:       false,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutputExists)
}

func TestIT_Scaffold_OutputExists_Force(t *testing.T) {
	templateDir := createTestTemplate(t)
	outputDir := filepath.Join(t.TempDir(), "output")

	// Create output directory with a file
	require.NoError(t, os.MkdirAll(outputDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, "existing.txt"), []byte("old content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		Meta: map[string]string{
			"project_name": "test_project",
			"author":       "Test",
		},
		NoInput: true,
		Force:   true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Old file should be gone
	assert.NoFileExists(t, filepath.Join(outputDir, "existing.txt"))
}

func TestIT_Scaffold_TemplateNotFound(t *testing.T) {
	opts := Options{
		TemplateDir: "/nonexistent/path",
		OutputDir:   t.TempDir(),
		NoInput:     true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTemplateNotFound)
}

func TestIT_Scaffold_ConfigNotFound(t *testing.T) {
	dir := t.TempDir()
	// Don't create tag.template.json

	opts := Options{
		TemplateDir: dir,
		OutputDir:   filepath.Join(t.TempDir(), "output"),
		NoInput:     true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrConfigNotFound)
}

func TestIT_Scaffold_BinaryFileCopied(t *testing.T) {
	dir := t.TempDir()

	// Create tag.template.json
	config := map[string]any{"vars": map[string]any{}}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), configData, 0o644))

	// Create a binary-like file with bytes that would be invalid templates
	binaryContent := []byte{0x00, 0x01, 0x02, 0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "image.png"), binaryContent, 0o644))

	outputDir := filepath.Join(t.TempDir(), "output")
	opts := Options{
		TemplateDir: dir,
		OutputDir:   outputDir,
		NoInput:     true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify binary file was copied byte-for-byte
	copiedContent, err := os.ReadFile(filepath.Join(outputDir, "image.png"))
	require.NoError(t, err)
	assert.Equal(t, binaryContent, copiedContent)
}

func TestIT_Scaffold_ProjectNameFromArg(t *testing.T) {
	dir := t.TempDir()

	// Create tag.template.json
	config := map[string]any{
		"vars": map[string]any{
			"project_name": "default_name",
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), configData, 0o644))

	// Create simple template
	require.NoError(t, os.WriteFile(filepath.Join(dir, "info.txt"), []byte("{{ vars.project_name }}\n"), 0o644))

	outputDir := filepath.Join(t.TempDir(), "output")
	opts := Options{
		TemplateDir: dir,
		OutputDir:   outputDir,
		ProjectName: "cli_project_name", // This should override the default
		NoInput:     true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify project_name was used from CLI arg
	content, err := os.ReadFile(filepath.Join(outputDir, "info.txt"))
	require.NoError(t, err)
	assert.Equal(t, "cli_project_name\n", string(content))
}

func TestIT_Scaffold_DefaultOutputDir(t *testing.T) {
	templateDir := createTestTemplate(t)

	// Change to temp directory for test
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(originalDir) }()

	opts := Options{
		TemplateDir: templateDir,
		// No OutputDir specified - should use project_name
		Meta: map[string]string{
			"project_name": "auto_output_dir",
			"author":       "Test",
		},
		NoInput: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Should create directory named after project_name in current dir
	assert.DirExists(t, filepath.Join(tempDir, "auto_output_dir"))
}

func TestIT_Scaffold_PathTraversalPrevention(t *testing.T) {
	dir := t.TempDir()

	// Create tag.template.json
	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test",
			"bad_var":      "../../escape",
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), configData, 0o644))

	// Create a template file with path that would escape output dir
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "{{ vars.bad_var }}"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "{{ vars.bad_var }}", "evil.txt"), []byte("content"), 0o644))

	outputDir := filepath.Join(t.TempDir(), "output")
	opts := Options{
		TemplateDir: dir,
		OutputDir:   outputDir,
		NoInput:     true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	// Should fail due to path traversal detection
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestUT_LoadAndValidateConfig_RejectsLargeFile(t *testing.T) {
	dir := t.TempDir()

	// Create an oversized config file (just over the limit)
	configPath := filepath.Join(dir, "tag.template.json")
	largeData := make([]byte, MaxConfigFileSize+1)
	for i := range largeData {
		largeData[i] = ' '
	}
	require.NoError(t, os.WriteFile(configPath, largeData, 0o644))

	s, err := NewScaffold(Options{NoInput: true})
	require.NoError(t, err)

	_, err = s.loadAndValidateConfig(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config file too large")
}

func TestUT_ValidateSafeOutputDir(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"root path", "/", true},
		{"home directory", os.Getenv("HOME"), true},
		{"shallow path", "/tmp", true},
		{"usr directory", "/usr", true},
		{"etc directory", "/etc", true},
		{"valid deep path", "/some/deep/path", false},
		{"valid nested path", "/home/user/projects/test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSafeOutputDir(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

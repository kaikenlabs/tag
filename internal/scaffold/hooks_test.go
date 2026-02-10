package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Integration Tests for Scaffold with Hooks ---

func TestIT_Scaffold_HooksSkippedInNoInputMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with hooks that create marker files
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{"touch hook_ran.txt"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		// AcceptHooks is false - hooks should be skipped
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Output should be created but hook marker should NOT exist
	assert.DirExists(t, outputDir)
	assert.FileExists(t, filepath.Join(outputDir, "test.txt"))
	assert.NoFileExists(t, filepath.Join(outputDir, "hook_ran.txt"), "hooks should be skipped without --accept-hooks")
}

func TestIT_Scaffold_PreHookSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with pre-scaffold hook
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"pre_scaffold": []string{"echo pre-hook executed"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	// Create a simple template file
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify output was created
	assert.DirExists(t, outputDir)
	assert.FileExists(t, filepath.Join(outputDir, "test.txt"))
}

func TestIT_Scaffold_PreHookFailure_NoOutputCreated(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with failing pre-scaffold hook
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"pre_scaffold": []string{"false"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pre-scaffold hook failed")

	// Verify output was NOT created (clean up symlink-resolved path too)
	assert.NoDirExists(t, outputDir)
}

func TestIT_Scaffold_PostHookSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with post-scaffold hook that creates a marker file
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{"touch post_hook_marker.txt"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify output and marker file were created
	assert.DirExists(t, outputDir)
	assert.FileExists(t, filepath.Join(outputDir, "test.txt"))
	assert.FileExists(t, filepath.Join(outputDir, "post_hook_marker.txt"))
}

func TestIT_Scaffold_PostHookFailure_OutputPreserved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with failing post-scaffold hook
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{"false"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	// Post-hook failures should NOT cause scaffold to fail
	err = s.Run(opts)
	require.NoError(t, err)

	// Verify output was still created despite hook failure
	assert.DirExists(t, outputDir)
	assert.FileExists(t, filepath.Join(outputDir, "test.txt"))
}

func TestIT_Scaffold_HooksReceiveEnvironmentVariables(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template that writes env vars to a file
	// Uses explicit shell invocation since variable expansion requires a shell
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "env_test_project",
			"author":       "Test Author",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{
				"sh -c 'echo TAG_PROJECT_NAME=$TAG_PROJECT_NAME > env_check.txt'",
				"sh -c 'echo TAG_VAR_PROJECT_NAME=$TAG_VAR_PROJECT_NAME >> env_check.txt'",
				"sh -c 'echo TAG_VAR_AUTHOR=$TAG_VAR_AUTHOR >> env_check.txt'",
				"sh -c 'echo TAG_OUTPUT_DIR=$TAG_OUTPUT_DIR >> env_check.txt'",
			},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Read and verify env check file
	envContent, err := os.ReadFile(filepath.Join(outputDir, "env_check.txt"))
	require.NoError(t, err)

	content := string(envContent)
	assert.Contains(t, content, "TAG_PROJECT_NAME=env_test_project")
	assert.Contains(t, content, "TAG_VAR_PROJECT_NAME=env_test_project")
	assert.Contains(t, content, "TAG_VAR_AUTHOR=Test Author")
	assert.Contains(t, content, "TAG_OUTPUT_DIR="+outputDir)
}

func TestIT_Scaffold_PreHooksRunInTemplateDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with pre-hook that writes pwd to a marker file
	// Uses explicit shell invocation since redirect requires a shell
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"pre_scaffold": []string{"sh -c 'pwd > pre_hook_pwd.txt'"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Read the marker file created in template directory
	pwdContent, err := os.ReadFile(filepath.Join(templateDir, "pre_hook_pwd.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(pwdContent), templateDir)
}

func TestIT_Scaffold_PostHooksRunInOutputDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with post-hook that writes pwd to a marker file
	// Uses explicit shell invocation since redirect requires a shell
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{"sh -c 'pwd > post_hook_pwd.txt'"},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Read the marker file created in output directory
	pwdContent, err := os.ReadFile(filepath.Join(outputDir, "post_hook_pwd.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(pwdContent), outputDir)
}

func TestIT_Scaffold_MultipleHooksExecuteInOrder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-specific test")
	}

	// Create template with multiple hooks that append to a file
	// Uses explicit shell invocation since redirects require a shell
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
		"hooks": map[string]any{
			"post_scaffold": []string{
				"sh -c 'echo first > order.txt'",
				"sh -c 'echo second >> order.txt'",
				"sh -c 'echo third >> order.txt'",
			},
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		AcceptHooks: true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify hooks ran in order
	orderContent, err := os.ReadFile(filepath.Join(outputDir, "order.txt"))
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(orderContent)), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "first", lines[0])
	assert.Equal(t, "second", lines[1])
	assert.Equal(t, "third", lines[2])
}

func TestIT_Scaffold_NoHooksConfigured(t *testing.T) {
	// Create template without hooks
	templateDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "output")

	config := map[string]any{
		"vars": map[string]any{
			"project_name": "test_project",
		},
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), configData, 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte("content"), 0o644))

	opts := Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
	}

	s, err := NewScaffold(opts)
	require.NoError(t, err)

	err = s.Run(opts)
	require.NoError(t, err)

	// Verify output was created successfully
	assert.DirExists(t, outputDir)
	assert.FileExists(t, filepath.Join(outputDir, "test.txt"))
}

package convert

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/kaikenlabs/tag/internal/types"
)

// ConvertInPlace converts a Cookiecutter template in-place by writing
// tag.template.json to the same directory as cookiecutter.json (test-only).
// It also converts {{ cookiecutter.* }} references to {{ vars.* }} in all
// text files, hook files, and directory/file names.
func (c *Converter) ConvertInPlace(ctx context.Context, templateDir string) error {
	// Verify it's a Cookiecutter template
	cookiecutterPath := filepath.Join(templateDir, types.CookiecutterConfigFile)
	if _, err := os.Stat(cookiecutterPath); os.IsNotExist(err) {
		return ErrNoCookiecutterConfig
	}

	// Read and convert cookiecutter.json
	configData, err := os.ReadFile(cookiecutterPath)
	if err != nil {
		return fmt.Errorf("failed to read cookiecutter.json: %w", err)
	}

	tagConfig, _, _, err := ConvertCookiecutterConfig(configData)
	if err != nil {
		return err
	}

	// Process hooks (detect only, don't copy - they're already in place)
	hooksProcessor := NewHooksProcessor(templateDir, templateDir, false)
	hookFindings, err := hooksProcessor.DetectHooks()
	if err != nil {
		return fmt.Errorf("failed to detect hooks: %w", err)
	}

	// Add shell hooks to config
	preHooks, postHooks := SuggestTagHooksConfig(hookFindings)
	if len(preHooks) > 0 || len(postHooks) > 0 {
		tagConfig.Hooks = &types.HooksConfig{
			PreScaffold:  preHooks,
			PostScaffold: postHooks,
		}
	}

	// Convert file contents and paths in-place
	if err := c.convertFilesInPlace(templateDir); err != nil { //nolint:govet // shadow in if-init is idiomatic
		return err
	}

	// Write tag.template.json
	tagJSON, err := GenerateTagTemplateJSON(tagConfig, "", "Converted from Cookiecutter template")
	if err != nil {
		return fmt.Errorf("failed to generate tag.template.json: %w", err)
	}

	tagConfigPath := filepath.Join(templateDir, types.TemplateConfigFile)
	if err := os.WriteFile(tagConfigPath, tagJSON, 0o600); err != nil {
		return fmt.Errorf("failed to write tag.template.json: %w", err)
	}

	return nil
}

// convertFilesInPlace walks the template directory and converts cookiecutter.*
// references to vars.* in text file contents and renames paths (test-only).
func (c *Converter) convertFilesInPlace(templateDir string) error {
	// First pass: convert file contents
	err := filepath.WalkDir(templateDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			baseName := filepath.Base(path)
			if baseName == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}
		if relPath == types.CookiecutterConfigFile {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !fileutil.IsTextContent(content) {
			return nil
		}

		converted, changed := ConvertContent(string(content))
		if changed {
			info, infoErr := d.Info()
			if infoErr != nil {
				return infoErr
			}
			return os.WriteFile(path, []byte(converted), info.Mode())
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to convert file contents: %w", err)
	}

	// Second pass: rename paths containing cookiecutter.* (bottom-up via collected list)
	var pathsToRename []string
	err = filepath.WalkDir(templateDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, relErr := filepath.Rel(templateDir, path)
		if relErr != nil {
			return relErr
		}
		if relPath == "." || relPath == types.CookiecutterConfigFile {
			return nil
		}
		baseName := filepath.Base(path)
		if HasCookiecutterPlaceholders(baseName) {
			pathsToRename = append(pathsToRename, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan paths: %w", err)
	}

	// Rename from deepest to shallowest to avoid breaking parent paths
	for i := len(pathsToRename) - 1; i >= 0; i-- {
		oldPath := pathsToRename[i]
		dir := filepath.Dir(oldPath)
		baseName := filepath.Base(oldPath)
		newBase, _ := ConvertPath(baseName)
		newPath := filepath.Join(dir, newBase)
		if oldPath != newPath {
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("failed to rename %s: %w", oldPath, err)
			}
		}
	}

	return nil
}

func TestUT_Converter_DryRun(t *testing.T) {
	// Create a minimal cookiecutter template
	srcDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "output")

	// Create cookiecutter.json
	ccConfig := `{
		"project_name": "test_project",
		"author": "Test Author"
	}`
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(ccConfig), 0o644))

	// Create a template file
	templateDir := filepath.Join(srcDir, "{{ cookiecutter.project_name }}")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "main.go"),
		[]byte("package main\n"),
		0o644,
	))

	// Run in dry-run mode
	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
		DryRun:      true,
	})

	require.NoError(t, err)
	assert.True(t, result.DryRun)
	assert.Equal(t, 2, result.VariablesConverted)
	assert.Equal(t, 1, result.DirsRenamed) // cookiecutter project_name placeholder

	// Verify nothing was written
	_, err = os.Stat(destDir)
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Converter_LocalTemplate(t *testing.T) {
	// Create a cookiecutter template
	srcDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "output")

	// Create cookiecutter.json
	ccConfig := `{
		"project_name": "my_project",
		"use_docker": true,
		"license": ["MIT", "Apache-2.0"]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(ccConfig), 0o644))

	// Create template structure
	projectDir := filepath.Join(srcDir, "{{ cookiecutter.project_name }}")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))

	// Create a template file
	mainContent := `package main

func main() {
	// Project: {{ cookiecutter.project_name }}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "main.go.tmpl"), []byte(mainContent), 0o644))

	// Run conversion
	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})

	require.NoError(t, err)
	assert.Equal(t, destDir, result.Destination)
	assert.Equal(t, 3, result.VariablesConverted)
	assert.Equal(t, 1, result.DirsRenamed)

	// Verify tag.template.json was created
	tagConfigPath := filepath.Join(destDir, "tag.template.json")
	_, err = os.Stat(tagConfigPath)
	require.NoError(t, err)

	tagConfig, err := os.ReadFile(tagConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(tagConfig), "project_name")
	assert.Contains(t, string(tagConfig), "use_docker")

	// Verify directory was renamed
	convertedProjectDir := filepath.Join(destDir, "{{ vars.project_name }}")
	_, err = os.Stat(convertedProjectDir)
	require.NoError(t, err)
}

func TestUT_Converter_OutputExists_NoForce(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir() // Already exists

	// Create cookiecutter.json
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(`{}`), 0o644))

	converter, err := NewConverter()
	require.NoError(t, err)

	_, err = converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
		Force:       false,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutputExists)
}

func TestUT_Converter_OutputExists_WithForce(t *testing.T) {
	srcDir := t.TempDir()
	destDir := t.TempDir()

	// Create cookiecutter.json
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(`{"name": "test"}`), 0o644))

	// Pre-populate destination
	require.NoError(t, os.WriteFile(filepath.Join(destDir, "old_file.txt"), []byte("old"), 0o644))

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
		Force:       true,
	})

	require.NoError(t, err)
	assert.Equal(t, destDir, result.Destination)

	// Verify old file was removed
	_, err = os.Stat(filepath.Join(destDir, "old_file.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Converter_MissingCookiecutterJSON(t *testing.T) {
	srcDir := t.TempDir()
	// Don't create cookiecutter.json

	converter, err := NewConverter()
	require.NoError(t, err)

	_, err = converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: t.TempDir(),
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoCookiecutterConfig)
}

func TestUT_Converter_WithHooks(t *testing.T) {
	srcDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "output")

	// Create cookiecutter.json
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(`{"name": "test"}`), 0o644))

	// Create hooks
	hooksDir := filepath.Join(srcDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre_gen_project.py"), []byte("# hook"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post_gen_project.sh"), []byte("#!/bin/bash"), 0o755))

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})

	require.NoError(t, err)
	assert.Equal(t, 2, result.HooksCopied)
	assert.True(t, len(result.Warnings) >= 2) // Warnings about hooks

	// Verify hooks were copied
	_, err = os.Stat(filepath.Join(destDir, "hooks", "pre_gen_project.py"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(destDir, "hooks", "post_gen_project.sh"))
	require.NoError(t, err)
}

func TestUT_Converter_ContentAnalysis(t *testing.T) {
	srcDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "output")

	// Create cookiecutter.json
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(`{"name": "test"}`), 0o644))

	// Create a template with Jinja2-specific syntax
	content := `{% for k, v in items.items() %}
{{ k }}: {{ v | default('none') }}
{% endfor %}`
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "template.tmpl"), []byte(content), 0o644))

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})

	require.NoError(t, err)
	assert.Greater(t, len(result.Incompatibilities), 0)

	// Check for specific incompatibilities
	kinds := make(map[string]bool)
	for _, inc := range result.Incompatibilities {
		kinds[inc.Kind] = true
	}
	assert.True(t, kinds["dict-iteration"] || kinds["filter-syntax"])
}

func TestUT_Converter_DefaultDestination(t *testing.T) {
	srcDir := t.TempDir()
	// Simulate a cookiecutter template directory name
	srcDir = filepath.Join(srcDir, "cookiecutter-myproject")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(`{}`), 0o644))

	converter, err := NewConverter()
	require.NoError(t, err)

	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: "", // Empty - should infer
	})

	require.NoError(t, err)
	// Should be "myproject-tag" (strips cookiecutter- prefix)
	assert.Equal(t, "myproject-tag", result.Destination)

	// Clean up
	os.RemoveAll(result.Destination)
}

func TestUT_ConvertInPlace_CreatesTagTemplateJSON(t *testing.T) {
	// Create a minimal cookiecutter template
	srcDir := t.TempDir()

	// Create cookiecutter.json
	ccConfig := `{
		"project_name": "test_project",
		"author": "Test Author",
		"use_docker": true
	}`
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(ccConfig), 0o644))

	// Create a template file
	templateDir := filepath.Join(srcDir, "{{ cookiecutter.project_name }}")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "main.go"),
		[]byte("package main\n"),
		0o644,
	))

	// Run in-place conversion
	converter, err := NewConverter()
	require.NoError(t, err)

	err = converter.ConvertInPlace(context.Background(), srcDir)
	require.NoError(t, err)

	// Verify tag.template.json was created in the same directory
	tagConfigPath := filepath.Join(srcDir, "tag.template.json")
	_, err = os.Stat(tagConfigPath)
	require.NoError(t, err)

	tagConfig, err := os.ReadFile(tagConfigPath)
	require.NoError(t, err)
	assert.Contains(t, string(tagConfig), "project_name")
	assert.Contains(t, string(tagConfig), "author")
	assert.Contains(t, string(tagConfig), "use_docker")

	// Verify template directory was renamed from {{ cookiecutter.* }} to {{ vars.* }}
	renamedDir := filepath.Join(srcDir, "{{ vars.project_name }}")
	_, err = os.Stat(renamedDir)
	require.NoError(t, err)

	// Verify cookiecutter.json is still there
	_, err = os.Stat(filepath.Join(srcDir, "cookiecutter.json"))
	require.NoError(t, err)
}

func TestUT_ConvertInPlace_MissingCookiecutterJSON(t *testing.T) {
	srcDir := t.TempDir()
	// Don't create cookiecutter.json

	converter, err := NewConverter()
	require.NoError(t, err)

	err = converter.ConvertInPlace(context.Background(), srcDir)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoCookiecutterConfig)
}

func TestUT_ConvertInPlace_WithHooks(t *testing.T) {
	srcDir := t.TempDir()

	// Create cookiecutter.json
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "cookiecutter.json"), []byte(`{"name": "test"}`), 0o644))

	// Create hooks
	hooksDir := filepath.Join(srcDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "pre_gen_project.sh"), []byte("#!/bin/bash\necho pre"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "post_gen_project.sh"), []byte("#!/bin/bash\necho post"), 0o755))

	converter, err := NewConverter()
	require.NoError(t, err)

	err = converter.ConvertInPlace(context.Background(), srcDir)
	require.NoError(t, err)

	// Verify tag.template.json was created with hooks configuration
	tagConfigPath := filepath.Join(srcDir, "tag.template.json")
	tagConfig, err := os.ReadFile(tagConfigPath)
	require.NoError(t, err)

	// Should contain hooks configuration
	assert.Contains(t, string(tagConfig), "hooks")
	assert.Contains(t, string(tagConfig), "pre_scaffold")
	assert.Contains(t, string(tagConfig), "post_scaffold")

	// Original hooks should still be in place (not copied anywhere)
	_, err = os.Stat(filepath.Join(hooksDir, "pre_gen_project.sh"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(hooksDir, "post_gen_project.sh"))
	require.NoError(t, err)
}

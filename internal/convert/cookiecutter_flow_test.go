package convert

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_Convert_EmptyDestination_DefaultsToBaseName(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	// Create source dir with cookiecutter- prefix to test default naming
	parentDir := t.TempDir()
	srcDir := filepath.Join(parentDir, "cookiecutter-myapp")
	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"name": "test"}`),
		0o644,
	))

	result, err := converter.Convert(context.Background(), Options{
		Source: srcDir,
		DryRun: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "myapp-tag", result.Destination)
}

func TestUT_Convert_InvalidCookiecutterJSON(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{invalid json}`),
		0o644,
	))

	_, err = converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: filepath.Join(t.TempDir(), "out"),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidConfig)
}

func TestUT_Convert_WithHooksAndContent(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	srcDir := t.TempDir()

	// Create cookiecutter.json with multiple variables
	ccConfig := `{
		"project_name": "myproject",
		"description": "A project",
		"version": "0.1.0",
		"license": ["MIT", "BSD-3", "Apache-2.0"]
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(ccConfig),
		0o644,
	))

	// Create hooks directory with pre and post hooks
	hooksDir := filepath.Join(srcDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "pre_gen_project.sh"),
		[]byte("#!/bin/bash\necho pre"),
		0o755,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "post_gen_project.py"),
		[]byte("#!/usr/bin/env python\nprint('post')"),
		0o755,
	))

	// Create template files
	projDir := filepath.Join(srcDir, "{{ cookiecutter.project_name }}")
	require.NoError(t, os.MkdirAll(projDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(projDir, "README.md"),
		[]byte("# {{ cookiecutter.project_name }}\n{{ cookiecutter.description }}"),
		0o644,
	))

	destDir := filepath.Join(t.TempDir(), "output")
	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})
	require.NoError(t, err)
	assert.Equal(t, 4, result.VariablesConverted)
	assert.Equal(t, 2, result.HooksCopied)
	assert.Equal(t, 1, result.DirsRenamed) // cookiecutter project_name placeholder

	// Verify hooks were copied
	_, err = os.Stat(filepath.Join(destDir, "hooks", "pre_gen_project.sh"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(destDir, "hooks", "post_gen_project.py"))
	require.NoError(t, err)

	// Verify converted content references vars.* not cookiecutter.*
	readme, err := os.ReadFile(filepath.Join(destDir, "{{ vars.project_name }}", "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "vars.project_name")
	assert.NotContains(t, string(readme), "cookiecutter.project_name")
}

func TestUT_Convert_SkipsGitDir(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"name": "test"}`),
		0o644,
	))

	// Create a .git directory (should be skipped)
	gitDir := filepath.Join(srcDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0o644))

	// Create a regular file
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0o644))

	destDir := filepath.Join(t.TempDir(), "output")
	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.FilesProcessed) // Only main.go, not .git/config

	// .git should not be copied
	_, err = os.Stat(filepath.Join(destDir, ".git"))
	assert.True(t, os.IsNotExist(err))
}

func TestUT_Convert_BinaryFilesCopied(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"name": "test"}`),
		0o644,
	))

	// Create a "binary" file (non-text content)
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE}
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "icon.png"), binaryContent, 0o644))

	destDir := filepath.Join(t.TempDir(), "output")
	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.FilesProcessed)

	// Binary file should be copied as-is
	copied, err := os.ReadFile(filepath.Join(destDir, "icon.png"))
	require.NoError(t, err)
	assert.Equal(t, binaryContent, copied)
}

func TestUT_Convert_DryRun_WithHooks(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, types.CookiecutterConfigFile),
		[]byte(`{"name": "test"}`),
		0o644,
	))

	hooksDir := filepath.Join(srcDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "post_gen_project.sh"),
		[]byte("#!/bin/bash"),
		0o755,
	))

	destDir := filepath.Join(t.TempDir(), "dry-output")
	result, err := converter.Convert(context.Background(), Options{
		Source:      srcDir,
		Destination: destDir,
		DryRun:      true,
	})
	require.NoError(t, err)
	assert.True(t, result.DryRun)

	// Verify nothing was written
	_, err = os.Stat(destDir)
	assert.True(t, os.IsNotExist(err))
}

func TestUT_ResolveSource_LocalPath(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	dir := t.TempDir()
	resolved, err := converter.resolveSource(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, dir, resolved)
}

func TestUT_ResolveSource_NonexistentRemote(t *testing.T) {
	t.Parallel()
	converter, err := NewConverter()
	require.NoError(t, err)

	// Invalid remote reference
	_, err = converter.resolveSource(context.Background(), "gh:nonexistent/repo-that-does-not-exist-12345")
	// This will fail during resolution (network or cache miss)
	// The important thing is it doesn't panic
	if err != nil {
		assert.Contains(t, err.Error(), "failed to resolve template")
	}
}

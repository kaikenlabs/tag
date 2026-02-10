package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_LoadConfigFile_WithTemplateOrigin(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"template": {
			"source": "gh:acme/nextjs-starter",
			"name": "nextjs-starter",
			"version": "1.2.0"
		},
		"variables": {
			"project_name": "my-app",
			"use_docker": true,
			"port": 8080
		},
		"env": {
			"TAG_PATH": ".tag",
			"TAG_SHARED_PATH": "_shared",
			"TAG_BUNDLE_PATH": "_bundles"
		},
		"hooks": {
			"pre": [],
			"post": []
		}
	}`
	err := os.WriteFile(filepath.Join(dir, File), []byte(content), 0o644)
	require.NoError(t, err)

	cfg, err := LoadConfigFile(dir)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Template origin
	require.NotNil(t, cfg.Template)
	assert.Equal(t, "gh:acme/nextjs-starter", cfg.Template.Source)
	assert.Equal(t, "nextjs-starter", cfg.Template.Name)
	assert.Equal(t, "1.2.0", cfg.Template.Version)

	// Variables
	assert.Equal(t, "my-app", cfg.Variables["project_name"])
	assert.Equal(t, true, cfg.Variables["use_docker"])
	assert.Equal(t, float64(8080), cfg.Variables["port"]) // JSON numbers → float64

	// Env (unchanged)
	assert.Equal(t, ".tag", cfg.Env.Path)
	assert.Equal(t, "_shared", cfg.Env.SharedPath)
	assert.Equal(t, "_bundles", cfg.Env.BundlePath)
}

func TestUT_LoadConfigFile_WithoutTemplateOrigin(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"env": {
			"TAG_PATH": ".tag",
			"TAG_SHARED_PATH": "_shared",
			"TAG_BUNDLE_PATH": "_bundles"
		},
		"hooks": {
			"pre": [],
			"post": []
		}
	}`
	err := os.WriteFile(filepath.Join(dir, File), []byte(content), 0o644)
	require.NoError(t, err)

	cfg, err := LoadConfigFile(dir)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Nil(t, cfg.Template)
	assert.Nil(t, cfg.Variables)
	assert.Equal(t, ".tag", cfg.Env.Path)
}

func TestUT_LoadConfigFile_TemplateOriginWithoutVersion(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"template": {
			"source": "gh:acme/repo",
			"name": "repo"
		},
		"env": {"TAG_PATH": ".tag", "TAG_SHARED_PATH": "_shared", "TAG_BUNDLE_PATH": "_bundles"},
		"hooks": {"pre": [], "post": []}
	}`
	err := os.WriteFile(filepath.Join(dir, File), []byte(content), 0o644)
	require.NoError(t, err)

	cfg, err := LoadConfigFile(dir)
	require.NoError(t, err)
	require.NotNil(t, cfg.Template)
	assert.Equal(t, "", cfg.Template.Version)
	assert.Equal(t, "repo", cfg.Template.Name)
}

func TestUT_LoadConfigFile_MissingFile(t *testing.T) {
	dir := t.TempDir()

	cfg, err := LoadConfigFile(dir)
	assert.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestUT_LoadConfigFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, File), []byte(`{invalid`), 0o644)
	require.NoError(t, err)

	cfg, err := LoadConfigFile(dir)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "cannot parse config file")
}

func TestUT_HasTemplateOrigin_WithOrigin(t *testing.T) {
	cfg := &Config{
		Template: &TemplateOrigin{
			Source: "gh:acme/repo",
			Name:   "repo",
		},
	}
	assert.True(t, cfg.HasTemplateOrigin())
}

func TestUT_HasTemplateOrigin_NilTemplate(t *testing.T) {
	cfg := &Config{}
	assert.False(t, cfg.HasTemplateOrigin())
}

func TestUT_HasTemplateOrigin_EmptyName(t *testing.T) {
	cfg := &Config{
		Template: &TemplateOrigin{
			Source: "gh:acme/repo",
			Name:   "",
		},
	}
	assert.False(t, cfg.HasTemplateOrigin())
}

func TestUT_HasTemplateOrigin_NilConfig(t *testing.T) {
	var cfg *Config
	assert.False(t, cfg.HasTemplateOrigin())
}

func TestUT_CheckConfig_NilConfig(t *testing.T) {
	err := CheckConfig(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init")
}

func TestUT_CheckConfig_ValidConfig(t *testing.T) {
	cfg := &Config{}
	err := CheckConfig(cfg)
	require.NoError(t, err)
}

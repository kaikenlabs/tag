package dialect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_LoadDefaults_ReturnsAllBuiltins(t *testing.T) {
	t.Parallel()
	reg, err := LoadDefaults()
	require.NoError(t, err)

	names := reg.Names()
	assert.Len(t, names, 6)
	assert.Contains(t, names, "go")
	assert.Contains(t, names, "postgres")
	assert.Contains(t, names, "mysql")
	assert.Contains(t, names, "typescript")
	assert.Contains(t, names, "openapi")
	assert.Contains(t, names, "protobuf")
}

func TestUT_LoadDefaults_AllDialectsHaveTypes(t *testing.T) {
	t.Parallel()
	reg, err := LoadDefaults()
	require.NoError(t, err)

	for _, name := range reg.Names() {
		d := reg.Get(name)
		require.NotNil(t, d, "dialect %s should exist", name)
		assert.NotEmpty(t, d.Types, "dialect %s should have types", name)
	}
}

func TestUT_LoadWithOverrides_UserDirOnly(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "go.yaml"), []byte(`
name: go
types:
  string: CustomString
`), 0o644))

	reg, err := LoadWithOverrides(userDir, "")
	require.NoError(t, err)

	goDialect := reg.Get("go")
	require.NotNil(t, goDialect)
	assert.Equal(t, "CustomString", goDialect.Types["string"])
	// Built-in types not overridden should remain
	assert.Equal(t, "time.Time", goDialect.Types["datetime"])
}

func TestUT_LoadWithOverrides_TemplateDirOnly(t *testing.T) {
	t.Parallel()
	templateDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "postgres.yaml"), []byte(`
name: postgres
types:
  uuid: MY_UUID_TYPE
`), 0o644))

	reg, err := LoadWithOverrides("", templateDir)
	require.NoError(t, err)

	pg := reg.Get("postgres")
	require.NotNil(t, pg)
	assert.Equal(t, "MY_UUID_TYPE", pg.Types["uuid"])
	assert.Equal(t, "TEXT", pg.Types["string"]) // Built-in untouched
}

func TestUT_LoadWithOverrides_TemplateDirOverridesUserDir(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "go.yaml"), []byte(`
name: go
types:
  string: UserString
`), 0o644))

	templateDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "go.yaml"), []byte(`
name: go
types:
  string: TemplateString
`), 0o644))

	reg, err := LoadWithOverrides(userDir, templateDir)
	require.NoError(t, err)

	goDialect := reg.Get("go")
	require.NotNil(t, goDialect)
	assert.Equal(t, "TemplateString", goDialect.Types["string"],
		"template-local should override user-global")
}

func TestUT_LoadWithOverrides_NonexistentDirs(t *testing.T) {
	t.Parallel()
	reg, err := LoadWithOverrides("/nonexistent/user/dir", "/nonexistent/template/dir")
	require.NoError(t, err, "nonexistent dirs should be silently skipped")
	assert.Len(t, reg.Names(), 6, "should still have all built-ins")
}

func TestUT_LoadWithOverrides_BothEmpty(t *testing.T) {
	t.Parallel()
	reg, err := LoadWithOverrides("", "")
	require.NoError(t, err)
	assert.Len(t, reg.Names(), 6)
}

func TestUT_LoadWithOverrides_InvalidYAML_UserDir(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "bad.yaml"), []byte("{invalid yaml"), 0o644))

	_, err := LoadWithOverrides(userDir, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load user-global dialects")
}

func TestUT_LoadWithOverrides_InvalidYAML_TemplateDir(t *testing.T) {
	t.Parallel()
	templateDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "bad.yaml"), []byte("{invalid yaml"), 0o644))

	_, err := LoadWithOverrides("", templateDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load template-local dialects")
}

func TestUT_UserDialectsDir_ReturnsPath(t *testing.T) {
	t.Parallel()
	dir, err := UserDialectsDir()
	require.NoError(t, err)
	assert.Contains(t, dir, "dialects")
}

func TestUT_LoadForTemplate_WithDirName(t *testing.T) {
	t.Parallel()
	templateRoot := t.TempDir()
	dialectsDir := filepath.Join(templateRoot, "_dialects")
	require.NoError(t, os.MkdirAll(dialectsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dialectsDir, "go.yaml"), []byte(`
name: go
types:
  string: TemplateLocalString
`), 0o644))

	reg, err := LoadForTemplate(templateRoot, "_dialects")
	require.NoError(t, err)

	goDialect := reg.Get("go")
	require.NotNil(t, goDialect)
	assert.Equal(t, "TemplateLocalString", goDialect.Types["string"])
}

func TestUT_LoadWithOverrides_NewDialectInUserDir(t *testing.T) {
	t.Parallel()
	userDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "custom.yaml"), []byte(`
name: custom
description: Custom dialect
types:
  string: CUSTOM_STRING
  int: CUSTOM_INT
`), 0o644))

	reg, err := LoadWithOverrides(userDir, "")
	require.NoError(t, err)

	// All built-ins plus the new custom dialect
	assert.Len(t, reg.Names(), 7)
	custom := reg.Get("custom")
	require.NotNil(t, custom)
	assert.Equal(t, "CUSTOM_STRING", custom.Types["string"])
}

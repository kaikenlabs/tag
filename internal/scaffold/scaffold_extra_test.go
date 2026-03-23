package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_ResolveOutputDir_FromProjectName(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	vars := map[string]any{"project_name": "my-project"}

	dir, err := resolveOutputDir("", vars, cwd)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cwd, "my-project"), dir)
}

func TestUT_ResolveOutputDir_ExplicitOutput(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	vars := map[string]any{}

	dir, err := resolveOutputDir("custom-dir", vars, cwd)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cwd, "custom-dir"), dir)
}

func TestUT_ResolveOutputDir_AbsoluteOutput(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	absDir := t.TempDir()

	dir, err := resolveOutputDir(absDir, map[string]any{}, cwd)
	require.NoError(t, err)
	assert.Equal(t, absDir, dir)
}

func TestUT_ResolveOutputDir_NoOutputNoProjectName(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()

	_, err := resolveOutputDir("", map[string]any{}, cwd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "output directory not specified")
}

func TestUT_ResolveOutputDir_PathTraversal(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()

	_, err := resolveOutputDir("../../escape", map[string]any{}, cwd)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "escapes working directory")
}

func TestUT_PrepareOutputDir_OutputNotExist(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "new-project")

	err := prepareOutputDir(dir, false)
	assert.NoError(t, err)
}

func TestUT_PrepareOutputDir_OutputExists_NoForce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	err := prepareOutputDir(dir, false)
	assert.ErrorIs(t, err, ErrOutputExists)
}

func TestUT_PrepareOutputDir_OutputExists_WithForce(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub", "project")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	err := prepareOutputDir(subDir, true)
	assert.NoError(t, err)

	// Directory should be removed
	_, statErr := os.Stat(subDir)
	assert.True(t, os.IsNotExist(statErr))
}

func TestUT_ValidateSafeOutputDir_RootPath(t *testing.T) {
	t.Parallel()
	err := validateSafeOutputDir("/")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "root directory")
}

func TestUT_ValidateSafeOutputDir_DangerousPaths(t *testing.T) {
	t.Parallel()
	for _, dp := range dangerousPaths {
		t.Run(dp, func(t *testing.T) {
			t.Parallel()
			err := validateSafeOutputDir(dp)
			assert.Error(t, err)
		})
	}
}

func TestUT_ValidateSafeOutputDir_TooShallow(t *testing.T) {
	t.Parallel()
	err := validateSafeOutputDir("/a")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too shallow")
}

func TestUT_ValidateSafeOutputDir_ValidPath(t *testing.T) {
	t.Parallel()
	err := validateSafeOutputDir("/home/user/projects/my-project")
	assert.NoError(t, err)
}

func TestUT_FindProjectWrapper_SingleWrapper(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "{{ vars.project_name }}"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte("{}"), 0o644))

	wrapper := findProjectWrapper(dir)
	assert.Equal(t, "{{ vars.project_name }}", wrapper)
}

func TestUT_FindProjectWrapper_MultipleWrappers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "{{ vars.a }}"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "{{ vars.b }}"), 0o755))

	wrapper := findProjectWrapper(dir)
	assert.Empty(t, wrapper, "multiple template dirs should not unwrap")
}

func TestUT_FindProjectWrapper_NoWrapper(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "src"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "docs"), 0o755))

	wrapper := findProjectWrapper(dir)
	assert.Empty(t, wrapper)
}

func TestUT_FindProjectWrapper_NonexistentDir(t *testing.T) {
	t.Parallel()
	wrapper := findProjectWrapper("/nonexistent/dir/12345")
	assert.Empty(t, wrapper)
}

func TestUT_SecretKeys_Empty(t *testing.T) {
	t.Parallel()
	keys := secretKeys(nil)
	assert.Empty(t, keys)
}

func TestUT_SecretKeys_WithSecrets(t *testing.T) {
	t.Parallel()
	defs := map[string]VariableDef{
		"name":     {Type: "string"},
		"api_key":  {Type: "string", Secret: true},
		"password": {Type: "string", Secret: true},
		"port":     {Type: "number"},
	}

	keys := secretKeys(defs)
	assert.Len(t, keys, 2)
	assert.True(t, keys["api_key"])
	assert.True(t, keys["password"])
	assert.False(t, keys["name"])
}

func TestUT_RenderHookCommands_Empty(t *testing.T) {
	t.Parallel()
	engine, err := template.NewEngine()
	require.NoError(t, err)

	result, err := renderHookCommands(engine, nil, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestUT_RenderHookCommands_NoTemplateExpression(t *testing.T) {
	t.Parallel()
	engine, err := template.NewEngine()
	require.NoError(t, err)

	cmds := []string{"echo hello", "make build"}
	result, err := renderHookCommands(engine, cmds, nil)
	require.NoError(t, err)
	assert.Equal(t, cmds, result)
}

func TestUT_RenderHookCommands_WithTemplateExpression(t *testing.T) {
	t.Parallel()
	engine, err := template.NewEngine()
	require.NoError(t, err)

	cmds := []string{"echo {{ vars.name }}"}
	vars := map[string]any{"name": "world"}
	result, err := renderHookCommands(engine, cmds, vars)
	require.NoError(t, err)
	assert.Equal(t, []string{"echo world"}, result)
}

func TestUT_RenderHooksConfig_Nil(t *testing.T) {
	t.Parallel()
	engine, err := template.NewEngine()
	require.NoError(t, err)

	result, err := renderHooksConfig(engine, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestUT_RenderHooksConfig_WithHooks(t *testing.T) {
	t.Parallel()
	engine, err := template.NewEngine()
	require.NoError(t, err)

	hc := &types.HooksConfig{
		PreScaffold:  []string{"echo pre"},
		PostScaffold: []string{"echo post"},
	}

	result, err := renderHooksConfig(engine, hc, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"echo pre"}, result.PreScaffold)
	assert.Equal(t, []string{"echo post"}, result.PostScaffold)
}

func TestUT_SaveReplayData_NoSave(t *testing.T) {
	t.Parallel()
	var buf safeBuffer
	opts := Options{NoSave: true, TemplateRef: "gh:user/repo"}
	config := &TemplateConfig{}
	vars := map[string]any{"x": 1}

	// Should not panic or error
	saveReplayData(&buf, opts, config, vars)
}

func TestUT_SaveReplayData_NoTemplateRef(t *testing.T) {
	t.Parallel()
	var buf safeBuffer
	opts := Options{TemplateRef: ""}
	config := &TemplateConfig{}
	vars := map[string]any{"x": 1}

	saveReplayData(&buf, opts, config, vars)
}

// safeBuffer is a simple thread-safe writer for tests.
type safeBuffer struct {
	data []byte
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

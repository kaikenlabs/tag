package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_GenerateInfo_Generator(t *testing.T) {
	t.Parallel()

	tmpDir := setupTempDir(t)

	// Create generator with tag.template.json, template file with frontmatter, and requires.
	genDir := filepath.Join(tmpDir, "api-endpoint")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	tcJSON := `{
  "description": "REST endpoint with handler and tests",
  "requires": ["use_rest_api"],
  "vars": {
    "method": {"type": "choice", "prompt": "HTTP method", "default": "GET", "options": ["GET", "POST", "PUT", "DELETE"]},
    "auth": {"type": "boolean", "prompt": "Require authentication?", "default": true}
  },
  "hooks": {"post_scaffold": ["go fmt ./..."]}
}`
	require.NoError(t, os.WriteFile(filepath.Join(genDir, types.TemplateConfigFile), []byte(tcJSON), 0o644))

	tmplContent := "---\nto: internal/handlers/{{ name | snake }}.go\n---\npackage handlers\n"
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "handler.go"), []byte(tmplContent), 0o644))

	injectContent := "---\nto: internal/routes/router.go\ninject: true\nafter: // routes\n---\nrouter.Handle()\n"
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "route.go"), []byte(injectContent), 0o644))

	cfg := createTestConfig(t, tmpDir)
	var buf bytes.Buffer
	err := generateInfo(cfg, "api-endpoint", &buf)
	require.NoError(t, err)

	var info generatorInfoJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &info))

	assert.Equal(t, "api-endpoint", info.Name)
	assert.Equal(t, "generator", info.Type)
	assert.Equal(t, "REST endpoint with handler and tests", info.Description)
	assert.Equal(t, "local", info.Source)
	assert.Equal(t, []string{"use_rest_api"}, info.Requires)
	assert.Len(t, info.Variables, 2)
	assert.Equal(t, "choice", info.Variables["method"].Type)
	assert.Equal(t, []string{"GET", "POST", "PUT", "DELETE"}, info.Variables["method"].Options)
	assert.Equal(t, "boolean", info.Variables["auth"].Type)
	require.NotNil(t, info.Hooks)
	assert.Equal(t, []string{"go fmt ./..."}, info.Hooks.PostScaffold)
	assert.GreaterOrEqual(t, len(info.Files), 2)
	assert.Contains(t, info.Usage, "tag generate")
	assert.Contains(t, info.Usage, "api-endpoint <name>")

	// Check file actions.
	var hasCreate, hasInject bool
	for _, f := range info.Files {
		switch f.Action {
		case actionCreate:
			hasCreate = true
			assert.Contains(t, f.To, "handlers")
		case actionInject:
			hasInject = true
			assert.Equal(t, "// routes", f.After)
		}
	}
	assert.True(t, hasCreate, "expected a create action")
	assert.True(t, hasInject, "expected an inject action")
}

func TestUT_GenerateInfo_Bundle(t *testing.T) {
	t.Parallel()

	tmpDir := setupTempDir(t)

	bundleJSON := `{
  "name": "crud",
  "description": "Full CRUD stack",
  "generators": [{"name": "model"}, {"name": "repository"}, {"name": "service"}]
}`
	createBundle(t, tmpDir, "crud", bundleJSON)

	cfg := createTestConfig(t, tmpDir)
	var buf bytes.Buffer
	err := generateInfo(cfg, "crud", &buf)
	require.NoError(t, err)

	var info generatorInfoJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &info))

	assert.Equal(t, "crud", info.Name)
	assert.Equal(t, "bundle", info.Type)
	assert.Equal(t, "Full CRUD stack", info.Description)
	assert.Equal(t, []string{"model", "repository", "service"}, info.Generators)
	assert.Nil(t, info.SelfContained)
	assert.Contains(t, info.Usage, "tag generate")
	assert.Contains(t, info.Usage, "crud <name>")
}

func TestUT_GenerateInfo_GeneratorNoConfig(t *testing.T) {
	t.Parallel()

	tmpDir := setupTempDir(t)

	// Create generator with only a template file (no tag.template.json).
	tmplContent := "---\nto: {{ name }}.go\ndesc: A simple generator\n---\npackage main\n"
	createGenerator(t, tmpDir, "simple", tmplContent)

	cfg := createTestConfig(t, tmpDir)
	var buf bytes.Buffer
	err := generateInfo(cfg, "simple", &buf)
	require.NoError(t, err)

	var info generatorInfoJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &info))

	assert.Equal(t, "simple", info.Name)
	assert.Equal(t, "generator", info.Type)
	assert.Equal(t, "A simple generator", info.Description)
	assert.Empty(t, info.Variables)
	assert.Nil(t, info.Hooks)
}

func TestUT_GenerateInfo_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	var buf bytes.Buffer
	err := generateInfo(cfg, "nonexistent", &buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestUT_ExtractFileInfos(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	genDir := filepath.Join(tmpDir, "mygen")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	// Create template — creates a file.
	require.NoError(t, os.WriteFile(
		filepath.Join(genDir, "create.go"),
		[]byte("---\nto: output/{{ name }}.go\n---\npackage main\n"),
		0o644,
	))

	// Append template.
	require.NoError(t, os.WriteFile(
		filepath.Join(genDir, "append.go"),
		[]byte("---\nto: output/routes.go\nappend: true\n---\nroute\n"),
		0o644,
	))

	// Inject template.
	require.NoError(t, os.WriteFile(
		filepath.Join(genDir, "inject.go"),
		[]byte("---\nto: output/main.go\ninject: true\nbefore: // end\n---\ncode\n"),
		0o644,
	))

	files, err := extractFileInfos(genDir)
	require.NoError(t, err)
	require.Len(t, files, 3)

	actions := map[string]bool{}
	for _, f := range files {
		actions[f.Action] = true
		if f.Action == actionInject {
			assert.Equal(t, "// end", f.Before)
		}
	}
	assert.True(t, actions[actionCreate])
	assert.True(t, actions[actionAppend])
	assert.True(t, actions[actionInject])
}

func TestUT_ConvertVariables(t *testing.T) {
	t.Parallel()

	vars := map[string]tmplconfig.VariableDef{
		"name": {
			Type:    tmplconfig.VarTypeString,
			Prompt:  "Name?",
			Default: "default",
		},
		"count": {
			Type:     tmplconfig.VarTypeNumber,
			Required: true,
			Default:  float64(42),
		},
		"flag": {
			Type:    tmplconfig.VarTypeBoolean,
			Default: true,
		},
		"env": {
			Type:    tmplconfig.VarTypeChoice,
			Options: []string{"dev", "prod"},
			Secret:  true,
		},
	}

	result := convertVariables(vars)
	require.Len(t, result, 4)

	assert.Equal(t, "string", result["name"].Type)
	assert.Equal(t, "Name?", result["name"].Prompt)
	assert.Equal(t, "default", result["name"].Default)

	assert.Equal(t, "number", result["count"].Type)
	assert.True(t, result["count"].Required)
	assert.Equal(t, float64(42), result["count"].Default)

	assert.Equal(t, "boolean", result["flag"].Type)
	assert.Equal(t, true, result["flag"].Default)

	assert.Equal(t, "choice", result["env"].Type)
	assert.Equal(t, []string{"dev", "prod"}, result["env"].Options)
	assert.True(t, result["env"].Secret)
}

func TestUT_ConvertVariables_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, convertVariables(nil))
}

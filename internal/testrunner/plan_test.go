package testrunner_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/testrunner"
)

func writeTemplateConfig(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(content), 0o644))
}

func TestUT_Plan_BasicConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"use_postgres": {"type": "boolean", "default": true},
			"use_amqp": {"type": "boolean", "default": false},
			"module_path": {"type": "string", "prompt": "Module path"}
		}
	}`)

	plan, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		MaxCases:    0,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"use_amqp", "use_postgres"}, plan.BoolVars)
	assert.Len(t, plan.Combos, 4)
	assert.Empty(t, plan.Commands)
	assert.Equal(t, "test-scaffold", plan.ProjectName)
}

func TestUT_Plan_WithTestConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"use_x": {"type": "boolean", "default": true}
		},
		"test": {
			"commands": ["go build ./..."],
			"project_name": "my-project",
			"env": {"CGO_ENABLED": "0"}
		}
	}`)

	plan, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		AcceptHooks: true,
		MaxCases:    0,
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"go build ./..."}, plan.Commands)
	assert.Equal(t, "my-project", plan.ProjectName)
	assert.Equal(t, map[string]string{"CGO_ENABLED": "0"}, plan.Env)
}

func TestUT_Plan_TemplateCommandsRequireAcceptHooks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"use_x": {"type": "boolean", "default": true}
		},
		"test": {
			"commands": ["go build ./..."]
		}
	}`)

	_, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		AcceptHooks: false,
		MaxCases:    0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--accept-hooks")
}

func TestUT_Plan_RunCommandsOverrideTemplateCommands(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"use_x": {"type": "boolean", "default": true}
		},
		"test": {
			"commands": ["go build ./..."]
		}
	}`)

	plan, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		RunCommands: []string{"echo override"},
		AcceptHooks: false,
		MaxCases:    0,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"echo override"}, plan.Commands)
}

func TestUT_Plan_MaxCasesZeroIsUnlimited(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"a": {"type": "boolean", "default": true},
			"b": {"type": "boolean", "default": true},
			"c": {"type": "boolean", "default": true},
			"d": {"type": "boolean", "default": true},
			"e": {"type": "boolean", "default": true},
			"f": {"type": "boolean", "default": true},
			"g": {"type": "boolean", "default": true}
		}
	}`)

	// 7 boolean vars = 128 combinations, exceeds default limit of 64.
	plan, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		MaxCases:    0,
	})
	require.NoError(t, err)
	assert.Len(t, plan.Combos, 128)
}

func TestUT_Plan_MaxCasesEnforced(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"a": {"type": "boolean", "default": true},
			"b": {"type": "boolean", "default": true},
			"c": {"type": "boolean", "default": true},
			"d": {"type": "boolean", "default": true},
			"e": {"type": "boolean", "default": true},
			"f": {"type": "boolean", "default": true},
			"g": {"type": "boolean", "default": true}
		}
	}`)

	// 128 combos > 64 max-cases limit.
	_, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		MaxCases:    64,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds safety limit")
	assert.Contains(t, err.Error(), "--max-cases 0")
}

func TestUT_Plan_SkipVarsReducesCombinations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"a": {"type": "boolean", "default": true},
			"b": {"type": "boolean", "default": true},
			"c": {"type": "boolean", "default": true}
		}
	}`)

	plan, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		SkipVars:    []string{"c"},
		MaxCases:    0,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, plan.BoolVars)
	assert.Len(t, plan.Combos, 4)
}

func TestUT_Plan_PinVarsReducesCombinations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"a": {"type": "boolean", "default": true},
			"b": {"type": "boolean", "default": true},
			"c": {"type": "boolean", "default": true}
		}
	}`)

	plan, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		PinVars:     map[string]string{"b": "true"},
		MaxCases:    0,
	})
	require.NoError(t, err)
	assert.Len(t, plan.Combos, 4)

	// All combos should have b=true.
	for _, c := range plan.Combos {
		assert.Equal(t, "true", c.Vars["b"])
	}
}

func TestUT_Plan_FilterApplied(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"a": {"type": "boolean", "default": true},
			"b": {"type": "boolean", "default": true}
		}
	}`)

	plan, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		Filter:      "a=true",
		MaxCases:    0,
	})
	require.NoError(t, err)
	assert.Len(t, plan.Combos, 2)
}

func TestUT_Plan_InvalidFilter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"a": {"type": "boolean", "default": true}
		}
	}`)

	_, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		Filter:      "noequals",
		MaxCases:    0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid filter")
}

func TestUT_Plan_MissingConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		Timeout:     time.Second,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read template config")
}

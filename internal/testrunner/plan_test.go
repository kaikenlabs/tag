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
	require.Len(t, plan.Cases, 1)
	assert.Len(t, plan.Cases[0].Combos, 4)
	assert.Empty(t, plan.Cases[0].Commands)
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
			"cases": [{"name": "build", "commands": ["go build ./..."]}],
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

	require.Len(t, plan.Cases, 1)
	assert.Equal(t, []string{"go build ./..."}, plan.Cases[0].Commands)
	assert.Equal(t, "build", plan.Cases[0].Name)
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
			"cases": [{"name": "build", "commands": ["go build ./..."]}]
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
			"cases": [{"name": "build", "commands": ["go build ./..."]}]
		}
	}`)

	plan, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		RunCommands: []string{"echo override"},
		AcceptHooks: false,
		MaxCases:    0,
	})
	require.NoError(t, err)
	require.Len(t, plan.Cases, 1)
	assert.Equal(t, []string{"echo override"}, plan.Cases[0].Commands)
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
	require.Len(t, plan.Cases, 1)
	assert.Len(t, plan.Cases[0].Combos, 128)
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
	require.Len(t, plan.Cases, 1)
	assert.Len(t, plan.Cases[0].Combos, 4)
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
	require.Len(t, plan.Cases, 1)
	assert.Len(t, plan.Cases[0].Combos, 4)

	// All combos should have b=true.
	for _, c := range plan.Cases[0].Combos {
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
	require.Len(t, plan.Cases, 1)
	assert.Len(t, plan.Cases[0].Combos, 2)
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

func TestUT_Plan_MultipleCases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"use_docker": {"type": "boolean", "default": true},
			"use_postgres": {"type": "boolean", "default": true}
		},
		"test": {
			"cases": [
				{
					"name": "Full test",
					"filters": {"use_docker": true, "use_postgres": true},
					"commands": ["go build ./...", "go vet ./..."]
				},
				{
					"name": "Light test",
					"commands": ["go build ./..."]
				}
			]
		}
	}`)

	plan, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		AcceptHooks: true,
		MaxCases:    0,
	})
	require.NoError(t, err)

	require.Len(t, plan.Cases, 2)

	// Full test: both vars pinned, so only 1 combination.
	assert.Equal(t, "Full test", plan.Cases[0].Name)
	assert.Len(t, plan.Cases[0].Combos, 1)
	assert.Equal(t, []string{"go build ./...", "go vet ./..."}, plan.Cases[0].Commands)
	assert.Equal(t, "true", plan.Cases[0].Combos[0].Vars["use_docker"])
	assert.Equal(t, "true", plan.Cases[0].Combos[0].Vars["use_postgres"])

	// Light test: no filters, all 4 combinations.
	assert.Equal(t, "Light test", plan.Cases[1].Name)
	assert.Len(t, plan.Cases[1].Combos, 4)
	assert.Equal(t, []string{"go build ./..."}, plan.Cases[1].Commands)
}

func TestUT_Plan_CaseNameFilter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"a": {"type": "boolean", "default": true}
		},
		"test": {
			"cases": [
				{"name": "build", "commands": ["go build ./..."]},
				{"name": "lint", "commands": ["golangci-lint run"]}
			]
		}
	}`)

	plan, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		AcceptHooks: true,
		CaseName:    "lint",
		MaxCases:    0,
	})
	require.NoError(t, err)

	require.Len(t, plan.Cases, 1)
	assert.Equal(t, "lint", plan.Cases[0].Name)
	assert.Equal(t, []string{"golangci-lint run"}, plan.Cases[0].Commands)
}

func TestUT_Plan_CaseNameNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"a": {"type": "boolean", "default": true}
		},
		"test": {
			"cases": [
				{"name": "build", "commands": ["go build ./..."]}
			]
		}
	}`)

	_, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		AcceptHooks: true,
		CaseName:    "nonexistent",
		MaxCases:    0,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "build")
}

func TestUT_Plan_CaseFiltersReduceCombinations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTemplateConfig(t, dir, `{
		"vars": {
			"a": {"type": "boolean", "default": true},
			"b": {"type": "boolean", "default": true},
			"c": {"type": "boolean", "default": true}
		},
		"test": {
			"cases": [
				{
					"name": "pinned",
					"filters": {"a": true, "b": false},
					"commands": ["echo test"]
				}
			]
		}
	}`)

	plan, err := testrunner.Plan(testrunner.Config{
		TemplateDir: dir,
		AcceptHooks: true,
		MaxCases:    0,
	})
	require.NoError(t, err)

	require.Len(t, plan.Cases, 1)
	// a and b pinned, only c varies = 2 combinations.
	assert.Len(t, plan.Cases[0].Combos, 2)
	for _, combo := range plan.Cases[0].Combos {
		assert.Equal(t, "true", combo.Vars["a"])
		assert.Equal(t, "false", combo.Vars["b"])
	}
}

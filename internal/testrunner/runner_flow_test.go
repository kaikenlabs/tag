package testrunner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestTemplate creates a minimal scaffold template directory with
// tag.template.json and a simple file template. Returns the template dir path.
func setupTestTemplate(t *testing.T, vars map[string]any) string {
	t.Helper()
	dir := t.TempDir()

	// Build tag.template.json.
	tmplCfg := map[string]any{
		"name": "test-tmpl",
	}
	if vars != nil {
		tmplCfg["vars"] = vars
	}
	cfgData, err := json.Marshal(tmplCfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), cfgData, 0o644))

	// Create a wrapper-style template: {{vars.project_name}}/main.go
	projDir := filepath.Join(dir, "{{vars.project_name}}")
	require.NoError(t, os.MkdirAll(projDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "main.go"), []byte("package main\n"), 0o644))

	return dir
}

func TestUT_Execute_PassingCase(t *testing.T) {
	templateDir := setupTestTemplate(t, map[string]any{
		"use_db": map[string]any{
			"type":    "boolean",
			"default": false,
		},
	})

	cfg := Config{
		TemplateDir: templateDir,
		RunCommands: []string{"true"},
		AcceptHooks: true,
		Parallel:    1,
		MaxCases:    64,
	}

	plan, err := Plan(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, plan.Cases)

	report, err := Execute(context.Background(), plan, cfg)
	require.NoError(t, err)
	assert.Greater(t, report.Passed, 0)
	assert.Equal(t, 0, report.Failed)
	assert.Equal(t, 0, report.Errored)
}

func TestUT_Execute_FailingValidation(t *testing.T) {
	templateDir := setupTestTemplate(t, nil)

	cfg := Config{
		TemplateDir: templateDir,
		RunCommands: []string{"false"},
		AcceptHooks: true,
		Parallel:    1,
		MaxCases:    64,
	}

	plan, err := Plan(cfg)
	require.NoError(t, err)

	report, err := Execute(context.Background(), plan, cfg)
	require.NoError(t, err)
	assert.Greater(t, report.Failed, 0)
}

func TestUT_Execute_ContextCancellation(t *testing.T) {
	templateDir := setupTestTemplate(t, map[string]any{
		"a": map[string]any{"type": "boolean", "default": false},
		"b": map[string]any{"type": "boolean", "default": false},
		"c": map[string]any{"type": "boolean", "default": false},
	})

	cfg := Config{
		TemplateDir: templateDir,
		RunCommands: []string{"sleep 10"},
		AcceptHooks: true,
		Parallel:    1,
		MaxCases:    64,
		Timeout:     100 * time.Millisecond,
	}

	plan, err := Plan(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	report, err := Execute(ctx, plan, cfg)
	require.NoError(t, err)
	// With an already-cancelled context, no cases should run or they should error/fail.
	assert.Equal(t, 0, report.Passed)
}

func TestUT_Plan_WithBoolVars(t *testing.T) {
	templateDir := setupTestTemplate(t, map[string]any{
		"use_db": map[string]any{
			"type":    "boolean",
			"default": false,
		},
	})

	cfg := Config{
		TemplateDir: templateDir,
		RunCommands: []string{"true"},
		AcceptHooks: true,
		MaxCases:    64,
	}

	plan, err := Plan(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, plan.BoolVars)
	assert.Contains(t, plan.BoolVars, "use_db")

	// With 1 boolean var, expect 2 combinations (true/false).
	totalCombos := 0
	for _, cp := range plan.Cases {
		totalCombos += len(cp.Combos)
	}
	assert.Equal(t, 2, totalCombos)
}

func TestUT_Execute_NoCommands_ScaffoldOnly(t *testing.T) {
	templateDir := setupTestTemplate(t, nil)

	cfg := Config{
		TemplateDir: templateDir,
		AcceptHooks: true,
		Parallel:    1,
		MaxCases:    64,
	}

	plan, err := Plan(cfg)
	require.NoError(t, err)

	report, err := Execute(context.Background(), plan, cfg)
	require.NoError(t, err)
	// With no validation commands, scaffold-only runs should all pass.
	assert.Greater(t, report.Passed, 0)
	assert.Equal(t, 0, report.Failed)
	assert.Equal(t, 0, report.Errored)
}

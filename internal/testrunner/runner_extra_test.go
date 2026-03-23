package testrunner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

func TestUT_BuildCasePlans_CLIRunCommandsOverride(t *testing.T) {
	t.Parallel()

	testCfg := &tmplconfig.TestConfig{
		Cases: []tmplconfig.TestCase{
			{Name: "case1", Commands: []string{"echo template"}},
		},
	}

	cfg := Config{
		RunCommands: []string{"echo override"},
		AcceptHooks: true,
	}

	plans, err := buildCasePlans(cfg, testCfg, nil)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	assert.Equal(t, "default", plans[0].Name)
	assert.Equal(t, []string{"echo override"}, plans[0].Commands)
}

func TestUT_BuildCasePlans_NoCases_DefaultCreated(t *testing.T) {
	t.Parallel()

	plans, err := buildCasePlans(Config{}, nil, nil)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	assert.Equal(t, "default", plans[0].Name)
	assert.Empty(t, plans[0].Commands)
}

func TestUT_BuildCasePlans_TemplateCases_RequireAcceptHooks(t *testing.T) {
	t.Parallel()

	testCfg := &tmplconfig.TestConfig{
		Cases: []tmplconfig.TestCase{
			{Name: "case1", Commands: []string{"echo test"}},
		},
	}

	_, err := buildCasePlans(Config{AcceptHooks: false}, testCfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--accept-hooks")
}

func TestUT_BuildCasePlans_CaseNameFilter_Found(t *testing.T) {
	t.Parallel()

	testCfg := &tmplconfig.TestConfig{
		Cases: []tmplconfig.TestCase{
			{Name: "a", Commands: []string{"echo a"}},
			{Name: "b", Commands: []string{"echo b"}},
		},
	}

	plans, err := buildCasePlans(Config{CaseName: "b", AcceptHooks: true}, testCfg, nil)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	assert.Equal(t, "b", plans[0].Name)
}

func TestUT_BuildCasePlans_CaseNameFilter_NotFound(t *testing.T) {
	t.Parallel()

	testCfg := &tmplconfig.TestConfig{
		Cases: []tmplconfig.TestCase{
			{Name: "a"},
		},
	}

	_, err := buildCasePlans(Config{CaseName: "missing", AcceptHooks: true}, testCfg, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUT_BuildCasePlans_MaxCasesExceeded(t *testing.T) {
	t.Parallel()

	// 3 bool vars = 8 combinations
	plans, err := buildCasePlans(
		Config{MaxCases: 2},
		nil,
		[]string{"a", "b", "c"},
	)
	assert.Nil(t, plans)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds safety limit")
}

func TestUT_Plan_InvalidTemplateDir(t *testing.T) {
	t.Parallel()

	_, err := Plan(Config{TemplateDir: "/nonexistent/path"})
	require.Error(t, err)
}

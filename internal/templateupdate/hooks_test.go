package templateupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/tmplconfig"
	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_DetectHookChanges_Added(t *testing.T) {
	oldCfg := &tmplconfig.TemplateConfig{}
	newCfg := &tmplconfig.TemplateConfig{
		Hooks: &types.HooksConfig{
			PostScaffold: []string{"go mod tidy", "go generate ./..."},
		},
	}

	changes := DetectHookChanges(oldCfg, newCfg)
	require.Len(t, changes, 1)
	assert.Equal(t, "post_scaffold", changes[0].Phase)
	assert.Equal(t, HookAdded, changes[0].Type)
	assert.Equal(t, []string{"go mod tidy", "go generate ./..."}, changes[0].NewHooks)
	assert.Nil(t, changes[0].OldHooks)
}

func TestUT_DetectHookChanges_Removed(t *testing.T) {
	oldCfg := &tmplconfig.TemplateConfig{
		Hooks: &types.HooksConfig{
			PreScaffold: []string{"echo setup"},
		},
	}
	newCfg := &tmplconfig.TemplateConfig{}

	changes := DetectHookChanges(oldCfg, newCfg)
	require.Len(t, changes, 1)
	assert.Equal(t, "pre_scaffold", changes[0].Phase)
	assert.Equal(t, HookRemoved, changes[0].Type)
	assert.Equal(t, []string{"echo setup"}, changes[0].OldHooks)
	assert.Nil(t, changes[0].NewHooks)
}

func TestUT_DetectHookChanges_Modified(t *testing.T) {
	oldCfg := &tmplconfig.TemplateConfig{
		Hooks: &types.HooksConfig{
			PostScaffold: []string{"go mod tidy"},
		},
	}
	newCfg := &tmplconfig.TemplateConfig{
		Hooks: &types.HooksConfig{
			PostScaffold: []string{"go mod tidy", "go generate ./..."},
		},
	}

	changes := DetectHookChanges(oldCfg, newCfg)
	require.Len(t, changes, 1)
	assert.Equal(t, HookModified, changes[0].Type)
	assert.Equal(t, "post_scaffold", changes[0].Phase)
}

func TestUT_DetectHookChanges_NoChanges(t *testing.T) {
	cfg := &tmplconfig.TemplateConfig{
		Hooks: &types.HooksConfig{
			PreScaffold:  []string{"echo hello"},
			PostScaffold: []string{"go mod tidy"},
		},
	}

	changes := DetectHookChanges(cfg, cfg)
	assert.Empty(t, changes)
}

func TestUT_DetectHookChanges_NilConfigs(t *testing.T) {
	changes := DetectHookChanges(nil, nil)
	assert.Empty(t, changes)
}

func TestUT_DetectHookChanges_MultiPhase(t *testing.T) {
	oldCfg := &tmplconfig.TemplateConfig{
		Hooks: &types.HooksConfig{
			PreScaffold: []string{"old-pre"},
		},
	}
	newCfg := &tmplconfig.TemplateConfig{
		Hooks: &types.HooksConfig{
			PreScaffold:  []string{"new-pre"},
			PostScaffold: []string{"new-post"},
		},
	}

	changes := DetectHookChanges(oldCfg, newCfg)
	require.Len(t, changes, 2)
	// Sorted by phase: post_scaffold before pre_scaffold.
	assert.Equal(t, "post_scaffold", changes[0].Phase)
	assert.Equal(t, HookAdded, changes[0].Type)
	assert.Equal(t, "pre_scaffold", changes[1].Phase)
	assert.Equal(t, HookModified, changes[1].Type)
}

func TestUT_HasExecutableChanges(t *testing.T) {
	tests := []struct {
		name    string
		changes []HookChange
		want    bool
	}{
		{"empty", nil, false},
		{"removed only", []HookChange{{Type: HookRemoved}}, false},
		{"added", []HookChange{{Type: HookAdded}}, true},
		{"modified", []HookChange{{Type: HookModified}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HasExecutableChanges(tt.changes))
		})
	}
}

func TestUT_CollectNewHooks(t *testing.T) {
	changes := []HookChange{
		{Phase: "pre_scaffold", Type: HookAdded, NewHooks: []string{"echo pre"}},
		{Phase: "post_scaffold", Type: HookModified, NewHooks: []string{"go mod tidy", "go generate ./..."}},
		{Phase: "post_scaffold", Type: HookRemoved, OldHooks: []string{"old-hook"}},
	}

	hooks := CollectNewHooks(changes)
	assert.Equal(t, []string{"echo pre"}, hooks.PreScaffold)
	assert.Equal(t, []string{"go mod tidy", "go generate ./..."}, hooks.PostScaffold)
}

func TestUT_FormatHookChanges(t *testing.T) {
	changes := []HookChange{
		{Phase: "post_scaffold", Type: HookAdded, NewHooks: []string{"go mod tidy"}},
		{Phase: "pre_scaffold", Type: HookRemoved, OldHooks: []string{"echo old"}},
		{Phase: "post_scaffold", Type: HookModified, OldHooks: []string{"echo v1"}, NewHooks: []string{"echo v2"}},
	}

	lines := FormatHookChanges(changes)
	assert.NotEmpty(t, lines)
	assert.Contains(t, lines[0], "NEW")
	assert.Contains(t, lines[0], "post_scaffold")
}

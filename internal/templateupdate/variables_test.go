package templateupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

func TestUT_DetectVarChanges_Added(t *testing.T) {
	oldCfg := &tmplconfig.TemplateConfig{
		Vars: map[string]tmplconfig.VariableDef{
			"project_name": {Type: tmplconfig.VarTypeString, Default: "myproject"},
		},
	}
	newCfg := &tmplconfig.TemplateConfig{
		Vars: map[string]tmplconfig.VariableDef{
			"project_name": {Type: tmplconfig.VarTypeString, Default: "myproject"},
			"go_version":   {Type: tmplconfig.VarTypeString, Required: true},
			"ci_provider":  {Type: tmplconfig.VarTypeChoice, Default: "github", Options: []string{"github", "gitlab"}},
		},
	}

	changes := DetectVarChanges(oldCfg, newCfg)
	require.Len(t, changes, 2)
	assert.Equal(t, VarAdded, changes[0].Type)
	assert.Equal(t, "ci_provider", changes[0].Name)
	assert.Equal(t, VarAdded, changes[1].Type)
	assert.Equal(t, "go_version", changes[1].Name)
}

func TestUT_DetectVarChanges_Removed(t *testing.T) {
	oldCfg := &tmplconfig.TemplateConfig{
		Vars: map[string]tmplconfig.VariableDef{
			"project_name": {Type: tmplconfig.VarTypeString},
			"old_var":      {Type: tmplconfig.VarTypeString, Default: "legacy"},
		},
	}
	newCfg := &tmplconfig.TemplateConfig{
		Vars: map[string]tmplconfig.VariableDef{
			"project_name": {Type: tmplconfig.VarTypeString},
		},
	}

	changes := DetectVarChanges(oldCfg, newCfg)
	require.Len(t, changes, 1)
	assert.Equal(t, VarRemoved, changes[0].Type)
	assert.Equal(t, "old_var", changes[0].Name)
}

func TestUT_DetectVarChanges_DefaultChanged(t *testing.T) {
	oldCfg := &tmplconfig.TemplateConfig{
		Vars: map[string]tmplconfig.VariableDef{
			"license": {Type: tmplconfig.VarTypeString, Default: "MIT"},
		},
	}
	newCfg := &tmplconfig.TemplateConfig{
		Vars: map[string]tmplconfig.VariableDef{
			"license": {Type: tmplconfig.VarTypeString, Default: "Apache-2.0"},
		},
	}

	changes := DetectVarChanges(oldCfg, newCfg)
	require.Len(t, changes, 1)
	assert.Equal(t, VarDefaultChanged, changes[0].Type)
	assert.Equal(t, "license", changes[0].Name)
}

func TestUT_DetectVarChanges_TypeChanged(t *testing.T) {
	oldCfg := &tmplconfig.TemplateConfig{
		Vars: map[string]tmplconfig.VariableDef{
			"port": {Type: tmplconfig.VarTypeString, Default: "8080"},
		},
	}
	newCfg := &tmplconfig.TemplateConfig{
		Vars: map[string]tmplconfig.VariableDef{
			"port": {Type: tmplconfig.VarTypeNumber, Default: 8080},
		},
	}

	changes := DetectVarChanges(oldCfg, newCfg)
	// Both type and default change detected — reflect.DeepEqual distinguishes "8080" from 8080.
	require.Len(t, changes, 2)
	// Sorted by VarChangeType enum: VarDefaultChanged (2) < VarTypeChanged (3).
	assert.Equal(t, VarDefaultChanged, changes[0].Type)
	assert.Equal(t, VarTypeChanged, changes[1].Type)
}

func TestUT_DetectVarChanges_NoChanges(t *testing.T) {
	cfg := &tmplconfig.TemplateConfig{
		Vars: map[string]tmplconfig.VariableDef{
			"name": {Type: tmplconfig.VarTypeString, Default: "foo"},
		},
	}

	changes := DetectVarChanges(cfg, cfg)
	assert.Empty(t, changes)
}

func TestUT_DetectVarChanges_NilConfigs(t *testing.T) {
	changes := DetectVarChanges(nil, nil)
	assert.Empty(t, changes)
}

func TestUT_VarChange_NeedsPrompt(t *testing.T) {
	tests := []struct {
		name   string
		change VarChange
		want   bool
	}{
		{
			name: "new required no default",
			change: VarChange{
				Type:   VarAdded,
				NewDef: &tmplconfig.VariableDef{Required: true},
			},
			want: true,
		},
		{
			name: "new required with default",
			change: VarChange{
				Type:   VarAdded,
				NewDef: &tmplconfig.VariableDef{Required: true, Default: "val"},
			},
			want: false,
		},
		{
			name: "new optional",
			change: VarChange{
				Type:   VarAdded,
				NewDef: &tmplconfig.VariableDef{Default: "val"},
			},
			want: false,
		},
		{
			name:   "removed",
			change: VarChange{Type: VarRemoved},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.change.NeedsPrompt())
		})
	}
}

func TestUT_ResolveNewVariables(t *testing.T) {
	changes := []VarChange{
		{
			Name:   "go_version",
			Type:   VarAdded,
			NewDef: &tmplconfig.VariableDef{Required: true},
		},
		{
			Name:   "ci_provider",
			Type:   VarAdded,
			NewDef: &tmplconfig.VariableDef{Default: "github"},
		},
		{
			Name:   "preset_var",
			Type:   VarAdded,
			NewDef: &tmplconfig.VariableDef{Required: true},
		},
		{
			Name:   "old_var",
			Type:   VarRemoved,
			OldDef: &tmplconfig.VariableDef{},
		},
	}

	vars := map[string]any{
		"project_name": "myproject",
		"old_var":      "legacy",
	}
	overrides := map[string]string{
		"preset_var": "override-value",
	}

	needsInput := ResolveNewVariables(changes, vars, overrides)

	// go_version needs input (required, no default, no override).
	assert.Equal(t, []string{"go_version"}, needsInput)

	// ci_provider gets its default.
	assert.Equal(t, "github", vars["ci_provider"])

	// preset_var gets override value.
	assert.Equal(t, "override-value", vars["preset_var"])

	// old_var removed.
	_, exists := vars["old_var"]
	assert.False(t, exists)

	// project_name untouched.
	assert.Equal(t, "myproject", vars["project_name"])
}

func TestUT_FormatVarChanges(t *testing.T) {
	changes := []VarChange{
		{
			Name:   "go_version",
			Type:   VarAdded,
			NewDef: &tmplconfig.VariableDef{Required: true},
		},
		{
			Name:   "ci_provider",
			Type:   VarAdded,
			NewDef: &tmplconfig.VariableDef{Default: "github"},
		},
		{
			Name:   "old_var",
			Type:   VarRemoved,
			OldDef: &tmplconfig.VariableDef{},
		},
		{
			Name:   "license",
			Type:   VarDefaultChanged,
			OldDef: &tmplconfig.VariableDef{Default: "MIT"},
			NewDef: &tmplconfig.VariableDef{Default: "Apache-2.0"},
		},
	}

	userVars := map[string]any{"license": "MIT"}
	lines := FormatVarChanges(changes, userVars)

	assert.Len(t, lines, 4)
	assert.Contains(t, lines[0], "go_version")
	assert.Contains(t, lines[0], "required")
	assert.Contains(t, lines[1], "ci_provider")
	assert.Contains(t, lines[1], "github")
	assert.Contains(t, lines[2], "old_var")
	assert.Contains(t, lines[2], "removed")
	assert.Contains(t, lines[3], "license")
	assert.Contains(t, lines[3], "keeping your value")
}

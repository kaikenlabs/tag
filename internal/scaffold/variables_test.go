package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaikenlabs/tag/internal/replay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockPrompter implements Prompter for testing.
type MockPrompter struct {
	InputResults   map[string]string
	SelectResults  map[string]string
	ConfirmResults map[string]bool
	NumberResults  map[string]float64
	CallCount      map[string]int
}

func NewMockPrompter() *MockPrompter {
	return &MockPrompter{
		InputResults:   make(map[string]string),
		SelectResults:  make(map[string]string),
		ConfirmResults: make(map[string]bool),
		NumberResults:  make(map[string]float64),
		CallCount:      make(map[string]int),
	}
}

func (m *MockPrompter) Input(label, defaultValue string, secret bool) (string, error) {
	m.CallCount["Input"]++
	if val, ok := m.InputResults[label]; ok {
		return val, nil
	}
	return defaultValue, nil
}

func (m *MockPrompter) Select(label string, options []string, defaultIndex int) (string, error) {
	m.CallCount["Select"]++
	if val, ok := m.SelectResults[label]; ok {
		return val, nil
	}
	if defaultIndex >= 0 && defaultIndex < len(options) {
		return options[defaultIndex], nil
	}
	return options[0], nil
}

func (m *MockPrompter) Confirm(label string, defaultValue bool) (bool, error) {
	m.CallCount["Confirm"]++
	if val, ok := m.ConfirmResults[label]; ok {
		return val, nil
	}
	return defaultValue, nil
}

func (m *MockPrompter) Number(label string, defaultValue float64) (float64, error) {
	m.CallCount["Number"]++
	if val, ok := m.NumberResults[label]; ok {
		return val, nil
	}
	return defaultValue, nil
}

func TestUT_VariableCollector_DefaultsOnly(t *testing.T) {
	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Default: "my_project"},
			"port":         {Type: VarTypeNumber, Default: float64(8080)},
			"use_docker":   {Type: VarTypeBoolean, Default: true},
		},
	}

	opts := CollectOptions{
		NoPrompt: true,
		IsTTY:    false,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	assert.Equal(t, "my_project", vars["project_name"])
	assert.Equal(t, float64(8080), vars["port"])
	assert.Equal(t, true, vars["use_docker"])
}

func TestUT_VariableCollector_PriorityChain(t *testing.T) {
	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"var1": {Type: VarTypeString, Default: "default_value"},
			"var2": {Type: VarTypeString, Default: "default_value"},
			"var3": {Type: VarTypeString, Default: "default_value"},
		},
	}

	// Create a temp values file
	tempDir := t.TempDir()
	valuesFile := filepath.Join(tempDir, "values.json")
	err := os.WriteFile(valuesFile, []byte(`{"var2": "values_file_value", "var3": "values_file_value"}`), 0o644)
	require.NoError(t, err)

	opts := CollectOptions{
		ValuesFile: valuesFile,
		Meta: map[string]string{
			"var3": "meta_override",
		},
		NoPrompt: true,
		IsTTY:    false,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	// var1: only default
	assert.Equal(t, "default_value", vars["var1"])
	// var2: values file overwrites default
	assert.Equal(t, "values_file_value", vars["var2"])
	// var3: meta overwrites values file
	assert.Equal(t, "meta_override", vars["var3"])
}

func TestUT_VariableCollector_RequiredMissing(t *testing.T) {
	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"required_var": {Type: VarTypeString, Required: true},
		},
	}

	opts := CollectOptions{
		NoPrompt: true,
		IsTTY:    false,
	}

	_, err := collector.Collect(config, opts)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRequiredVariableMissing)
	assert.Contains(t, err.Error(), "required_var")
}

func TestUT_VariableCollector_PromptForRequired(t *testing.T) {
	mockPrompter := NewMockPrompter()
	mockPrompter.InputResults["Enter value for project_name"] = "prompted_value"
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Required: true},
		},
	}

	opts := CollectOptions{
		NoPrompt: false,
		IsTTY:    true,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	assert.Equal(t, "prompted_value", vars["project_name"])
	assert.Equal(t, 1, mockPrompter.CallCount["Input"])
}

func TestUT_VariableCollector_SkipPrivateVars(t *testing.T) {
	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"_private_var": {Type: VarTypeString, Default: "computed"},
			"public_var":   {Type: VarTypeString, Default: "public"},
		},
	}

	opts := CollectOptions{
		NoPrompt: false,
		IsTTY:    true,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	// Private var should get its default
	assert.Equal(t, "computed", vars["_private_var"])
	// Should not prompt for private vars
	// (Only prompt for optional vars without values, which is the public one)
}

func TestUT_VariableCollector_TypeCoercion(t *testing.T) {
	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"bool_var":   {Type: VarTypeBoolean, Default: false},
			"number_var": {Type: VarTypeNumber, Default: float64(0)},
		},
	}

	opts := CollectOptions{
		Meta: map[string]string{
			"bool_var":   "true",
			"number_var": "42.5",
		},
		NoPrompt: true,
		IsTTY:    false,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	assert.Equal(t, true, vars["bool_var"])
	assert.Equal(t, 42.5, vars["number_var"])
}

func TestUT_VariableCollector_InvalidTypeCoercion(t *testing.T) {
	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"number_var": {Type: VarTypeNumber, Default: float64(0)},
		},
	}

	opts := CollectOptions{
		Meta: map[string]string{
			"number_var": "not_a_number",
		},
		NoPrompt: true,
		IsTTY:    false,
	}

	_, err := collector.Collect(config, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "number_var")
}

func TestUT_VariableCollector_ChoicePrompt(t *testing.T) {
	mockPrompter := NewMockPrompter()
	mockPrompter.SelectResults["Select a license"] = "MIT"
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"license": {
				Type:    VarTypeChoice,
				Prompt:  "Select a license",
				Options: []string{"MIT", "BSD-3", "Apache-2.0"},
			},
		},
	}

	opts := CollectOptions{
		NoPrompt: false,
		IsTTY:    true,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	assert.Equal(t, "MIT", vars["license"])
	assert.Equal(t, 1, mockPrompter.CallCount["Select"])
}

func TestUT_VariableCollector_BooleanPrompt(t *testing.T) {
	mockPrompter := NewMockPrompter()
	mockPrompter.ConfirmResults["Include Docker setup?"] = true
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"use_docker": {
				Type:   VarTypeBoolean,
				Prompt: "Include Docker setup?",
			},
		},
	}

	opts := CollectOptions{
		NoPrompt: false,
		IsTTY:    true,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	assert.Equal(t, true, vars["use_docker"])
	assert.Equal(t, 1, mockPrompter.CallCount["Confirm"])
}

func TestUT_VariableCollector_NumberPrompt(t *testing.T) {
	mockPrompter := NewMockPrompter()
	mockPrompter.NumberResults["Server port"] = 3000
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"port": {
				Type:   VarTypeNumber,
				Prompt: "Server port",
			},
		},
	}

	opts := CollectOptions{
		NoPrompt: false,
		IsTTY:    true,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	assert.Equal(t, float64(3000), vars["port"])
	assert.Equal(t, 1, mockPrompter.CallCount["Number"])
}

func TestUT_ParseMetaFlags(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected map[string]string
		wantErr  bool
	}{
		{
			name:     "valid flags",
			flags:    []string{"key1=value1", "key2=value2"},
			expected: map[string]string{"key1": "value1", "key2": "value2"},
			wantErr:  false,
		},
		{
			name:     "value with equals sign",
			flags:    []string{"key=value=with=equals"},
			expected: map[string]string{"key": "value=with=equals"},
			wantErr:  false,
		},
		{
			name:     "empty flags",
			flags:    []string{},
			expected: map[string]string{},
			wantErr:  false,
		},
		{
			name:    "invalid flag format",
			flags:   []string{"invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseMetaFlags(tt.flags)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestUT_ParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
		wantErr  bool
	}{
		{"true", true, false},
		{"True", true, false},
		{"TRUE", true, false},
		{"yes", true, false},
		{"Yes", true, false},
		{"y", true, false},
		{"Y", true, false},
		{"1", true, false},
		{"on", true, false},
		{"false", false, false},
		{"False", false, false},
		{"FALSE", false, false},
		{"no", false, false},
		{"No", false, false},
		{"n", false, false},
		{"N", false, false},
		{"0", false, false},
		{"off", false, false},
		{"", false, false},
		{"invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseBool(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// =============================================================================
// Replay-specific tests
// =============================================================================

func TestUT_VariableCollector_ReplayPriority(t *testing.T) {
	// Set up a temp home directory for replay files
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Create replay data
	templateRef := "gh:test/replay-test"
	err := replay.Save(templateRef, "v1.0.0", map[string]any{
		"var1": "replay_value",
		"var2": "replay_value",
		"var3": "replay_value",
	}, nil)
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"var1": {Type: VarTypeString, Default: "default_value"},
			"var2": {Type: VarTypeString, Default: "default_value"},
			"var3": {Type: VarTypeString, Default: "default_value"},
		},
	}

	// Create a temp values file that overrides var2
	valuesFile := filepath.Join(tempHome, "values.json")
	err = os.WriteFile(valuesFile, []byte(`{"var2": "values_file_value", "var3": "values_file_value"}`), 0o644)
	require.NoError(t, err)

	opts := CollectOptions{
		Replay:      true,
		TemplateRef: templateRef,
		ValuesFile:  valuesFile,
		Meta: map[string]string{
			"var3": "meta_override",
		},
		NoPrompt: true,
		IsTTY:    false,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	// var1: replay overwrites default
	assert.Equal(t, "replay_value", vars["var1"])
	// var2: values file overwrites replay
	assert.Equal(t, "values_file_value", vars["var2"])
	// var3: meta overwrites values file (which overwrote replay)
	assert.Equal(t, "meta_override", vars["var3"])
}

func TestUT_VariableCollector_ReplayOverridesDefaults(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	templateRef := "gh:test/replay-defaults"
	err := replay.Save(templateRef, "", map[string]any{
		"project_name": "replayed-project",
		"port":         float64(9000),
		"use_docker":   true,
	}, nil)
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Default: "default-project"},
			"port":         {Type: VarTypeNumber, Default: float64(8080)},
			"use_docker":   {Type: VarTypeBoolean, Default: false},
		},
	}

	opts := CollectOptions{
		Replay:      true,
		TemplateRef: templateRef,
		NoPrompt:    true,
		IsTTY:       false,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	assert.Equal(t, "replayed-project", vars["project_name"])
	assert.Equal(t, float64(9000), vars["port"])
	assert.Equal(t, true, vars["use_docker"])
}

func TestUT_VariableCollector_ReplayNotFound(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"var1": {Type: VarTypeString, Default: "default"},
		},
	}

	opts := CollectOptions{
		Replay:      true,
		TemplateRef: "gh:nonexistent/repo",
		NoPrompt:    true,
		IsTTY:       false,
	}

	_, err := collector.Collect(config, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no saved replay data found")
}

func TestUT_VariableCollector_ReplayWithoutTemplateRef(t *testing.T) {
	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"var1": {Type: VarTypeString, Default: "default"},
		},
	}

	opts := CollectOptions{
		Replay:      true,
		TemplateRef: "", // Missing template ref
		NoPrompt:    true,
		IsTTY:       false,
	}

	_, err := collector.Collect(config, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template reference")
}

func TestUT_VariableCollector_ReplayPromptsForNewVars(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Save replay with only var1
	templateRef := "gh:test/replay-new-vars"
	err := replay.Save(templateRef, "", map[string]any{
		"var1": "replayed_value",
	}, nil)
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	mockPrompter.InputResults["Enter value for new_var"] = "prompted_new_value"
	collector := NewVariableCollector(mockPrompter)

	// Config has both var1 (in replay) and new_var (not in replay)
	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"var1":    {Type: VarTypeString, Default: "default"},
			"new_var": {Type: VarTypeString, Required: true},
		},
	}

	opts := CollectOptions{
		Replay:      true,
		TemplateRef: templateRef,
		NoPrompt:    false,
		IsTTY:       true,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	// var1 should come from replay
	assert.Equal(t, "replayed_value", vars["var1"])
	// new_var should be prompted for
	assert.Equal(t, "prompted_new_value", vars["new_var"])
	assert.Equal(t, 1, mockPrompter.CallCount["Input"])
}

func TestUT_VariableCollector_ReplayTypeCoercion(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Save replay with typed values
	templateRef := "gh:test/replay-types"
	err := replay.Save(templateRef, "", map[string]any{
		"str_var":  "string_value",
		"bool_var": true,
		"num_var":  float64(42),
	}, nil)
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"str_var":  {Type: VarTypeString},
			"bool_var": {Type: VarTypeBoolean},
			"num_var":  {Type: VarTypeNumber},
		},
	}

	opts := CollectOptions{
		Replay:      true,
		TemplateRef: templateRef,
		NoPrompt:    true,
		IsTTY:       false,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	assert.Equal(t, "string_value", vars["str_var"])
	assert.Equal(t, true, vars["bool_var"])
	assert.Equal(t, float64(42), vars["num_var"])
}

func TestUT_VariableCollector_ReplayIgnoresRemovedVars(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Save replay with a variable that's no longer in the template
	templateRef := "gh:test/replay-removed"
	err := replay.Save(templateRef, "", map[string]any{
		"current_var": "current_value",
		"removed_var": "removed_value",
	}, nil)
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	// Template only has current_var, not removed_var
	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"current_var": {Type: VarTypeString},
		},
	}

	opts := CollectOptions{
		Replay:      true,
		TemplateRef: templateRef,
		NoPrompt:    true,
		IsTTY:       false,
	}

	vars, err := collector.Collect(config, opts)
	require.NoError(t, err)

	// current_var should be loaded from replay
	assert.Equal(t, "current_value", vars["current_var"])
	// removed_var should still be in vars (unknown variables are kept)
	assert.Equal(t, "removed_value", vars["removed_var"])
}

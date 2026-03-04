package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/replay"
	"github.com/kaikenlabs/tag/internal/template"
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

	opts := Options{
		NoInput: true,
	}

	vars, err := collector.Collect(config, opts, false)
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

	opts := Options{
		ValuesFile: valuesFile,
		Meta: map[string]string{
			"var3": "meta_override",
		},
		NoInput: true,
	}

	vars, err := collector.Collect(config, opts, false)
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

	opts := Options{
		NoInput: true,
	}

	_, err := collector.Collect(config, opts, false)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRequiredVariableMissing)
	assert.Contains(t, err.Error(), "required_var")
	assert.Contains(t, err.Error(), "--meta required_var=<value>")
	assert.Contains(t, err.Error(), "--values <file.json>")
}

func TestUT_VariableCollector_RequiredMissing_MultipleVars(t *testing.T) {
	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Required: true},
			"author":       {Type: VarTypeString, Required: true},
		},
	}

	opts := Options{
		NoInput: true,
	}

	_, err := collector.Collect(config, opts, false)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRequiredVariableMissing)
	// Both vars should be listed (sorted)
	assert.Contains(t, err.Error(), "author, project_name")
	// Both should have hints
	assert.Contains(t, err.Error(), "--meta author=<value>")
	assert.Contains(t, err.Error(), "--meta project_name=<value>")
	assert.Contains(t, err.Error(), "--values <file.json>")
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

	opts := Options{}

	vars, err := collector.Collect(config, opts, true)
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

	opts := Options{}

	vars, err := collector.Collect(config, opts, true)
	require.NoError(t, err)

	// Private var should get its default
	assert.Equal(t, "computed", vars["_private_var"])
	// Should not prompt for private vars (but should prompt for public_var)
	assert.Equal(t, 1, mockPrompter.CallCount["Input"]) // Only public_var prompted
}

func TestUT_VariableCollector_PromptForVarsWithDefaults(t *testing.T) {
	// This test verifies that variables WITH defaults are still prompted in interactive mode
	mockPrompter := NewMockPrompter()
	mockPrompter.InputResults["Enter value for project_name"] = "user_provided_value"
	mockPrompter.InputResults["Enter value for author"] = "Jane Doe"
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Default: "default_project"},
			"author":       {Type: VarTypeString, Default: "John Doe"},
		},
	}

	opts := Options{}

	vars, err := collector.Collect(config, opts, true)
	require.NoError(t, err)

	// Both variables should be prompted even though they have defaults
	assert.Equal(t, "user_provided_value", vars["project_name"])
	assert.Equal(t, "Jane Doe", vars["author"])
	assert.Equal(t, 2, mockPrompter.CallCount["Input"])
}

func TestUT_VariableCollector_ValuesFileSkipsPrompt(t *testing.T) {
	// This test verifies that variables provided via values file are NOT re-prompted
	mockPrompter := NewMockPrompter()
	mockPrompter.InputResults["Enter value for prompted_var"] = "prompted_value"
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"file_var":     {Type: VarTypeString, Default: "default"},
			"prompted_var": {Type: VarTypeString, Default: "default"},
		},
	}

	// Create a temp values file that provides file_var
	tempDir := t.TempDir()
	valuesFile := filepath.Join(tempDir, "values.json")
	err := os.WriteFile(valuesFile, []byte(`{"file_var": "from_file"}`), 0o644)
	require.NoError(t, err)

	opts := Options{
		ValuesFile: valuesFile,
	}

	vars, err := collector.Collect(config, opts, true)
	require.NoError(t, err)

	// file_var should come from values file (not prompted)
	assert.Equal(t, "from_file", vars["file_var"])
	// prompted_var should be prompted since it wasn't in values file
	assert.Equal(t, "prompted_value", vars["prompted_var"])
	// Only one prompt should have happened
	assert.Equal(t, 1, mockPrompter.CallCount["Input"])
}

func TestUT_VariableCollector_SkipDerivedVars(t *testing.T) {
	// This test verifies that derived variables (whose defaults contain template expressions)
	// are NOT prompted, following Cookiecutter behavior
	mockPrompter := NewMockPrompter()
	mockPrompter.InputResults["Enter value for package_display_name"] = "My Package"
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"package_display_name": {Type: VarTypeString, Default: "Package Name"},
			// Derived variable - should not be prompted
			"package_name": {Type: VarTypeString, Default: "{{ vars.package_display_name.lower().replace(' ', '_') }}"},
			// Another derived variable
			"github_repo": {Type: VarTypeString, Default: "{{vars.package_name}}"},
		},
	}

	opts := Options{}

	vars, err := collector.Collect(config, opts, true)
	require.NoError(t, err)

	// package_display_name should be prompted (user input)
	assert.Equal(t, "My Package", vars["package_display_name"])
	// package_name should NOT be prompted - keeps template expression as default
	assert.Equal(t, "{{ vars.package_display_name.lower().replace(' ', '_') }}", vars["package_name"])
	// github_repo should NOT be prompted - keeps template expression as default
	assert.Equal(t, "{{vars.package_name}}", vars["github_repo"])
	// Only one prompt should have happened (for package_display_name)
	assert.Equal(t, 1, mockPrompter.CallCount["Input"])
}

func TestUT_VariableCollector_DerivedVarsWithMethodCalls(t *testing.T) {
	mockPrompter := NewMockPrompter()
	mockPrompter.InputResults["Enter value for project_name"] = "My Project"
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Default: "Default Project"},
			"project_slug": {Type: VarTypeString, Default: "{{ vars.project_name | snake }}"},
		},
	}

	opts := Options{}

	vars, err := collector.Collect(config, opts, true)
	require.NoError(t, err)

	// project_name should be prompted
	assert.Equal(t, "My Project", vars["project_name"])
	// project_slug should NOT be prompted (derived)
	assert.Equal(t, "{{ vars.project_name | snake }}", vars["project_slug"])
	// Only project_name was prompted
	assert.Equal(t, 1, mockPrompter.CallCount["Input"])
}

func TestUT_VariableCollector_MetaSkipsPrompt(t *testing.T) {
	// Variables provided via --meta flag should NOT be prompted in interactive mode
	mockPrompter := NewMockPrompter()
	mockPrompter.InputResults["Enter value for prompted_var"] = "prompted_value"
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"meta_var":     {Type: VarTypeString, Default: "default"},
			"prompted_var": {Type: VarTypeString, Default: "default"},
		},
	}

	opts := Options{
		Meta: map[string]string{
			"meta_var": "from_meta",
		},
	}

	vars, err := collector.Collect(config, opts, true)
	require.NoError(t, err)

	// meta_var should come from --meta (not prompted)
	assert.Equal(t, "from_meta", vars["meta_var"])
	// prompted_var should be prompted since it wasn't in --meta
	assert.Equal(t, "prompted_value", vars["prompted_var"])
	// Only one prompt should have happened (prompted_var only)
	assert.Equal(t, 1, mockPrompter.CallCount["Input"])
}

func TestUT_VariableCollector_MetaSkipsPromptAllTypes(t *testing.T) {
	// Verify --meta skips prompts for boolean, number, and choice types too
	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"bool_var":   {Type: VarTypeBoolean, Default: false},
			"number_var": {Type: VarTypeNumber, Default: float64(0)},
			"choice_var": {Type: VarTypeChoice, Options: []string{"a", "b"}, Default: "a"},
		},
	}

	opts := Options{
		Meta: map[string]string{
			"bool_var":   "true",
			"number_var": "42",
			"choice_var": "b",
		},
	}

	vars, err := collector.Collect(config, opts, true)
	require.NoError(t, err)

	assert.Equal(t, true, vars["bool_var"])
	assert.Equal(t, float64(42), vars["number_var"])
	assert.Equal(t, "b", vars["choice_var"])
	// No prompts should have happened
	assert.Equal(t, 0, mockPrompter.CallCount["Input"])
	assert.Equal(t, 0, mockPrompter.CallCount["Confirm"])
	assert.Equal(t, 0, mockPrompter.CallCount["Number"])
	assert.Equal(t, 0, mockPrompter.CallCount["Select"])
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

	opts := Options{
		Meta: map[string]string{
			"bool_var":   "true",
			"number_var": "42.5",
		},
		NoInput: true,
	}

	vars, err := collector.Collect(config, opts, false)
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

	opts := Options{
		Meta: map[string]string{
			"number_var": "not_a_number",
		},
		NoInput: true,
	}

	_, err := collector.Collect(config, opts, false)
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

	opts := Options{}

	vars, err := collector.Collect(config, opts, true)
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

	opts := Options{}

	vars, err := collector.Collect(config, opts, true)
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

	opts := Options{}

	vars, err := collector.Collect(config, opts, true)
	require.NoError(t, err)

	assert.Equal(t, float64(3000), vars["port"])
	assert.Equal(t, 1, mockPrompter.CallCount["Number"])
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

	opts := Options{
		Replay:      true,
		TemplateRef: templateRef,
		ValuesFile:  valuesFile,
		Meta: map[string]string{
			"var3": "meta_override",
		},
		NoInput: true,
	}

	vars, err := collector.Collect(config, opts, false)
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

	opts := Options{
		Replay:      true,
		TemplateRef: templateRef,
		NoInput:     true,
	}

	vars, err := collector.Collect(config, opts, false)
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

	opts := Options{
		Replay:      true,
		TemplateRef: "gh:nonexistent/repo",
		NoInput:     true,
	}

	_, err := collector.Collect(config, opts, false)
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

	opts := Options{
		Replay:      true,
		TemplateRef: "", // Missing template ref
		NoInput:     true,
	}

	_, err := collector.Collect(config, opts, false)
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

	opts := Options{
		Replay:      true,
		TemplateRef: templateRef,
	}

	vars, err := collector.Collect(config, opts, true)
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

	opts := Options{
		Replay:      true,
		TemplateRef: templateRef,
		NoInput:     true,
	}

	vars, err := collector.Collect(config, opts, false)
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

	opts := Options{
		Replay:      true,
		TemplateRef: templateRef,
		NoInput:     true,
	}

	vars, err := collector.Collect(config, opts, false)
	require.NoError(t, err)

	// current_var should be loaded from replay
	assert.Equal(t, "current_value", vars["current_var"])
	// removed_var should still be in vars (unknown variables are kept)
	assert.Equal(t, "removed_value", vars["removed_var"])
}

func TestUT_ResolveDerivedVars_Basic(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name":   {Type: VarTypeString, Default: "My Service"},
			"__project_slug": {Type: VarTypeString, Default: "{{ vars.project_name | lower }}"},
		},
	}

	vars := map[string]any{
		"project_name":   "My Service",
		"__project_slug": "{{ vars.project_name | lower }}",
	}

	err = ResolveDerivedVars(engine, config, vars)
	require.NoError(t, err)

	assert.Equal(t, "my service", vars["__project_slug"])
	// Non-derived variable should be unchanged
	assert.Equal(t, "My Service", vars["project_name"])
}

func TestUT_ResolveDerivedVars_WithFilters(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"display_name":  {Type: VarTypeString, Default: "My Cool Project"},
			"_package_name": {Type: VarTypeString, Default: "{{ vars.display_name | snake }}"},
		},
	}

	vars := map[string]any{
		"display_name":  "My Cool Project",
		"_package_name": "{{ vars.display_name | snake }}",
	}

	err = ResolveDerivedVars(engine, config, vars)
	require.NoError(t, err)

	assert.Equal(t, "my_cool_project", vars["_package_name"])
}

func TestUT_ResolveDerivedVars_ChainedDerived(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name":   {Type: VarTypeString, Default: "My Project"},
			"__project_slug": {Type: VarTypeString, Default: "{{ vars.project_name | lower }}"},
			// References another derived var (sorted after __project_slug)
			"__repo_name": {Type: VarTypeString, Default: "{{ vars.__project_slug }}"},
		},
	}

	vars := map[string]any{
		"project_name":   "My Project",
		"__project_slug": "{{ vars.project_name | lower }}",
		"__repo_name":    "{{ vars.__project_slug }}",
	}

	err = ResolveDerivedVars(engine, config, vars)
	require.NoError(t, err)

	assert.Equal(t, "my project", vars["__project_slug"])
	assert.Equal(t, "my project", vars["__repo_name"])
}

func TestUT_ResolveDerivedVars_SkipsNonDerived(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Default: "test"},
			"author":       {Type: VarTypeString, Default: "Test Author"},
		},
	}

	vars := map[string]any{
		"project_name": "test",
		"author":       "Test Author",
	}

	err = ResolveDerivedVars(engine, config, vars)
	require.NoError(t, err)

	// Both should be unchanged — neither is derived
	assert.Equal(t, "test", vars["project_name"])
	assert.Equal(t, "Test Author", vars["author"])
}

func TestUT_ResolveDerivedVars_MethodCallSyntax(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"display_name": {Type: VarTypeString, Default: "My Package"},
			"_slug":        {Type: VarTypeString, Default: "{{ vars.display_name.lower().replace(' ', '_') }}"},
		},
	}

	vars := map[string]any{
		"display_name": "My Package",
		"_slug":        "{{ vars.display_name.lower().replace(' ', '_') }}",
	}

	err = ResolveDerivedVars(engine, config, vars)
	require.NoError(t, err)

	assert.Equal(t, "my_package", vars["_slug"])
}

// TestUT_EvaluatedDefault_PromptShownWithResolvedDefault verifies that an
// evaluated-default variable (expanded form with explicit prompt + expression
// default) is prompted interactively, and the prompt receives the resolved
// expression as its suggested default.
func TestUT_EvaluatedDefault_PromptShownWithResolvedDefault(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Default: "my-service"},
			"module_path": {
				Type:    VarTypeString,
				Prompt:  "Go module path",
				Default: "bitbucket.org/whalar/{{ vars.project_name }}",
			},
		},
	}

	// MockPrompter.Input returns defaultValue when no explicit result is registered,
	// simulating the user pressing Enter to accept the suggested default.
	vars, err := collector.Collect(config, Options{}, true)
	require.NoError(t, err)

	// Verify module_path was resolved before prompting and stored correctly.
	assert.Equal(t, "bitbucket.org/whalar/my-service", vars["module_path"])
	// Verify the prompt was actually called (variable was not silently derived).
	assert.Equal(t, 2, mockPrompter.CallCount["Input"]) // project_name + module_path
}

// TestUT_EvaluatedDefault_UserCanOverride verifies that the user can provide
// a custom value to override the resolved expression default.
func TestUT_EvaluatedDefault_UserCanOverride(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	mockPrompter.InputResults["Go module path"] = "github.com/myorg/my-service"
	collector := NewVariableCollector(mockPrompter)
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Default: "my-service"},
			"module_path": {
				Type:    VarTypeString,
				Prompt:  "Go module path",
				Default: "bitbucket.org/whalar/{{ vars.project_name }}",
			},
		},
	}

	vars, err := collector.Collect(config, Options{}, true)
	require.NoError(t, err)

	// User overrode the resolved default.
	assert.Equal(t, "github.com/myorg/my-service", vars["module_path"])
}

// TestUT_EvaluatedDefault_NonTTYResolvesExpression verifies that in non-TTY
// mode (no interactive prompt), the expression default is resolved by
// ResolveDerivedVars just like a classic derived variable.
func TestUT_EvaluatedDefault_NonTTYResolvesExpression(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Default: "my-service"},
			"module_path": {
				Type:    VarTypeString,
				Prompt:  "Go module path",
				Default: "bitbucket.org/whalar/{{ vars.project_name }}",
			},
		},
	}

	// Non-TTY: isTTY=false, no prompts fired.
	vars, err := collector.Collect(config, Options{}, false)
	require.NoError(t, err)

	// After Collect(), module_path still holds the raw expression (not prompted).
	// ResolveDerivedVars should resolve it.
	err = ResolveDerivedVars(engine, config, vars)
	require.NoError(t, err)

	assert.Equal(t, "bitbucket.org/whalar/my-service", vars["module_path"])
	// No prompts should have fired.
	assert.Equal(t, 0, mockPrompter.CallCount["Input"])
}

// TestUT_ResolveDerivedVars_EvaluatedDefaultSkipsResolvedValue verifies that
// ResolveDerivedVars does NOT overwrite an evaluated-default variable that was
// already resolved interactively (i.e., its value is no longer a template expression).
func TestUT_ResolveDerivedVars_EvaluatedDefaultSkipsResolvedValue(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Default: "my-service"},
			"module_path": {
				Type:    VarTypeString,
				Prompt:  "Go module path",
				Default: "bitbucket.org/whalar/{{ vars.project_name }}",
			},
		},
	}

	// Simulate post-prompt state: module_path already has a concrete user-provided value.
	vars := map[string]any{
		"project_name": "my-service",
		"module_path":  "github.com/myorg/my-service", // user overrode
	}

	err = ResolveDerivedVars(engine, config, vars)
	require.NoError(t, err)

	// Should not be overwritten.
	assert.Equal(t, "github.com/myorg/my-service", vars["module_path"])
}

// TestUT_EvaluatedDefault_SeesPositionalProjectName verifies that an
// evaluated-default expression resolves using the positional project_name
// argument (routed through opts.Meta), not the static default.
func TestUT_EvaluatedDefault_SeesPositionalProjectName(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Default: "my-service"},
			"module_path": {
				Type:    VarTypeString,
				Prompt:  "Go module path",
				Default: "example.com/myorg/{{ vars.project_name | kebab }}",
			},
		},
	}

	// Positional project_name is routed through opts.Meta by collectVars.
	opts := Options{
		Meta: map[string]string{"project_name": "hello-world"},
	}

	vars, err := collector.Collect(config, opts, true)
	require.NoError(t, err)

	// The evaluated default should see "hello-world", not "my-service".
	assert.Equal(t, "hello-world", vars["project_name"])
	assert.Equal(t, "example.com/myorg/hello-world", vars["module_path"])
}

// TestUT_EvaluatedDefault_SeesMetaOverride verifies that evaluated defaults
// see explicit -m flag overrides, not just static defaults.
func TestUT_EvaluatedDefault_SeesMetaOverride(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"org_name":     {Type: VarTypeString, Default: "default-org"},
			"project_name": {Type: VarTypeString, Default: "my-service"},
			"repo_url": {
				Type:    VarTypeString,
				Prompt:  "Repository URL",
				Default: "github.com/{{ vars.org_name }}/{{ vars.project_name }}",
			},
		},
	}

	opts := Options{
		Meta: map[string]string{
			"org_name":     "mycompany",
			"project_name": "cool-app",
		},
	}

	vars, err := collector.Collect(config, opts, true)
	require.NoError(t, err)

	assert.Equal(t, "github.com/mycompany/cool-app", vars["repo_url"])
}

// TestUT_VariableCollector_DependencyOrdering verifies that a variable whose
// default references another variable is prompted after its dependency,
// regardless of alphabetical order.
func TestUT_VariableCollector_DependencyOrdering(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	// Track prompt order via a wrapper prompter.
	promptOrder := []string{}
	collector := NewVariableCollector(&orderTrackingPrompter{
		inner:       NewMockPrompter(),
		promptOrder: &promptOrder,
	})
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			// "aaa_module" sorts before "zzz_name" alphabetically,
			// but aaa_module depends on zzz_name.
			"aaa_module": {
				Type:    VarTypeString,
				Prompt:  "Module path",
				Default: "example.com/{{ vars.zzz_name }}",
			},
			"zzz_name": {Type: VarTypeString, Default: "my-project", Prompt: "Project name"},
		},
	}

	vars, err := collector.Collect(config, Options{}, true)
	require.NoError(t, err)

	// zzz_name should be prompted before aaa_module despite alphabetical order.
	require.Len(t, promptOrder, 2)
	assert.Equal(t, "Project name", promptOrder[0])
	assert.Equal(t, "Module path", promptOrder[1])
	assert.Equal(t, "example.com/my-project", vars["aaa_module"])
}

// TestUT_VariableCollector_CircularDependencyError verifies that circular
// variable dependencies (A → B → A) produce a clear error.
func TestUT_VariableCollector_CircularDependencyError(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"var_a": {
				Type:    VarTypeString,
				Prompt:  "Variable A",
				Default: "{{ vars.var_b }}",
			},
			"var_b": {
				Type:    VarTypeString,
				Prompt:  "Variable B",
				Default: "{{ vars.var_a }}",
			},
		},
	}

	_, err = collector.Collect(config, Options{}, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular")
}

// TestUT_VariableCollector_SelfReferenceError verifies that a variable
// referencing itself in its default produces a circular dependency error.
func TestUT_VariableCollector_SelfReferenceError(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"name": {
				Type:    VarTypeString,
				Prompt:  "Name",
				Default: "prefix-{{ vars.name }}",
			},
		},
	}

	_, err = collector.Collect(config, Options{}, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular")
}

// TestUT_VariableCollector_MultiHopDependencyChain verifies that a chain
// A → B → C is resolved in correct order (C first, then B, then A).
func TestUT_VariableCollector_MultiHopDependencyChain(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	promptOrder := []string{}
	collector := NewVariableCollector(&orderTrackingPrompter{
		inner:       NewMockPrompter(),
		promptOrder: &promptOrder,
	})
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"full_path": {
				Type:    VarTypeString,
				Prompt:  "Full path",
				Default: "{{ vars.module_path }}/cmd",
			},
			"module_path": {
				Type:    VarTypeString,
				Prompt:  "Module",
				Default: "example.com/{{ vars.project_name }}",
			},
			"project_name": {Type: VarTypeString, Default: "svc", Prompt: "Project"},
		},
	}

	vars, err := collector.Collect(config, Options{}, true)
	require.NoError(t, err)

	// Order must be: project_name → module_path → full_path
	require.Len(t, promptOrder, 3)
	assert.Equal(t, "Project", promptOrder[0])
	assert.Equal(t, "Module", promptOrder[1])
	assert.Equal(t, "Full path", promptOrder[2])
	assert.Equal(t, "example.com/svc/cmd", vars["full_path"])
}

// TestUT_VariableCollector_DeterministicTieBreak verifies that independent
// variables (no dependencies between them) are sorted lexicographically.
func TestUT_VariableCollector_DeterministicTieBreak(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	promptOrder := []string{}
	collector := NewVariableCollector(&orderTrackingPrompter{
		inner:       NewMockPrompter(),
		promptOrder: &promptOrder,
	})
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"charlie": {Type: VarTypeString, Default: "c", Prompt: "charlie"},
			"alpha":   {Type: VarTypeString, Default: "a", Prompt: "alpha"},
			"bravo":   {Type: VarTypeString, Default: "b", Prompt: "bravo"},
		},
	}

	_, err = collector.Collect(config, Options{}, true)
	require.NoError(t, err)

	// Independent vars should maintain lexicographic order.
	require.Len(t, promptOrder, 3)
	assert.Equal(t, "alpha", promptOrder[0])
	assert.Equal(t, "bravo", promptOrder[1])
	assert.Equal(t, "charlie", promptOrder[2])
}

// TestUT_EvaluatedDefault_NonTTY_MetaOverride verifies that non-interactive
// mode correctly resolves evaluated defaults when meta overrides are provided.
func TestUT_EvaluatedDefault_NonTTY_MetaOverride(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	mockPrompter := NewMockPrompter()
	collector := NewVariableCollector(mockPrompter)
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"project_name": {Type: VarTypeString, Default: "my-service"},
			"module_path": {
				Type:    VarTypeString,
				Prompt:  "Go module path",
				Default: "example.com/myorg/{{ vars.project_name }}",
			},
		},
	}

	opts := Options{
		Meta: map[string]string{"project_name": "hello-world"},
	}

	// Non-TTY mode
	vars, err := collector.Collect(config, opts, false)
	require.NoError(t, err)

	// After Collect, module_path should still have the expression (not prompted).
	// ResolveDerivedVars should then resolve it with the meta-overridden project_name.
	err = ResolveDerivedVars(engine, config, vars)
	require.NoError(t, err)

	assert.Equal(t, "hello-world", vars["project_name"])
	assert.Equal(t, "example.com/myorg/hello-world", vars["module_path"])
	assert.Equal(t, 0, mockPrompter.CallCount["Input"])
}

// TestUT_ExtractVarRefs verifies that variable references are correctly
// extracted from template expressions.
func TestUT_ExtractVarRefs(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected []string
	}{
		{
			name:     "single reference",
			expr:     "{{ vars.project_name }}",
			expected: []string{"project_name"},
		},
		{
			name:     "reference with filter",
			expr:     "{{ vars.project_name | kebab }}",
			expected: []string{"project_name"},
		},
		{
			name:     "multiple references",
			expr:     "{{ vars.org }}/{{ vars.project_name }}",
			expected: []string{"org", "project_name"},
		},
		{
			name:     "duplicate references deduplicated",
			expr:     "{{ vars.name }}-{{ vars.name }}",
			expected: []string{"name"},
		},
		{
			name:     "no references",
			expr:     "static-value",
			expected: nil,
		},
		{
			name:     "underscore and digits in name",
			expr:     "{{ vars._private_var2 }}",
			expected: []string{"_private_var2"},
		},
		{
			name:     "method call syntax",
			expr:     "{{ vars.name.lower() }}",
			expected: []string{"name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := extractVarRefs(tt.expr)
			assert.Equal(t, tt.expected, refs)
		})
	}
}

// TestUT_TopologicalSortVars_Basic verifies correct topological ordering.
func TestUT_TopologicalSortVars_Basic(t *testing.T) {
	vars := map[string]VariableDef{
		"module_path": {
			Type:    VarTypeString,
			Prompt:  "Module",
			Default: "example.com/{{ vars.project_name }}",
		},
		"project_name": {Type: VarTypeString, Default: "svc"},
	}

	sorted, err := topologicalSortVars(vars)
	require.NoError(t, err)

	// project_name must come before module_path
	nameIdx := indexOf(sorted, "project_name")
	pathIdx := indexOf(sorted, "module_path")
	assert.True(t, nameIdx < pathIdx, "project_name should come before module_path, got %v", sorted)
}

// TestUT_TopologicalSortVars_CircularDependency verifies cycle detection.
func TestUT_TopologicalSortVars_CircularDependency(t *testing.T) {
	vars := map[string]VariableDef{
		"a": {
			Type:    VarTypeString,
			Prompt:  "A",
			Default: "{{ vars.b }}",
		},
		"b": {
			Type:    VarTypeString,
			Prompt:  "B",
			Default: "{{ vars.a }}",
		},
	}

	_, err := topologicalSortVars(vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular")
}

// TestUT_TopologicalSortVars_DeterministicTieBreak verifies that independent
// nodes are sorted lexicographically for deterministic output.
func TestUT_TopologicalSortVars_DeterministicTieBreak(t *testing.T) {
	vars := map[string]VariableDef{
		"zebra": {Type: VarTypeString, Default: "z"},
		"alpha": {Type: VarTypeString, Default: "a"},
		"mango": {Type: VarTypeString, Default: "m"},
	}

	sorted, err := topologicalSortVars(vars)
	require.NoError(t, err)

	assert.Equal(t, []string{"alpha", "mango", "zebra"}, sorted)
}

// orderTrackingPrompter wraps a Prompter and records the order of prompts.
type orderTrackingPrompter struct {
	inner       Prompter
	promptOrder *[]string
}

func (p *orderTrackingPrompter) Input(label, defaultValue string, secret bool) (string, error) {
	*p.promptOrder = append(*p.promptOrder, label)
	return p.inner.Input(label, defaultValue, secret)
}

func (p *orderTrackingPrompter) Select(label string, options []string, defaultIndex int) (string, error) {
	*p.promptOrder = append(*p.promptOrder, label)
	return p.inner.Select(label, options, defaultIndex)
}

func (p *orderTrackingPrompter) Confirm(label string, defaultValue bool) (bool, error) {
	*p.promptOrder = append(*p.promptOrder, label)
	return p.inner.Confirm(label, defaultValue)
}

func (p *orderTrackingPrompter) Number(label string, defaultValue float64) (float64, error) {
	*p.promptOrder = append(*p.promptOrder, label)
	return p.inner.Number(label, defaultValue)
}

// indexOf returns the index of s in slice, or -1.
func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}

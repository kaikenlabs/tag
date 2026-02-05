package scaffold

import (
	"os"
	"path/filepath"
	"testing"

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

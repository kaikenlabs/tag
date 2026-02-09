package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/scaffold"
)

func TestUT_ConvertCookiecutterConfig_StringDefaults(t *testing.T) {
	input := `{
		"project_name": "my_project",
		"author": "John Doe"
	}`

	config, conversions, warnings, err := ConvertCookiecutterConfig([]byte(input))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Len(t, conversions, 2)

	// Check project_name
	assert.Equal(t, "my_project", config.RawVars["project_name"])
	varDef := config.Vars["project_name"]
	assert.Equal(t, scaffold.VarTypeString, varDef.Type)
	assert.Equal(t, "my_project", varDef.Default)

	// Check author
	assert.Equal(t, "John Doe", config.RawVars["author"])
}

func TestUT_ConvertCookiecutterConfig_DerivedVariables(t *testing.T) {
	// Test that derived variables (with templated defaults) have their namespace converted
	input := `{
		"package_display_name": "My Package",
		"package_name": "{{cookiecutter.package_display_name.lower().replace(' ', '_').replace('-', '_')}}",
		"github_repo": "{{ cookiecutter.package_name }}"
	}`

	config, _, _, err := ConvertCookiecutterConfig([]byte(input))
	require.NoError(t, err)

	// Check that cookiecutter namespace was converted to vars namespace in defaults
	assert.Equal(t, "{{vars.package_display_name.lower().replace(' ', '_').replace('-', '_')}}", config.RawVars["package_name"])
	assert.Equal(t, "{{ vars.package_name }}", config.RawVars["github_repo"])
}

func TestUT_ConvertCookiecutterConfig_BooleanDefaults(t *testing.T) {
	input := `{
		"use_docker": true,
		"include_tests": false
	}`

	config, conversions, warnings, err := ConvertCookiecutterConfig([]byte(input))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Len(t, conversions, 2)

	// Check boolean type
	varDef := config.Vars["use_docker"]
	assert.Equal(t, scaffold.VarTypeBoolean, varDef.Type)
	assert.Equal(t, true, varDef.Default)

	varDef = config.Vars["include_tests"]
	assert.Equal(t, scaffold.VarTypeBoolean, varDef.Type)
	assert.Equal(t, false, varDef.Default)
}

func TestUT_ConvertCookiecutterConfig_NumberDefaults(t *testing.T) {
	input := `{
		"port": 8080,
		"timeout": 30.5
	}`

	config, conversions, warnings, err := ConvertCookiecutterConfig([]byte(input))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Len(t, conversions, 2)

	// Check number type
	varDef := config.Vars["port"]
	assert.Equal(t, scaffold.VarTypeNumber, varDef.Type)
	assert.Equal(t, float64(8080), varDef.Default)
}

func TestUT_ConvertCookiecutterConfig_ChoiceVariables(t *testing.T) {
	input := `{
		"license": ["MIT", "BSD-3", "Apache-2.0"],
		"python_version": ["3.11", "3.10", "3.9"]
	}`

	config, conversions, warnings, err := ConvertCookiecutterConfig([]byte(input))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Len(t, conversions, 2)

	// Check choice type
	varDef := config.Vars["license"]
	assert.Equal(t, scaffold.VarTypeChoice, varDef.Type)
	assert.Equal(t, []string{"MIT", "BSD-3", "Apache-2.0"}, varDef.Options)
	assert.Equal(t, "MIT", varDef.Default) // First option is default
}

func TestUT_ConvertCookiecutterConfig_PrivateVariables(t *testing.T) {
	input := `{
		"_project_slug": "my-project",
		"project_name": "My Project"
	}`

	config, conversions, _, err := ConvertCookiecutterConfig([]byte(input))
	require.NoError(t, err)
	assert.Len(t, conversions, 2)

	// Find private variable conversion
	var privateConv VariableConversion
	for _, c := range conversions {
		if c.Name == "_project_slug" {
			privateConv = c
			break
		}
	}
	assert.True(t, privateConv.IsPrivate)
	assert.Contains(t, config.RawVars, "_project_slug")
}

func TestUT_ConvertCookiecutterConfig_SpecialKeys(t *testing.T) {
	input := `{
		"project_name": "test",
		"_copy_without_render": ["*.png", "*.jpg"],
		"_extensions": ["jinja2_time.TimeExtension"]
	}`

	config, conversions, warnings, err := ConvertCookiecutterConfig([]byte(input))
	require.NoError(t, err)

	// Special keys should be skipped with warnings
	assert.Len(t, conversions, 1)
	assert.Len(t, warnings, 2)
	assert.NotContains(t, config.RawVars, "_copy_without_render")
	assert.NotContains(t, config.RawVars, "_extensions")
}

func TestUT_ConvertCookiecutterConfig_NullDefault(t *testing.T) {
	input := `{
		"optional_field": null
	}`

	config, conversions, warnings, err := ConvertCookiecutterConfig([]byte(input))
	require.NoError(t, err)
	assert.Len(t, conversions, 1)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "null default")

	// Should be converted to empty string
	assert.Equal(t, "", config.RawVars["optional_field"])
}

func TestUT_ConvertCookiecutterConfig_NestedObject(t *testing.T) {
	input := `{
		"database": {
			"host": "localhost",
			"port": 5432
		}
	}`

	config, conversions, warnings, err := ConvertCookiecutterConfig([]byte(input))
	require.NoError(t, err)
	assert.Len(t, conversions, 1)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "nested object")

	// Should be converted to JSON string
	val, ok := config.RawVars["database"].(string)
	assert.True(t, ok)
	assert.Contains(t, val, "localhost")
}

func TestUT_ConvertCookiecutterConfig_InvalidJSON(t *testing.T) {
	input := `{ invalid json }`

	_, _, _, err := ConvertCookiecutterConfig([]byte(input))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidConfig)
}

func TestUT_ConvertCookiecutterConfig_EmptyConfig(t *testing.T) {
	input := `{}`

	config, conversions, warnings, err := ConvertCookiecutterConfig([]byte(input))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Empty(t, conversions)
	assert.Empty(t, config.RawVars)
}

func TestUT_GenerateTagTemplateJSON(t *testing.T) {
	config := &scaffold.TemplateConfig{
		RawVars: map[string]any{
			"project_name": "test",
			"use_docker": map[string]any{
				"type":    "boolean",
				"default": true,
			},
		},
		Hooks: &scaffold.HooksConfig{
			PreScaffold:  []string{"echo pre"},
			PostScaffold: []string{"echo post"},
		},
	}

	data, err := GenerateTagTemplateJSON(config, "My Template", "A test template")
	require.NoError(t, err)

	// Verify JSON structure
	assert.Contains(t, string(data), `"name": "My Template"`)
	assert.Contains(t, string(data), `"description": "A test template"`)
	assert.Contains(t, string(data), `"project_name": "test"`)
	assert.Contains(t, string(data), `"pre_scaffold"`)
	assert.Contains(t, string(data), `"post_scaffold"`)
}

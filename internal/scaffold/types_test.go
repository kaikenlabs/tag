package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ParseTemplateConfig_ShortForm(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantVars map[string]VariableDef
	}{
		{
			name: "short form string",
			json: `{"vars": {"project_name": "my_project"}}`,
			wantVars: map[string]VariableDef{
				"project_name": {Type: VarTypeString, Default: "my_project"},
			},
		},
		{
			name: "short form boolean",
			json: `{"vars": {"use_docker": true}}`,
			wantVars: map[string]VariableDef{
				"use_docker": {Type: VarTypeBoolean, Default: true},
			},
		},
		{
			name: "short form number",
			json: `{"vars": {"port": 8080}}`,
			wantVars: map[string]VariableDef{
				"port": {Type: VarTypeNumber, Default: float64(8080)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseTemplateConfig([]byte(tt.json))
			require.NoError(t, err)
			assert.Equal(t, tt.wantVars, config.Vars)
		})
	}
}

func TestUT_ParseTemplateConfig_LongForm(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantVar VariableDef
		varName string
	}{
		{
			name: "long form string with all fields",
			json: `{
				"vars": {
					"author": {
						"type": "string",
						"prompt": "Who is the author?",
						"default": "Your Name",
						"required": true
					}
				}
			}`,
			varName: "author",
			wantVar: VariableDef{
				Type:     VarTypeString,
				Prompt:   "Who is the author?",
				Default:  "Your Name",
				Required: true,
			},
		},
		{
			name: "long form choice",
			json: `{
				"vars": {
					"license": {
						"type": "choice",
						"prompt": "Select a license",
						"options": ["MIT", "BSD-3", "Apache-2.0"],
						"default": "MIT"
					}
				}
			}`,
			varName: "license",
			wantVar: VariableDef{
				Type:    VarTypeChoice,
				Prompt:  "Select a license",
				Options: []string{"MIT", "BSD-3", "Apache-2.0"},
				Default: "MIT",
			},
		},
		{
			name: "long form boolean",
			json: `{
				"vars": {
					"use_docker": {
						"type": "boolean",
						"prompt": "Include Docker setup?",
						"default": false
					}
				}
			}`,
			varName: "use_docker",
			wantVar: VariableDef{
				Type:    VarTypeBoolean,
				Prompt:  "Include Docker setup?",
				Default: false,
			},
		},
		{
			name: "long form number",
			json: `{
				"vars": {
					"port": {
						"type": "number",
						"prompt": "Server port",
						"default": 8080
					}
				}
			}`,
			varName: "port",
			wantVar: VariableDef{
				Type:    VarTypeNumber,
				Prompt:  "Server port",
				Default: float64(8080),
			},
		},
		{
			name: "long form with secret",
			json: `{
				"vars": {
					"api_key": {
						"type": "string",
						"prompt": "Enter API key",
						"secret": true,
						"required": true
					}
				}
			}`,
			varName: "api_key",
			wantVar: VariableDef{
				Type:     VarTypeString,
				Prompt:   "Enter API key",
				Secret:   true,
				Required: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseTemplateConfig([]byte(tt.json))
			require.NoError(t, err)
			assert.Equal(t, tt.wantVar, config.Vars[tt.varName])
		})
	}
}

func TestUT_ParseTemplateConfig_Metadata(t *testing.T) {
	json := `{
		"name": "go-service",
		"description": "A Go microservice template",
		"version": "1.0.0",
		"vars": {},
		"hooks": {
			"pre_scaffold": ["echo 'Starting'"],
			"post_scaffold": ["go mod tidy"]
		}
	}`

	config, err := ParseTemplateConfig([]byte(json))
	require.NoError(t, err)

	assert.Equal(t, "go-service", config.Name)
	assert.Equal(t, "A Go microservice template", config.Description)
	assert.Equal(t, "1.0.0", config.Version)
	require.NotNil(t, config.Hooks)
	assert.Equal(t, []string{"echo 'Starting'"}, config.Hooks.PreScaffold)
	assert.Equal(t, []string{"go mod tidy"}, config.Hooks.PostScaffold)
}

func TestUT_ParseTemplateConfig_ChoiceWithoutOptions(t *testing.T) {
	json := `{
		"vars": {
			"license": {
				"type": "choice",
				"prompt": "Select a license"
			}
		}
	}`

	_, err := ParseTemplateConfig([]byte(json))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must have options")
}

func TestUT_ParseTemplateConfig_InvalidJSON(t *testing.T) {
	json := `{invalid json}`

	_, err := ParseTemplateConfig([]byte(json))
	require.Error(t, err)
}

func TestUT_VariableDef_IsPrivate(t *testing.T) {
	tests := []struct {
		name     string
		varName  string
		expected bool
	}{
		{"private variable", "_project_slug", true},
		{"public variable", "project_name", false},
		{"empty name", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := VariableDef{}
			assert.Equal(t, tt.expected, def.IsPrivate(tt.varName))
		})
	}
}

func TestUT_VariableDef_GetPrompt(t *testing.T) {
	tests := []struct {
		name     string
		def      VariableDef
		varName  string
		expected string
	}{
		{
			name:     "custom prompt",
			def:      VariableDef{Prompt: "Enter your name:"},
			varName:  "author",
			expected: "Enter your name:",
		},
		{
			name:     "default prompt",
			def:      VariableDef{},
			varName:  "author",
			expected: "Enter value for author",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.def.GetPrompt(tt.varName))
		})
	}
}

func TestUT_VariableDef_IsDerived(t *testing.T) {
	tests := []struct {
		name     string
		def      VariableDef
		expected bool
	}{
		{
			name:     "simple string default - not derived",
			def:      VariableDef{Default: "my_project"},
			expected: false,
		},
		{
			name:     "vars namespace - derived",
			def:      VariableDef{Default: "{{ vars.project_name }}"},
			expected: true,
		},
		{
			name:     "vars namespace no spaces - derived",
			def:      VariableDef{Default: "{{vars.project_name}}"},
			expected: true,
		},
		{
			name:     "cookiecutter namespace - derived",
			def:      VariableDef{Default: "{{ cookiecutter.project_name }}"},
			expected: true,
		},
		{
			name:     "cookiecutter with method calls - derived",
			def:      VariableDef{Default: "{{cookiecutter.package_display_name.lower().replace(' ', '_')}}"},
			expected: true,
		},
		{
			name:     "vars with filter - derived",
			def:      VariableDef{Default: "{{ vars.project_name | snake }}"},
			expected: true,
		},
		{
			name:     "nil default - not derived",
			def:      VariableDef{Default: nil},
			expected: false,
		},
		{
			name:     "boolean default - not derived",
			def:      VariableDef{Default: true},
			expected: false,
		},
		{
			name:     "number default - not derived",
			def:      VariableDef{Default: float64(8080)},
			expected: false,
		},
		{
			name:     "empty string - not derived",
			def:      VariableDef{Default: ""},
			expected: false,
		},
		{
			name:     "string with braces but no vars - not derived",
			def:      VariableDef{Default: "{{ some_other_thing }}"},
			expected: false,
		},
		{
			name:     "jinja2 now tag - not derived",
			def:      VariableDef{Default: "{% now 'utc', '%Y' %}"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.def.IsDerived())
		})
	}
}

func TestUT_ContainsTemplateExpression(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"vars with spaces", "{{ vars.name }}", true},
		{"vars without spaces", "{{vars.name}}", true},
		{"cookiecutter with spaces", "{{ cookiecutter.name }}", true},
		{"cookiecutter without spaces", "{{cookiecutter.name}}", true},
		{"complex expression", "{{vars.package_display_name.lower().replace(' ', '_')}}", true},
		{"with filter", "{{ vars.name | snake }}", true},
		{"plain string", "hello world", false},
		{"empty string", "", false},
		{"braces but no vars", "{{ something_else }}", false},
		{"partial match vars", "{{ variable }}", false},
		{"just braces", "{{}}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, containsTemplateExpression(tt.input))
		})
	}
}

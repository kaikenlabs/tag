package tmplconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

// ---------------------------------------------------------------------------
// ParseTemplateConfig
// ---------------------------------------------------------------------------

func TestUT_ParseTemplateConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		assert    func(t *testing.T, cfg *TemplateConfig)
		wantErr   bool
		errSubstr string
	}{
		{
			name:  "valid JSON with name description and short-form vars",
			input: `{"name":"tpl","description":"A template","vars":{"project_name":"my-proj","use_docker":true,"port":8080}}`,
			assert: func(t *testing.T, cfg *TemplateConfig) {
				t.Helper()
				assert.Equal(t, "tpl", cfg.Name)
				assert.Equal(t, "A template", cfg.Description)
				require.Len(t, cfg.Vars, 3)

				assert.Equal(t, VarTypeString, cfg.Vars["project_name"].Type)
				assert.Equal(t, "my-proj", cfg.Vars["project_name"].Default)

				assert.Equal(t, VarTypeBoolean, cfg.Vars["use_docker"].Type)
				assert.Equal(t, true, cfg.Vars["use_docker"].Default)

				assert.Equal(t, VarTypeNumber, cfg.Vars["port"].Type)
				assert.Equal(t, float64(8080), cfg.Vars["port"].Default)
			},
		},
		{
			name: "long-form variable with all fields",
			input: `{
				"vars": {
					"db_type": {
						"type": "choice",
						"prompt": "Select database",
						"default": "postgres",
						"required": true,
						"secret": true,
						"options": ["postgres", "mysql", "sqlite"]
					}
				}
			}`,
			assert: func(t *testing.T, cfg *TemplateConfig) {
				t.Helper()
				require.Contains(t, cfg.Vars, "db_type")
				v := cfg.Vars["db_type"]
				assert.Equal(t, VarTypeChoice, v.Type)
				assert.Equal(t, "Select database", v.Prompt)
				assert.Equal(t, "postgres", v.Default)
				assert.True(t, v.Required)
				assert.True(t, v.Secret)
				assert.Equal(t, []string{"postgres", "mysql", "sqlite"}, v.Options)
			},
		},
		{
			name:      "choice variable missing options returns error",
			input:     `{"vars":{"db_type":{"type":"choice"}}}`,
			wantErr:   true,
			errSubstr: "must have options",
		},
		{
			name:      "invalid JSON returns error",
			input:     `{invalid`,
			wantErr:   true,
			errSubstr: "failed to parse template config",
		},
		{
			name:      "unsupported variable type returns error",
			input:     `{"vars":{"items":["a","b"]}}`,
			wantErr:   true,
			errSubstr: "unsupported variable format",
		},
		{
			name:  "empty vars object produces empty Vars map",
			input: `{"vars":{}}`,
			assert: func(t *testing.T, cfg *TemplateConfig) {
				t.Helper()
				require.NotNil(t, cfg.Vars)
				assert.Empty(t, cfg.Vars)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := ParseTemplateConfig([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}
			require.NoError(t, err)
			if tt.assert != nil {
				tt.assert(t, cfg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ParseTestConfig
// ---------------------------------------------------------------------------

func TestUT_ParseTestConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		assert    func(t *testing.T, tc *TestConfig)
		wantNil   bool
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid test config with cases",
			input: `{
				"test": {
					"project_name": "test-proj",
					"cases": [
						{"name": "default", "commands": ["go build ./..."]}
					],
					"env": {"CI": "true"}
				}
			}`,
			assert: func(t *testing.T, tc *TestConfig) {
				t.Helper()
				assert.Equal(t, "test-proj", tc.ProjectName)
				require.Len(t, tc.Cases, 1)
				assert.Equal(t, "default", tc.Cases[0].Name)
				assert.Equal(t, []string{"go build ./..."}, tc.Cases[0].Commands)
				assert.Equal(t, "true", tc.Env["CI"])
			},
		},
		{
			name:    "no test key returns nil",
			input:   `{"name": "mytemplate"}`,
			wantNil: true,
		},
		{
			name:    "invalid JSON returns error",
			input:   `{bad`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tc, err := ParseTestConfig([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, tc)
				return
			}
			require.NotNil(t, tc)
			if tt.assert != nil {
				tt.assert(t, tc)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ContainsTemplateExpression
// ---------------------------------------------------------------------------

func TestUT_ContainsTemplateExpression(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "has double braces and vars dot", input: "{{ vars.name }}", want: true},
		{name: "has double braces but no vars dot", input: "{{ foo }}", want: false},
		{name: "has vars dot but no double braces", input: "vars.name", want: false},
		{name: "empty string", input: "", want: false},
		{name: "vars dot embedded in braces", input: "prefix-{{ vars.x }}-suffix", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ContainsTemplateExpression(tt.input))
		})
	}
}

// ---------------------------------------------------------------------------
// VariableDef.IsPrivate
// ---------------------------------------------------------------------------

func TestUT_VariableDef_IsPrivate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		varName  string
		expected bool
	}{
		{name: "underscore prefix is private", varName: "_foo", expected: true},
		{name: "no underscore prefix is not private", varName: "foo", expected: false},
		{name: "empty string is not private", varName: "", expected: false},
	}

	v := VariableDef{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, v.IsPrivate(tt.varName))
		})
	}
}

// ---------------------------------------------------------------------------
// VariableDef.IsDerived
// ---------------------------------------------------------------------------

func TestUT_VariableDef_IsDerived(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		varDef   VariableDef
		expected bool
	}{
		{
			name:     "template expression default without prompt is derived",
			varDef:   VariableDef{Default: "{{ vars.project_name }}_api"},
			expected: true,
		},
		{
			name:     "template expression default with prompt is not derived",
			varDef:   VariableDef{Default: "{{ vars.name }}", Prompt: "Enter name"},
			expected: false,
		},
		{
			name:     "nil default is not derived",
			varDef:   VariableDef{},
			expected: false,
		},
		{
			name:     "non-string default is not derived",
			varDef:   VariableDef{Default: 42},
			expected: false,
		},
		{
			name:     "plain string default is not derived",
			varDef:   VariableDef{Default: "plain"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.varDef.IsDerived())
		})
	}
}

// ---------------------------------------------------------------------------
// VariableDef.IsEvaluatedDefault
// ---------------------------------------------------------------------------

func TestUT_VariableDef_IsEvaluatedDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		varDef   VariableDef
		expected bool
	}{
		{
			name:     "prompt with template expression default is evaluated",
			varDef:   VariableDef{Prompt: "API name", Default: "{{ vars.project_name }}_api"},
			expected: true,
		},
		{
			name:     "no prompt is not evaluated",
			varDef:   VariableDef{Default: "{{ vars.name }}"},
			expected: false,
		},
		{
			name:     "nil default is not evaluated",
			varDef:   VariableDef{Prompt: "Name?"},
			expected: false,
		},
		{
			name:     "non-string default is not evaluated",
			varDef:   VariableDef{Prompt: "Count?", Default: 5},
			expected: false,
		},
		{
			name:     "prompt with plain default is not evaluated",
			varDef:   VariableDef{Prompt: "Name?", Default: "plain"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.varDef.IsEvaluatedDefault())
		})
	}
}

// ---------------------------------------------------------------------------
// VariableDef.GetPrompt
// ---------------------------------------------------------------------------

func TestUT_VariableDef_GetPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		varDef   VariableDef
		varName  string
		expected string
	}{
		{
			name:     "custom prompt is returned",
			varDef:   VariableDef{Prompt: "What is the project name?"},
			varName:  "project_name",
			expected: "What is the project name?",
		},
		{
			name:     "empty prompt returns default message",
			varDef:   VariableDef{},
			varName:  "project_name",
			expected: "Enter value for project_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.varDef.GetPrompt(tt.varName))
		})
	}
}

// ---------------------------------------------------------------------------
// IsCookiecutterTemplate
// ---------------------------------------------------------------------------

func TestUT_IsCookiecutterTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setup        func(t *testing.T, dir string)
		wantOK       bool
		wantNonEmpty bool
	}{
		{
			name: "directory with cookiecutter.json returns path and true",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, types.CookiecutterConfigFile),
					[]byte(`{}`), 0o644,
				))
			},
			wantOK:       true,
			wantNonEmpty: true,
		},
		{
			name:         "directory without cookiecutter.json returns empty and false",
			setup:        func(_ *testing.T, _ string) {},
			wantOK:       false,
			wantNonEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			tt.setup(t, dir)

			path, ok := IsCookiecutterTemplate(dir)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantNonEmpty {
				assert.Contains(t, path, types.CookiecutterConfigFile)
			} else {
				assert.Empty(t, path)
			}
		})
	}
}

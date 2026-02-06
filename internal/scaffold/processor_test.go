package scaffold

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_PathProcessor_SimpleVar(t *testing.T) {
	processor := NewPathProcessor()
	vars := map[string]any{
		"project_name": "my_project",
		"module_name":  "users",
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "single placeholder",
			path:     "{{ vars.project_name }}",
			expected: "my_project",
		},
		{
			name:     "placeholder in path",
			path:     "{{ vars.project_name }}/cmd/main.go",
			expected: "my_project/cmd/main.go",
		},
		{
			name:     "placeholder in filename",
			path:     "internal/{{ vars.module_name }}.go",
			expected: "internal/users.go",
		},
		{
			name:     "multiple placeholders",
			path:     "{{ vars.project_name }}/internal/{{ vars.module_name }}/service.go",
			expected: "my_project/internal/users/service.go",
		},
		{
			name:     "no placeholders",
			path:     "cmd/main.go",
			expected: "cmd/main.go",
		},
		{
			name:     "python __init__.py not a placeholder",
			path:     "{{ vars.project_name }}/src/__init__.py",
			expected: "my_project/src/__init__.py",
		},
		{
			name:     "cookiecutter alias",
			path:     "{{ cookiecutter.project_name }}/main.go",
			expected: "my_project/main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPath(tt.path, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_PathProcessor_WithFilter(t *testing.T) {
	processor := NewPathProcessor()
	vars := map[string]any{
		"project_name": "MyProject",
		"model":        "user",
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "snake filter",
			path:     "{{ vars.project_name | snake }}",
			expected: "my_project",
		},
		{
			name:     "pascal filter",
			path:     "{{ vars.project_name | pascal }}",
			expected: "MyProject",
		},
		{
			name:     "camel filter",
			path:     "{{ vars.project_name | camel }}",
			expected: "myProject",
		},
		{
			name:     "kebab filter",
			path:     "{{ vars.project_name | kebab }}",
			expected: "my-project",
		},
		{
			name:     "lower filter",
			path:     "{{ vars.project_name | lower }}",
			expected: "myproject",
		},
		{
			name:     "upper filter",
			path:     "{{ vars.project_name | upper }}",
			expected: "MYPROJECT",
		},
		{
			name:     "plural filter",
			path:     "{{ vars.model | plural }}",
			expected: "users",
		},
		{
			name:     "singular filter from plural",
			path:     "users/{{ vars.model | singular }}",
			expected: "users/user",
		},
		{
			name:     "filter without spaces",
			path:     "{{vars.project_name|snake}}",
			expected: "my_project",
		},
		{
			name:     "filter in complex path",
			path:     "{{ vars.project_name | snake }}/internal/{{ vars.model | plural }}/service.go",
			expected: "my_project/internal/users/service.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPath(tt.path, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_PathProcessor_UndefinedVar(t *testing.T) {
	processor := NewPathProcessor()
	vars := map[string]any{
		"project_name": "my_project",
	}

	// Gonja returns empty string for undefined variables (consistent with content templates)
	result, err := processor.ProcessPath("{{ vars.unknown_var }}/file.go", vars)
	require.NoError(t, err)
	assert.Equal(t, "file.go", result) // Empty segment is skipped
}

func TestUT_PathProcessor_InvalidFilter(t *testing.T) {
	processor := NewPathProcessor()
	vars := map[string]any{
		"project_name": "my_project",
	}

	_, err := processor.ProcessPath("{{ vars.project_name | invalid_filter }}", vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filter")
	assert.Contains(t, err.Error(), "invalid_filter")
}

func TestUT_PathProcessor_EmptyValue(t *testing.T) {
	processor := NewPathProcessor()
	vars := map[string]any{
		"project_name": "",
		"module_name":  "users",
	}

	// Empty value in path segment should collapse that segment
	result, err := processor.ProcessPath("{{ vars.project_name }}/{{ vars.module_name }}/service.go", vars)
	require.NoError(t, err)
	// Note: empty segment is skipped, so result doesn't start with "/"
	assert.Equal(t, "users/service.go", result)
}

func TestUT_PathProcessor_MultiplePlaceholdersInSegment(t *testing.T) {
	processor := NewPathProcessor()
	vars := map[string]any{
		"prefix": "api",
		"suffix": "v1",
	}

	result, err := processor.ProcessPath("{{ vars.prefix }}-{{ vars.suffix }}/handler.go", vars)
	require.NoError(t, err)
	assert.Equal(t, "api-v1/handler.go", result)
}

func TestUT_PathProcessor_ComplexExpressions(t *testing.T) {
	processor := NewPathProcessor()
	vars := map[string]any{
		"package_display_name": "My Cool Package",
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "lower method",
			path:     "{{ vars.package_display_name.lower() }}",
			expected: "my cool package",
		},
		{
			name:     "replace filter",
			path:     "{{ vars.package_display_name | replace(' ', '_') }}",
			expected: "My_Cool_Package",
		},
		{
			name:     "chained filters (recommended approach)",
			path:     "{{ cookiecutter.package_display_name | lower | replace(' ', '_') | replace('-', '_') }}",
			expected: "my_cool_package",
		},
		{
			name:     "upper method",
			path:     "{{ vars.package_display_name.upper() }}",
			expected: "MY COOL PACKAGE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPath(tt.path, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_ExtractPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "single placeholder",
			path:     "{{ vars.project_name }}",
			expected: []string{"project_name"},
		},
		{
			name:     "multiple placeholders",
			path:     "{{ vars.project_name }}/{{ vars.module_name }}",
			expected: []string{"project_name", "module_name"},
		},
		{
			name:     "placeholder with filter",
			path:     "{{ vars.project_name | snake }}",
			expected: []string{"project_name"},
		},
		{
			name:     "no placeholders",
			path:     "cmd/main.go",
			expected: []string{},
		},
		{
			name:     "duplicate placeholders",
			path:     "{{ vars.name }}/{{ vars.name }}",
			expected: []string{"name"},
		},
		{
			name:     "python dunder not a placeholder",
			path:     "__init__.py",
			expected: []string{},
		},
		{
			name:     "cookiecutter alias",
			path:     "{{ cookiecutter.name }}",
			expected: []string{"name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPlaceholders(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_HasPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"has placeholder", "{{ vars.name }}", true},
		{"has placeholder with filter", "{{ vars.name | snake }}", true},
		{"cookiecutter alias", "{{ cookiecutter.name }}", true},
		{"no placeholder", "cmd/main.go", false},
		{"python dunder not a placeholder", "__init__.py", false},
		{"python main not a placeholder", "__main__.py", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasPlaceholders(tt.path))
		})
	}
}

func TestUT_StripTemplateExtension(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{"has .tmpl", "main.go.tmpl", "main.go"},
		{"no .tmpl", "main.go", "main.go"},
		{"only .tmpl", ".tmpl", ""},
		{"double extension", "config.yaml.tmpl", "config.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, StripTemplateExtension(tt.filename))
		})
	}
}

func TestUT_IsTemplateFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{"is template", "main.go.tmpl", true},
		{"not template", "main.go", false},
		{"only .tmpl", ".tmpl", true},
		{"yaml template", "config.yaml.tmpl", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsTemplateFile(tt.filename))
		})
	}
}

func TestUT_PathProcessor_WhitespaceAroundFilter(t *testing.T) {
	processor := NewPathProcessor()
	vars := map[string]any{
		"project_name": "MyProject",
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"no spaces", "{{vars.project_name|snake}}", "my_project"},
		{"space before pipe", "{{ vars.project_name |snake }}", "my_project"},
		{"space after pipe", "{{ vars.project_name| snake }}", "my_project"},
		{"spaces both sides", "{{ vars.project_name | snake }}", "my_project"},
		{"multiple spaces", "{{  vars.project_name  |  snake  }}", "my_project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPath(tt.path, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

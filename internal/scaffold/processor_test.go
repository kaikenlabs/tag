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
			path:     "__project_name__",
			expected: "my_project",
		},
		{
			name:     "placeholder in path",
			path:     "__project_name__/cmd/main.go",
			expected: "my_project/cmd/main.go",
		},
		{
			name:     "placeholder in filename",
			path:     "internal/__module_name__.go",
			expected: "internal/users.go",
		},
		{
			name:     "multiple placeholders",
			path:     "__project_name__/internal/__module_name__/service.go",
			expected: "my_project/internal/users/service.go",
		},
		{
			name:     "no placeholders",
			path:     "cmd/main.go",
			expected: "cmd/main.go",
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
			path:     "__project_name | snake__",
			expected: "my_project",
		},
		{
			name:     "pascal filter",
			path:     "__project_name | pascal__",
			expected: "MyProject",
		},
		{
			name:     "camel filter",
			path:     "__project_name | camel__",
			expected: "myProject",
		},
		{
			name:     "kebab filter",
			path:     "__project_name | kebab__",
			expected: "my-project",
		},
		{
			name:     "lower filter",
			path:     "__project_name | lower__",
			expected: "myproject",
		},
		{
			name:     "upper filter",
			path:     "__project_name | upper__",
			expected: "MYPROJECT",
		},
		{
			name:     "plural filter",
			path:     "__model | plural__",
			expected: "users",
		},
		{
			name:     "singular filter from plural",
			path:     "users/__model | singular__",
			expected: "users/user",
		},
		{
			name:     "filter without spaces",
			path:     "__project_name|snake__",
			expected: "my_project",
		},
		{
			name:     "filter in complex path",
			path:     "__project_name | snake__/internal/__model | plural__/service.go",
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

func TestUT_PathProcessor_InvalidVar(t *testing.T) {
	processor := NewPathProcessor()
	vars := map[string]any{
		"project_name": "my_project",
	}

	_, err := processor.ProcessPath("__unknown_var__/file.go", vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "undefined variable")
	assert.Contains(t, err.Error(), "unknown_var")
}

func TestUT_PathProcessor_InvalidFilter(t *testing.T) {
	processor := NewPathProcessor()
	vars := map[string]any{
		"project_name": "my_project",
	}

	_, err := processor.ProcessPath("__project_name | invalid_filter__", vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown or unsupported filter")
	assert.Contains(t, err.Error(), "invalid_filter")
}

func TestUT_PathProcessor_EmptyValue(t *testing.T) {
	processor := NewPathProcessor()
	vars := map[string]any{
		"project_name": "",
		"module_name":  "users",
	}

	// Empty value in path segment should collapse that segment
	result, err := processor.ProcessPath("__project_name__/__module_name__/service.go", vars)
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

	result, err := processor.ProcessPath("__prefix__-__suffix__/handler.go", vars)
	require.NoError(t, err)
	assert.Equal(t, "api-v1/handler.go", result)
}

func TestUT_ExtractPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "single placeholder",
			path:     "__project_name__",
			expected: []string{"project_name"},
		},
		{
			name:     "multiple placeholders",
			path:     "__project_name__/__module_name__",
			expected: []string{"project_name", "module_name"},
		},
		{
			name:     "placeholder with filter",
			path:     "__project_name | snake__",
			expected: []string{"project_name"},
		},
		{
			name:     "no placeholders",
			path:     "cmd/main.go",
			expected: []string{},
		},
		{
			name:     "duplicate placeholders",
			path:     "__name__/__name__",
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
		{"has placeholder", "__name__", true},
		{"has placeholder with filter", "__name | snake__", true},
		{"no placeholder", "cmd/main.go", false},
		{"similar but not placeholder", "__name", false},
		{"similar but not placeholder 2", "name__", false},
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
		{"no spaces", "__project_name|snake__", "my_project"},
		{"space before pipe", "__project_name |snake__", "my_project"},
		{"space after pipe", "__project_name| snake__", "my_project"},
		{"spaces both sides", "__project_name | snake__", "my_project"},
		{"multiple spaces", "__project_name  |  snake__", "my_project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPath(tt.path, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

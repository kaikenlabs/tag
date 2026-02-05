package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_ConvertPath_SimplePlaceholder(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		changed  bool
	}{
		{
			name:     "single placeholder",
			input:    "{{ cookiecutter.project_name }}",
			expected: "__project_name__",
			changed:  true,
		},
		{
			name:     "placeholder in path",
			input:    "{{ cookiecutter.project_name }}/src/main.go",
			expected: "__project_name__/src/main.go",
			changed:  true,
		},
		{
			name:     "placeholder in filename",
			input:    "src/{{ cookiecutter.module_name }}.py",
			expected: "src/__module_name__.py",
			changed:  true,
		},
		{
			name:     "multiple placeholders",
			input:    "{{ cookiecutter.project_name }}/{{ cookiecutter.module_name }}/service.go",
			expected: "__project_name__/__module_name__/service.go",
			changed:  true,
		},
		{
			name:     "no placeholders",
			input:    "src/main.go",
			expected: "src/main.go",
			changed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed := ConvertPath(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.changed, changed)
		})
	}
}

func TestUT_ConvertPath_WithFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "snake filter",
			input:    "{{ cookiecutter.project_name | lower }}",
			expected: "__project_name | lower__",
		},
		{
			name:     "filter without spaces",
			input:    "{{ cookiecutter.project_name|upper }}",
			expected: "__project_name | upper__",
		},
		{
			name:     "filter with extra spaces",
			input:    "{{  cookiecutter.project_name  |  snake  }}",
			expected: "__project_name | snake__",
		},
		{
			name:     "filter in complex path",
			input:    "{{ cookiecutter.project_name | snake }}/internal/{{ cookiecutter.model | plural }}/handler.go",
			expected: "__project_name | snake__/internal/__model | plural__/handler.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed := ConvertPath(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.True(t, changed)
		})
	}
}

func TestUT_ConvertPath_WhitespaceVariants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no spaces",
			input:    "{{cookiecutter.var}}",
			expected: "__var__",
		},
		{
			name:     "single space",
			input:    "{{ cookiecutter.var }}",
			expected: "__var__",
		},
		{
			name:     "extra spaces",
			input:    "{{   cookiecutter.var   }}",
			expected: "__var__",
		},
		{
			name:     "mixed spacing with filter",
			input:    "{{ cookiecutter.var| lower}}",
			expected: "__var | lower__",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed := ConvertPath(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.True(t, changed)
		})
	}
}

func TestUT_ConvertPath_NoPlaceholders(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"plain path", "src/main.go"},
		{"path with dots", "package.json"},
		{"path with underscores", "my_module/__init__.py"},
		{"similar but not placeholder", "{{ not_cookiecutter.var }}"},
		{"broken placeholder", "{{ cookiecutter."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed := ConvertPath(tt.input)
			assert.Equal(t, tt.input, result)
			assert.False(t, changed)
		})
	}
}

func TestUT_ConvertPathWithDetails(t *testing.T) {
	input := "{{ cookiecutter.project_name }}/src/{{ cookiecutter.module | lower }}.go"

	result, conversions := ConvertPathWithDetails(input)

	assert.Equal(t, "__project_name__/src/__module | lower__.go", result)
	assert.Len(t, conversions, 2)

	assert.Equal(t, "{{ cookiecutter.project_name }}", conversions[0].From)
	assert.Equal(t, "__project_name__", conversions[0].To)

	assert.Equal(t, "{{ cookiecutter.module | lower }}", conversions[1].From)
	assert.Equal(t, "__module | lower__", conversions[1].To)
}

func TestUT_HasCookiecutterPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"has placeholder", "{{ cookiecutter.var }}", true},
		{"has placeholder with filter", "{{ cookiecutter.var | lower }}", true},
		{"no placeholder", "src/main.go", false},
		{"similar but different", "{{ vars.project }}", false},
		{"TAG placeholder", "__var__", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasCookiecutterPlaceholders(tt.input))
		})
	}
}

func TestUT_ExtractCookiecutterVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single var",
			input:    "{{ cookiecutter.project_name }}",
			expected: []string{"project_name"},
		},
		{
			name:     "multiple vars",
			input:    "{{ cookiecutter.project }}/{{ cookiecutter.module }}",
			expected: []string{"project", "module"},
		},
		{
			name:     "duplicate vars",
			input:    "{{ cookiecutter.name }}/{{ cookiecutter.name }}",
			expected: []string{"name"},
		},
		{
			name:     "no vars",
			input:    "src/main.go",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractCookiecutterVars(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_NormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"unix path", "src/main.go", "src/main.go"},
		{"windows path", "src\\main.go", "src/main.go"},
		{"leading slash", "/src/main.go", "src/main.go"},
		{"trailing slash", "src/main.go/", "src/main.go"},
		{"double slashes", "src//main.go", "src/main.go"},
		{"mixed", "/src\\\\main.go/", "src/main.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, NormalizePath(tt.input))
		})
	}
}

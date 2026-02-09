package convert

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// HasCookiecutterPlaceholders checks if a path contains {{ cookiecutter.* }} placeholders (test-only).
func HasCookiecutterPlaceholders(path string) bool {
	matches := expressionBlockRegex.FindAllString(path, -1)
	return slices.ContainsFunc(matches, cookiecutterNamespaceRegex.MatchString)
}

// simplePatternRegex matches simple {{ cookiecutter.var }} and {{ cookiecutter.var | filter }} patterns.
// Used for extracting variable names from simple patterns (test-only).
var simplePatternRegex = regexp.MustCompile(
	`\{\{\s*cookiecutter\.([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:\|\s*([a-zA-Z_][a-zA-Z0-9_]*))?\s*\}\}`,
)

// ConvertPathWithDetails returns detailed information about path conversions (test-only).
// This handles all cookiecutter expressions, including complex ones with method calls.
func ConvertPathWithDetails(path string) (string, []PathConversion) {
	var conversions []PathConversion

	matches := expressionBlockRegex.FindAllStringIndex(path, -1)
	if len(matches) == 0 {
		return path, nil
	}

	// Process matches from end to start to preserve indices during replacement
	result := path
	for i := len(matches) - 1; i >= 0; i-- {
		start, end := matches[i][0], matches[i][1]
		fullMatch := path[start:end]

		// Check if this block contains cookiecutter namespace (with word boundary)
		if cookiecutterNamespaceRegex.MatchString(fullMatch) {
			replacement := cookiecutterNamespaceRegex.ReplaceAllString(fullMatch, "${1}vars.")

			// Append conversion (will be reversed later to get original order)
			conversions = append(conversions, PathConversion{
				From: fullMatch,
				To:   replacement,
			})

			result = result[:start] + replacement + result[end:]
		}
	}

	// Reverse conversions to get original order (since we processed end to start)
	for i, j := 0, len(conversions)-1; i < j; i, j = i+1, j-1 {
		conversions[i], conversions[j] = conversions[j], conversions[i]
	}

	return result, conversions
}

// ExtractCookiecutterVars extracts variable names from simple {{ cookiecutter.var }} patterns (test-only).
// Note: This only extracts from simple patterns, not complex expressions like method calls.
func ExtractCookiecutterVars(path string) []string {
	matches := simplePatternRegex.FindAllStringSubmatch(path, -1)
	vars := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) >= 2 && !seen[match[1]] {
			vars = append(vars, match[1])
			seen[match[1]] = true
		}
	}

	return vars
}

// NormalizePath ensures consistent path separators and removes leading/trailing slashes (test-only).
func NormalizePath(path string) string {
	// Convert Windows separators to Unix
	path = strings.ReplaceAll(path, "\\", "/")

	// Remove leading/trailing slashes
	path = strings.Trim(path, "/")

	// Collapse multiple slashes
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	return path
}

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
			expected: "{{ vars.project_name }}",
			changed:  true,
		},
		{
			name:     "placeholder in path",
			input:    "{{ cookiecutter.project_name }}/src/main.go",
			expected: "{{ vars.project_name }}/src/main.go",
			changed:  true,
		},
		{
			name:     "placeholder in filename",
			input:    "src/{{ cookiecutter.module_name }}.py",
			expected: "src/{{ vars.module_name }}.py",
			changed:  true,
		},
		{
			name:     "multiple placeholders",
			input:    "{{ cookiecutter.project_name }}/{{ cookiecutter.module_name }}/service.go",
			expected: "{{ vars.project_name }}/{{ vars.module_name }}/service.go",
			changed:  true,
		},
		{
			name:     "no placeholders",
			input:    "src/main.go",
			expected: "src/main.go",
			changed:  false,
		},
		{
			name:     "python __init__.py preserved",
			input:    "{{ cookiecutter.project_name }}/src/__init__.py",
			expected: "{{ vars.project_name }}/src/__init__.py",
			changed:  true,
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
			name:     "lower filter",
			input:    "{{ cookiecutter.project_name | lower }}",
			expected: "{{ vars.project_name | lower }}",
		},
		{
			name:     "filter without spaces",
			input:    "{{ cookiecutter.project_name|upper }}",
			expected: "{{ vars.project_name|upper }}",
		},
		{
			name:     "filter with extra spaces",
			input:    "{{  cookiecutter.project_name  |  snake  }}",
			expected: "{{  vars.project_name  |  snake  }}",
		},
		{
			name:     "filter in complex path",
			input:    "{{ cookiecutter.project_name | snake }}/internal/{{ cookiecutter.model | plural }}/handler.go",
			expected: "{{ vars.project_name | snake }}/internal/{{ vars.model | plural }}/handler.go",
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
	// Note: The converter preserves whitespace as-is (no normalization).
	// Gonja handles whitespace variants correctly, so normalization is unnecessary.
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no spaces",
			input:    "{{cookiecutter.var}}",
			expected: "{{vars.var}}",
		},
		{
			name:     "single space",
			input:    "{{ cookiecutter.var }}",
			expected: "{{ vars.var }}",
		},
		{
			name:     "extra spaces",
			input:    "{{   cookiecutter.var   }}",
			expected: "{{   vars.var   }}",
		},
		{
			name:     "mixed spacing with filter",
			input:    "{{ cookiecutter.var| lower}}",
			expected: "{{ vars.var| lower}}",
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

	assert.Equal(t, "{{ vars.project_name }}/src/{{ vars.module | lower }}.go", result)
	assert.Len(t, conversions, 2)

	assert.Equal(t, "{{ cookiecutter.project_name }}", conversions[0].From)
	assert.Equal(t, "{{ vars.project_name }}", conversions[0].To)

	assert.Equal(t, "{{ cookiecutter.module | lower }}", conversions[1].From)
	assert.Equal(t, "{{ vars.module | lower }}", conversions[1].To)
}

func TestUT_ConvertPath_ComplexExpressions(t *testing.T) {
	// Test complex expressions that Cookiecutter templates actually use
	tests := []struct {
		name     string
		input    string
		expected string
		changed  bool
	}{
		{
			name:     "method chaining",
			input:    "{{cookiecutter.package_display_name.lower().replace(' ', '_').replace('-', '_')}}",
			expected: "{{vars.package_display_name.lower().replace(' ', '_').replace('-', '_')}}",
			changed:  true,
		},
		{
			name:     "method chaining with spaces",
			input:    "{{ cookiecutter.name.lower().replace(' ', '_') }}",
			expected: "{{ vars.name.lower().replace(' ', '_') }}",
			changed:  true,
		},
		{
			name:     "filter with method on result",
			input:    "{{ cookiecutter.project_name | lower | replace(' ', '_') }}",
			expected: "{{ vars.project_name | lower | replace(' ', '_') }}",
			changed:  true,
		},
		{
			name:     "mixed filters and methods",
			input:    "src/{{ cookiecutter.name.strip().lower() }}/{{ cookiecutter.module | snake }}",
			expected: "src/{{ vars.name.strip().lower() }}/{{ vars.module | snake }}",
			changed:  true,
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

func TestUT_HasCookiecutterPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"has placeholder", "{{ cookiecutter.var }}", true},
		{"has placeholder with filter", "{{ cookiecutter.var | lower }}", true},
		{"no placeholder", "src/main.go", false},
		{"vars namespace not cookiecutter", "{{ vars.project }}", false},
		{"python dunder not a placeholder", "__init__.py", false},
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

func TestUT_ConvertContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		changed  bool
	}{
		{
			name:     "expression block",
			input:    "{{ cookiecutter.name }}",
			expected: "{{ vars.name }}",
			changed:  true,
		},
		{
			name:     "control block if",
			input:    "{% if cookiecutter.use_docker %}yes{% endif %}",
			expected: "{% if vars.use_docker %}yes{% endif %}",
			changed:  true,
		},
		{
			name:     "control block for",
			input:    "{% for item in cookiecutter.items %}{{ item }}{% endfor %}",
			expected: "{% for item in vars.items %}{{ item }}{% endfor %}",
			changed:  true,
		},
		{
			name:     "comment block",
			input:    "{# cookiecutter.note #}",
			expected: "{# vars.note #}",
			changed:  true,
		},
		{
			name:     "mixed blocks",
			input:    "{% if cookiecutter.flag %}{{ cookiecutter.name }}{% endif %}",
			expected: "{% if vars.flag %}{{ vars.name }}{% endif %}",
			changed:  true,
		},
		{
			name:     "no cookiecutter references",
			input:    "{% if vars.flag %}{{ vars.name }}{% endif %}",
			expected: "{% if vars.flag %}{{ vars.name }}{% endif %}",
			changed:  false,
		},
		{
			name:     "plain text unchanged",
			input:    "Hello world",
			expected: "Hello world",
			changed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed := ConvertContent(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.changed, changed)
		})
	}
}

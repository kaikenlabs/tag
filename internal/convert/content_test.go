package convert

import (
	"testing"

	"github.com/kaikenlabs/tag/internal/fileutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_AnalyzeContent_FilterParentheses(t *testing.T) {
	analyzer := NewContentAnalyzer()

	tests := []struct {
		name          string
		content       string
		expectedCount int
		expectedKind  string
		hasSuggestion bool
	}{
		{
			name:          "default filter with parentheses",
			content:       `{{ name | default('Anonymous') }}`,
			expectedCount: 1,
			expectedKind:  "filter-syntax",
			hasSuggestion: true,
		},
		{
			name:          "replace filter",
			content:       `{{ name | replace('-', '_') }}`,
			expectedCount: 1,
			expectedKind:  "filter-syntax",
			hasSuggestion: true,
		},
		{
			name:          "multiple filters",
			content:       "{{ name | default('a') }}\n{{ other | format('%s') }}",
			expectedCount: 2,
			expectedKind:  "filter-syntax",
			hasSuggestion: true,
		},
		{
			name:          "gonja style colon - no warning",
			content:       `{{ name | default:"Anonymous" }}`,
			expectedCount: 0,
		},
		{
			name:          "filter without args - no warning",
			content:       `{{ name | lower }}`,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := analyzer.AnalyzeString("test.tmpl", tt.content)
			assert.Len(t, findings, tt.expectedCount)

			if tt.expectedCount > 0 {
				assert.Equal(t, tt.expectedKind, findings[0].Kind)
				if tt.hasSuggestion {
					assert.NotEmpty(t, findings[0].Suggestion)
				}
			}
		})
	}
}

func TestUT_AnalyzeContent_DictIteration(t *testing.T) {
	analyzer := NewContentAnalyzer()

	content := `{% for key, value in my_dict.items() %}
{{ key }}: {{ value }}
{% endfor %}`

	findings := analyzer.AnalyzeString("test.tmpl", content)

	require.Len(t, findings, 1)
	assert.Equal(t, "dict-iteration", findings[0].Kind)
	assert.Equal(t, 1, findings[0].Line)
	assert.Contains(t, findings[0].Suggestion, "{% for k, v in dict %}")
}

func TestUT_AnalyzeContent_MacroUsage(t *testing.T) {
	analyzer := NewContentAnalyzer()

	content := `{% macro input(name, type="text") %}
<input type="{{ type }}" name="{{ name }}">
{% endmacro %}`

	findings := analyzer.AnalyzeString("test.tmpl", content)

	require.Len(t, findings, 1)
	assert.Equal(t, "macro", findings[0].Kind)
	assert.Equal(t, 1, findings[0].Line)
}

func TestUT_AnalyzeContent_Extends(t *testing.T) {
	analyzer := NewContentAnalyzer()

	content := `{% extends "base.html" %}
{% block content %}
Hello World
{% endblock %}`

	findings := analyzer.AnalyzeString("test.tmpl", content)

	require.Len(t, findings, 1)
	assert.Equal(t, "extends", findings[0].Kind)
	assert.Equal(t, SeverityInfo, findings[0].Severity)
}

func TestUT_AnalyzeContent_Import(t *testing.T) {
	analyzer := NewContentAnalyzer()

	content := `{% import "macros.html" as macros %}
{{ macros.input("name") }}`

	findings := analyzer.AnalyzeString("test.tmpl", content)

	require.Len(t, findings, 1)
	assert.Equal(t, "import", findings[0].Kind)
}

func TestUT_AnalyzeContent_RawBlock(t *testing.T) {
	analyzer := NewContentAnalyzer()

	// Content inside raw block should NOT be analyzed
	content := `{% raw %}
{{ name | default('should not be flagged') }}
{% endraw %}
{{ other | default('this should be flagged') }}`

	findings := analyzer.AnalyzeString("test.tmpl", content)

	// Only the one outside raw block should be flagged
	require.Len(t, findings, 1)
	assert.Equal(t, 4, findings[0].Line) // Line 4, not inside raw block
}

func TestUT_AnalyzeContent_MultipleFindings(t *testing.T) {
	analyzer := NewContentAnalyzer()

	content := `{% extends "base.html" %}
{% import "macros.html" as m %}
{{ name | default('test') }}
{% for k, v in data.items() %}`

	findings := analyzer.AnalyzeString("test.tmpl", content)

	assert.Len(t, findings, 4)

	// Verify different kinds
	kinds := make(map[string]bool)
	for _, f := range findings {
		kinds[f.Kind] = true
	}
	assert.True(t, kinds["extends"])
	assert.True(t, kinds["import"])
	assert.True(t, kinds["filter-syntax"])
	assert.True(t, kinds["dict-iteration"])
}

func TestUT_AnalyzeContent_NoIncompatibilities(t *testing.T) {
	analyzer := NewContentAnalyzer()

	content := `{% if use_docker %}
FROM python:{{ python_version }}
{% endif %}

{{ project_name | snake }}
{{ author | upper }}

{% for item in items %}
- {{ item }}
{% endfor %}`

	findings := analyzer.AnalyzeString("test.tmpl", content)
	assert.Empty(t, findings)
}

func TestUT_AnalyzeContent_BinaryFile(t *testing.T) {
	analyzer := NewContentAnalyzer()

	// Binary content with null bytes
	content := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00, 0x00, 0x00}

	findings := analyzer.Analyze("image.png", content)
	assert.Empty(t, findings) // Should be skipped
}

func TestUT_isTextContent(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "valid utf8 text",
			content:  []byte("Hello, World!"),
			expected: true,
		},
		{
			name:     "text with newlines",
			content:  []byte("line1\nline2\nline3"),
			expected: true,
		},
		{
			name:     "empty content",
			content:  []byte{},
			expected: true,
		},
		{
			name:     "binary with null bytes",
			content:  []byte{0x00, 0x01, 0x02},
			expected: false,
		},
		{
			name:     "PNG header",
			content:  []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, fileutil.IsTextContent(tt.content))
		})
	}
}

func TestUT_AnalyzeContent_LineNumbers(t *testing.T) {
	analyzer := NewContentAnalyzer()

	content := `Line 1 - no issue
Line 2 - no issue
Line 3 - {{ name | default('test') }}
Line 4 - no issue
Line 5 - {{ other | format('%s') }}`

	findings := analyzer.AnalyzeString("test.tmpl", content)

	require.Len(t, findings, 2)
	assert.Equal(t, 3, findings[0].Line)
	assert.Equal(t, 5, findings[1].Line)
}

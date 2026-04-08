package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_FilterIndent_Basic(t *testing.T) {
	engine := MustNewEngine()

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "indent second line only",
			template: `{{ "line1\nline2\nline3" | indent(4) }}`,
			expected: "line1\n    line2\n    line3",
		},
		{
			name:     "indent all including first",
			template: `{{ "line1\nline2" | indent(4, true) }}`,
			expected: "    line1\n    line2",
		},
		{
			name:     "single line no change",
			template: `{{ "hello" | indent(4) }}`,
			expected: "hello",
		},
		{
			name:     "preserves empty lines",
			template: `{{ "line1\n\nline3" | indent(2) }}`,
			expected: "line1\n\n  line3",
		},
		{
			name:     "zero width",
			template: `{{ "line1\nline2" | indent(0) }}`,
			expected: "line1\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.ExecuteToString(tt.template, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterIndent_TooFewArgs(t *testing.T) {
	engine := MustNewEngine()
	_, err := engine.ExecuteToString(`{{ "test" | indent() }}`, nil)
	assert.Error(t, err)
}

func TestUT_FilterIndent_TooManyArgs(t *testing.T) {
	engine := MustNewEngine()
	_, err := engine.ExecuteToString(`{{ "test" | indent(4, true, "extra") }}`, nil)
	assert.Error(t, err)
}

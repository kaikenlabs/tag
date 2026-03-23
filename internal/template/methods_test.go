package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_CreateCustomStringMethods_ReplaceAll(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	// replace without count — should replace all occurrences
	tmpl := `{{ "hello world hello".replace("hello", "hi") }}`
	result, err := engine.ExecuteToString(tmpl, Context{})
	require.NoError(t, err)
	assert.Equal(t, "hi world hi", result)
}

func TestUT_CreateCustomStringMethods_ReplaceWithCount(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	// replace with count=1 — should replace only the first occurrence
	tmpl := `{{ "aaa".replace("a", "b", 1) }}`
	result, err := engine.ExecuteToString(tmpl, Context{})
	require.NoError(t, err)
	assert.Equal(t, "baa", result)
}

func TestUT_CreateCustomStringMethods_ReplaceWithNegativeCount(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	// replace with count=-1 — should replace all (same as no count)
	tmpl := `{{ "aaa".replace("a", "b", -1) }}`
	result, err := engine.ExecuteToString(tmpl, Context{})
	require.NoError(t, err)
	assert.Equal(t, "bbb", result)
}

func TestUT_CreateCustomStringMethods_BuiltinMethodsAvailable(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	tests := []struct {
		name     string
		tmpl     string
		expected string
	}{
		{"upper", `{{ "hello".upper() }}`, "HELLO"},
		{"lower", `{{ "HELLO".lower() }}`, "hello"},
		{"startswith", `{% if "hello".startswith("hel") %}yes{% else %}no{% endif %}`, "yes"},
		{"endswith", `{% if "hello".endswith("llo") %}yes{% else %}no{% endif %}`, "yes"},
		{"strip", `{{ "  hello  ".strip() }}`, "hello"},
		{"split join", `{{ "a,b,c".split(",") | join("-") }}`, "a-b-c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := engine.ExecuteToString(tt.tmpl, Context{})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_CreateCustomStringMethods_ReplaceWithZeroCount(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	// replace with count=0 — should replace nothing
	tmpl := `{{ "aaa".replace("a", "b", 0) }}`
	result, err := engine.ExecuteToString(tmpl, Context{})
	require.NoError(t, err)
	assert.Equal(t, "aaa", result)
}

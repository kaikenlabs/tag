package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_FilterSplit_TooManyArgs(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	tmpl := `{{ name|split(",", "x") }}`
	ctx := NewContext("a,b,c", nil)
	_, err := engine.ExecuteToString(tmpl, ctx)
	// split with 2 args should produce an error value
	assert.Error(t, err)
}

func TestUT_FilterContains_WrongArgCount(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	tmpl := `{{ name|contains }}`
	ctx := NewContext("hello", nil)
	_, err := engine.ExecuteToString(tmpl, ctx)
	assert.Error(t, err)
}

func TestUT_FilterHasPrefix_WrongArgCount(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	tmpl := `{{ name|hasprefix }}`
	ctx := NewContext("hello", nil)
	_, err := engine.ExecuteToString(tmpl, ctx)
	assert.Error(t, err)
}

func TestUT_FilterHasSuffix_WrongArgCount(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	tmpl := `{{ name|hassuffix }}`
	ctx := NewContext("hello", nil)
	_, err := engine.ExecuteToString(tmpl, ctx)
	assert.Error(t, err)
}

func TestUT_FilterReplace_WrongArgCount(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	tmpl := `{{ name|replace("a") }}`
	ctx := NewContext("hello", nil)
	_, err := engine.ExecuteToString(tmpl, ctx)
	assert.Error(t, err)
}

func TestUT_FilterTrim_TooManyArgs(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	tmpl := `{{ name|trim("a", "b") }}`
	ctx := NewContext("hello", nil)
	_, err := engine.ExecuteToString(tmpl, ctx)
	assert.Error(t, err)
}

func TestUT_FilterDefault_WrongArgCount(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	tmpl := `{{ name|default }}`
	ctx := NewContext("hello", nil)
	_, err := engine.ExecuteToString(tmpl, ctx)
	assert.Error(t, err)
}

func TestUT_FilterTruncate_NegativeLength(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	tmpl := `{{ name|truncate(-1) }}`
	ctx := NewContext("hello", nil)
	_, err := engine.ExecuteToString(tmpl, ctx)
	assert.Error(t, err)
}

func TestUT_FilterTruncate_WrongArgCount(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	tmpl := `{{ name|truncate }}`
	ctx := NewContext("hello", nil)
	_, err := engine.ExecuteToString(tmpl, ctx)
	assert.Error(t, err)
}

func TestUT_FilterJoin_NonList(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	// join on a non-list should return the value as-is
	tmpl := `{{ name|join(",") }}`
	ctx := NewContext("hello", nil)
	result, err := engine.ExecuteToString(tmpl, ctx)
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestUT_FilterDefault_NilInput(t *testing.T) {
	t.Parallel()
	engine := MustNewEngine()

	// When the variable is undefined, default should kick in
	tmpl := `{{ missing|default("fallback") }}`
	ctx := Context{"missing": nil}
	result, err := engine.ExecuteToString(tmpl, ctx)
	require.NoError(t, err)
	assert.Equal(t, "fallback", result)
}

func TestUT_RegisterFilters_Success(t *testing.T) {
	t.Parallel()
	// Verifying RegisterFilters doesn't error by creating an engine
	engine, err := NewEngine()
	require.NoError(t, err)
	assert.NotNil(t, engine)
}

func TestUT_FilterCaseAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tmpl     string
		input    string
		expected string
	}{
		{"pascal_case", `{{ name|pascal_case }}`, "hello_world", "HelloWorld"},
		{"camel_case", `{{ name|camel_case }}`, "hello_world", "helloWorld"},
		{"kebab_case", `{{ name|kebab_case }}`, "hello_world", "hello-world"},
		{"singularize", `{{ name|singularize }}`, "users", "user"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			eng := MustNewEngine()
			ctx := NewContext(tt.input, nil)
			result, err := eng.ExecuteToString(tt.tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

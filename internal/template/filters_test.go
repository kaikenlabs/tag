package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_FilterSnake(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"PascalCase", "PascalCase", "pascal_case"},
		{"camelCase", "camelCase", "camel_case"},
		{"kebab-case", "kebab-case", "kebab_case"},
		{"Title Case", "Title Case", "title_case"},
		{"empty", "", ""},
		{"single word", "hello", "hello"},
		{"already snake", "snake_case", "snake_case"},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := `{{ name|snake }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterPascal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"snake_case", "snake_case", "SnakeCase"},
		{"kebab-case", "kebab-case", "KebabCase"},
		{"camelCase", "camelCase", "CamelCase"},
		{"Title Case", "Title Case", "TitleCase"},
		{"empty", "", ""},
		{"single word", "hello", "Hello"},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := `{{ name|pascal }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterCamel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"snake_case", "snake_case", "snakeCase"},
		{"PascalCase", "PascalCase", "pascalCase"},
		{"kebab-case", "kebab-case", "kebabCase"},
		{"empty", "", ""},
		{"single word", "hello", "hello"},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := `{{ name|camel }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterKebab(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"PascalCase", "PascalCase", "pascal-case"},
		{"snake_case", "snake_case", "snake-case"},
		{"camelCase", "camelCase", "camel-case"},
		{"empty", "", ""},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := `{{ name|kebab }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterLower(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"HELLO", "hello"},
		{"Hello World", "hello world"},
		{"", ""},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tmpl := `{{ name|lower }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterUpper(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "HELLO"},
		{"Hello World", "HELLO WORLD"},
		{"", ""},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tmpl := `{{ name|upper }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello world", "Hello World"},
		{"HELLO", "Hello"}, // cases.Title correctly lowercases non-initial letters (matches Python/Jinja2)
		{"", ""},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tmpl := `{{ name|title }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterPlural(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user", "users"},
		{"person", "people"},
		{"child", "children"},
		{"ox", "oxen"},
		{"", ""},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tmpl := `{{ name|plural }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterSingular(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "user"},
		{"people", "person"},
		{"children", "child"},
		{"", ""},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tmpl := `{{ name|singular }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterOrdinalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1", "1st"},
		{"2", "2nd"},
		{"3", "3rd"},
		{"4", "4th"},
		{"11", "11th"},
		{"12", "12th"},
		{"13", "13th"},
		{"21", "21st"},
		{"22", "22nd"},
		{"23", "23rd"},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tmpl := `{{ name|ordinalize }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterTitleize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello_world", "Hello World"},
		{"user_id", "User ID"},
		{"", ""},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tmpl := `{{ name|titleize }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterHumanize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user_id", "User ID"},
		{"employee_salary", "Employee salary"},
		{"", ""},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tmpl := `{{ name|humanize }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterPast(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"PascalCase cancel", "OrderCancel", "OrderCancelled"},
		{"PascalCase create", "OrderCreate", "OrderCreated"},
		{"PascalCase irregular", "OrderSend", "OrderSent"},
		{"camelCase", "orderCancel", "orderCancelled"},
		{"single word", "cancel", "cancelled"},
		{"already past", "created", "created"},
		{"empty", "", ""},
	}

	engine := MustNewEngine()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := `{{ name|past }}`
			ctx := NewContext(tt.input, nil)
			result, err := engine.ExecuteToString(tmpl, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_FilterPastTenseAlias(t *testing.T) {
	engine := MustNewEngine()
	tmpl := `{{ name|past_tense }}`
	ctx := NewContext("OrderCancel", nil)
	result, err := engine.ExecuteToString(tmpl, ctx)
	require.NoError(t, err)
	assert.Equal(t, "OrderCancelled", result)
}

func TestUT_FilterPastChaining(t *testing.T) {
	engine := MustNewEngine()

	t.Run("past then snake", func(t *testing.T) {
		tmpl := `{{ name|past|snake }}`
		ctx := NewContext("OrderCancel", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "order_cancelled", result)
	})

	t.Run("past then kebab", func(t *testing.T) {
		tmpl := `{{ name|past|kebab }}`
		ctx := NewContext("OrderCreate", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "order-created", result)
	})
}

func TestUT_FilterSplit(t *testing.T) {
	engine := MustNewEngine()

	t.Run("split by comma", func(t *testing.T) {
		tmpl := `{% for item in name|split(",") %}{{ item }};{% endfor %}`
		ctx := NewContext("a,b,c", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "a;b;c;", result)
	})

	t.Run("split by whitespace default", func(t *testing.T) {
		tmpl := `{% for item in name|split %}{{ item }};{% endfor %}`
		ctx := NewContext("a b c", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "a;b;c;", result)
	})
}

func TestUT_FilterJoin(t *testing.T) {
	engine := MustNewEngine()

	t.Run("join with comma", func(t *testing.T) {
		tmpl := `{{ items|join(",") }}`
		ctx := Context{"items": []string{"a", "b", "c"}}
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "a,b,c", result)
	})

	t.Run("join with empty separator", func(t *testing.T) {
		tmpl := `{{ items|join }}`
		ctx := Context{"items": []string{"a", "b", "c"}}
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "abc", result)
	})
}

func TestUT_FilterContains(t *testing.T) {
	engine := MustNewEngine()

	t.Run("contains substring", func(t *testing.T) {
		tmpl := `{% if name|contains("world") %}yes{% else %}no{% endif %}`
		ctx := NewContext("hello world", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "yes", result)
	})

	t.Run("does not contain", func(t *testing.T) {
		tmpl := `{% if name|contains("xyz") %}yes{% else %}no{% endif %}`
		ctx := NewContext("hello world", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "no", result)
	})
}

func TestUT_FilterHasPrefix(t *testing.T) {
	engine := MustNewEngine()

	t.Run("has prefix", func(t *testing.T) {
		tmpl := `{% if name|hasprefix("hello") %}yes{% else %}no{% endif %}`
		ctx := NewContext("hello world", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "yes", result)
	})

	t.Run("does not have prefix", func(t *testing.T) {
		tmpl := `{% if name|hasprefix("world") %}yes{% else %}no{% endif %}`
		ctx := NewContext("hello world", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "no", result)
	})
}

func TestUT_FilterHasSuffix(t *testing.T) {
	engine := MustNewEngine()

	t.Run("has suffix", func(t *testing.T) {
		tmpl := `{% if name|hassuffix("world") %}yes{% else %}no{% endif %}`
		ctx := NewContext("hello world", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "yes", result)
	})

	t.Run("does not have suffix", func(t *testing.T) {
		tmpl := `{% if name|hassuffix("hello") %}yes{% else %}no{% endif %}`
		ctx := NewContext("hello world", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "no", result)
	})
}

func TestUT_FilterReplace(t *testing.T) {
	engine := MustNewEngine()

	t.Run("replace substring", func(t *testing.T) {
		tmpl := `{{ name|replace("world", "universe") }}`
		ctx := NewContext("hello world", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "hello universe", result)
	})

	t.Run("replace multiple occurrences", func(t *testing.T) {
		tmpl := `{{ name|replace("o", "0") }}`
		ctx := NewContext("hello world", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "hell0 w0rld", result)
	})
}

func TestUT_FilterTrim(t *testing.T) {
	engine := MustNewEngine()

	t.Run("trim whitespace", func(t *testing.T) {
		tmpl := `{{ name|trim }}`
		ctx := NewContext("  hello world  ", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "hello world", result)
	})

	t.Run("trim specific chars", func(t *testing.T) {
		tmpl := `{{ name|trim("x") }}`
		ctx := NewContext("xxxhelloxxx", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "hello", result)
	})
}

func TestUT_FilterDefault(t *testing.T) {
	engine := MustNewEngine()

	t.Run("use default for empty", func(t *testing.T) {
		tmpl := `{{ name|default("fallback") }}`
		ctx := NewContext("", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "fallback", result)
	})

	t.Run("use value when present", func(t *testing.T) {
		tmpl := `{{ name|default("fallback") }}`
		ctx := NewContext("hello", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "hello", result)
	})
}

func TestUT_FilterTruncate(t *testing.T) {
	engine := MustNewEngine()

	t.Run("truncate with ellipsis", func(t *testing.T) {
		tmpl := `{{ name|truncate(5) }}`
		ctx := NewContext("hello world", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "hello...", result)
	})

	t.Run("truncate with custom ending", func(t *testing.T) {
		tmpl := `{{ name|truncate(5, "!") }}`
		ctx := NewContext("hello world", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "hello!", result)
	})

	t.Run("no truncate when short enough", func(t *testing.T) {
		tmpl := `{{ name|truncate(20) }}`
		ctx := NewContext("hello", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("truncate UTF-8 multi-byte characters correctly", func(t *testing.T) {
		tmpl := `{{ name|truncate(3) }}`
		// Use emoji which are multi-byte UTF-8 characters
		ctx := NewContext("🎉🎊🎈🎁", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		// Should truncate by rune count, not byte count
		assert.Equal(t, "🎉🎊🎈...", result)
	})

	t.Run("truncate CJK characters correctly", func(t *testing.T) {
		tmpl := `{{ name|truncate(2) }}`
		ctx := NewContext("你好世界", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "你好...", result)
	})
}

func TestUT_FilterChaining(t *testing.T) {
	engine := MustNewEngine()

	t.Run("snake then upper", func(t *testing.T) {
		tmpl := `{{ name|snake|upper }}`
		ctx := NewContext("HelloWorld", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "HELLO_WORLD", result)
	})

	t.Run("plural then pascal", func(t *testing.T) {
		tmpl := `{{ name|plural|pascal }}`
		ctx := NewContext("user", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "Users", result)
	})
}

func TestUT_FilterAliases(t *testing.T) {
	engine := MustNewEngine()

	t.Run("snake_case alias", func(t *testing.T) {
		tmpl := `{{ name|snake_case }}`
		ctx := NewContext("HelloWorld", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "hello_world", result)
	})

	t.Run("pluralize alias", func(t *testing.T) {
		tmpl := `{{ name|pluralize }}`
		ctx := NewContext("user", nil)
		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "users", result)
	})
}

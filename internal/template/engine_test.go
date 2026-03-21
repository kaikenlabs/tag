package template

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Engine_NewEngine(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		engine, err := NewEngine()
		require.NoError(t, err)
		assert.NotNil(t, engine)
	})

	t.Run("with base dir", func(t *testing.T) {
		engine, err := NewEngine(WithBaseDir("/tmp/templates"))
		require.NoError(t, err)
		assert.Equal(t, "/tmp/templates", engine.baseDir)
	})
}

func TestUT_Engine_ParseString_Simple(t *testing.T) {
	engine := MustNewEngine()

	tmpl, err := engine.ParseString("Hello, {{ name }}!")
	require.NoError(t, err)
	require.NotNil(t, tmpl)

	ctx := NewContext("World", nil)
	result, err := tmpl.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", result)
}

func TestUT_Engine_ParseString_WithFilters(t *testing.T) {
	engine := MustNewEngine()

	tmpl, err := engine.ParseString("{{ name|upper }}")
	require.NoError(t, err)

	ctx := NewContext("hello", nil)
	result, err := tmpl.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, "HELLO", result)
}

func TestUT_Engine_ParseString_WithConditionals(t *testing.T) {
	engine := MustNewEngine()

	tmpl, err := engine.ParseString("{% if show %}visible{% else %}hidden{% endif %}")
	require.NoError(t, err)

	t.Run("condition true", func(t *testing.T) {
		ctx := Context{"show": true}
		result, err := tmpl.Execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, "visible", result)
	})

	t.Run("condition false", func(t *testing.T) {
		ctx := Context{"show": false}
		result, err := tmpl.Execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, "hidden", result)
	})
}

func TestUT_Engine_ParseString_WithLoops(t *testing.T) {
	engine := MustNewEngine()

	tmpl, err := engine.ParseString("{% for item in items %}{{ item }},{% endfor %}")
	require.NoError(t, err)

	ctx := Context{"items": []string{"a", "b", "c"}}
	result, err := tmpl.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, "a,b,c,", result)
}

func TestUT_Engine_ParseString_NestedAccess(t *testing.T) {
	engine := MustNewEngine()

	tmpl, err := engine.ParseString("{{ vars.project_name }}")
	require.NoError(t, err)

	vars := map[string]any{"project_name": "my-project"}
	ctx := NewContext("test", vars)

	result, err := tmpl.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, "my-project", result)
}

func TestUT_Engine_ParseString_FilterChain(t *testing.T) {
	engine := MustNewEngine()

	tmpl, err := engine.ParseString("{{ name|snake|upper }}")
	require.NoError(t, err)

	ctx := NewContext("HelloWorld", nil)
	result, err := tmpl.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, "HELLO_WORLD", result)
}

func TestUT_Engine_ParseString_SyntaxError(t *testing.T) {
	engine := MustNewEngine()

	_, err := engine.ParseString("{{ unclosed")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestUT_Engine_ExecuteToString(t *testing.T) {
	engine := MustNewEngine()

	ctx := NewContext("World", nil)
	result, err := engine.ExecuteToString("Hello, {{ name }}!", ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", result)
}

func TestUT_Engine_MustParseString(t *testing.T) {
	engine := MustNewEngine()

	t.Run("valid template", func(t *testing.T) {
		tmpl := engine.MustParseString("Hello, {{ name }}!")
		assert.NotNil(t, tmpl)
	})

	t.Run("invalid template panics", func(t *testing.T) {
		assert.Panics(t, func() {
			engine.MustParseString("{{ unclosed")
		})
	})
}

func TestUT_Engine_ParseFile(t *testing.T) {
	// Create a temporary directory and file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.tmpl")
	err := os.WriteFile(tmpFile, []byte("Hello, {{ name }}!"), 0o644)
	require.NoError(t, err)

	engine := MustNewEngine(WithBaseDir(tmpDir))

	tmpl, err := engine.ParseFile("test.tmpl")
	require.NoError(t, err)

	ctx := NewContext("World", nil)
	result, err := tmpl.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", result)
}

func TestUT_Engine_ParseFile_NotFound(t *testing.T) {
	engine := MustNewEngine(WithBaseDir("/nonexistent"))

	_, err := engine.ParseFile("missing.tmpl")
	assert.Error(t, err)
}

func TestUT_Engine_Environment(t *testing.T) {
	engine := MustNewEngine()

	env := engine.Environment()
	assert.NotNil(t, env)
	assert.NotNil(t, env.Filters)
}

func TestUT_Engine_ComplexTemplate(t *testing.T) {
	engine := MustNewEngine()

	template := `package {{ vars.package_name }}

type {{ n.pascal_case }} struct {
	ID   int
	Name string
}

{% for field in vars.fields %}
func ({{ n.camel_case }} *{{ n.pascal_case }}) Get{{ field|pascal }}() string {
	return {{ n.camel_case }}.{{ field|pascal }}
}
{% endfor %}`

	vars := map[string]any{
		"package_name": "models",
		"fields":       []string{"name", "email", "phone"},
	}
	ctx := NewContext("User", vars)

	result, err := engine.ExecuteToString(template, ctx)
	require.NoError(t, err)

	assert.Contains(t, result, "package models")
	assert.Contains(t, result, "type User struct")
	assert.Contains(t, result, "func (user *User) GetName()")
	assert.Contains(t, result, "func (user *User) GetEmail()")
	assert.Contains(t, result, "func (user *User) GetPhone()")
}

func TestUT_Engine_BuiltinFilters(t *testing.T) {
	engine := MustNewEngine()

	// Test that builtin Gonja filters still work
	tests := []struct {
		template string
		ctx      Context
		expected string
	}{
		{
			template: "{{ name|length }}",
			ctx:      Context{"name": "hello"},
			expected: "5",
		},
		{
			template: "{{ items|first }}",
			ctx:      Context{"items": []string{"a", "b", "c"}},
			expected: "a",
		},
		{
			template: "{{ items|last }}",
			ctx:      Context{"items": []string{"a", "b", "c"}},
			expected: "c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			result, err := engine.ExecuteToString(tt.template, tt.ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_Engine_NowFunction_NoArgs(t *testing.T) {
	engine := MustNewEngine()
	ctx := NewContext("test", nil)

	result, err := engine.ExecuteToString("{{ now() }}", ctx)
	require.NoError(t, err)

	// Should be valid RFC3339
	_, parseErr := time.Parse(time.RFC3339, result)
	assert.NoError(t, parseErr, "now() without args should return valid RFC3339, got: %s", result)
}

func TestUT_Engine_NowFunction_DateFormat(t *testing.T) {
	engine := MustNewEngine()
	ctx := NewContext("test", nil)

	result, err := engine.ExecuteToString(`{{ now("20060102") }}`, ctx)
	require.NoError(t, err)

	expected := time.Now().Format("20060102")
	assert.Equal(t, expected, result)
}

func TestUT_Engine_NowFunction_TimeFormat(t *testing.T) {
	engine := MustNewEngine()
	ctx := NewContext("test", nil)

	result, err := engine.ExecuteToString(`{{ now("15:04:05") }}`, ctx)
	require.NoError(t, err)

	// Parse to verify it's a valid time — allow 1s tolerance for test execution
	_, parseErr := time.Parse("15:04:05", result)
	assert.NoError(t, parseErr, "now(\"15:04:05\") should return valid time, got: %s", result)
}

func TestUT_Engine_NowFunction_MigrationFormat(t *testing.T) {
	engine := MustNewEngine()
	ctx := NewContext("test", nil)

	result, err := engine.ExecuteToString(`{{ now("20060102150405") }}`, ctx)
	require.NoError(t, err)

	assert.Len(t, result, 14, "migration format should be 14 digits")
	// Verify it starts with today's date
	assert.Equal(t, time.Now().Format("20060102"), result[:8])
}

func TestUT_Engine_NowFunction_InFilePath(t *testing.T) {
	engine := MustNewEngine()
	ctx := NewContext("users", nil)

	tmpl := `migrations/{{ now("20060102150405") }}_create_{{ name }}.up.sql`
	result, err := engine.ExecuteToString(tmpl, ctx)
	require.NoError(t, err)

	assert.Contains(t, result, "migrations/")
	assert.Contains(t, result, "_create_users.up.sql")
	assert.Len(t, result, len("migrations/")+14+len("_create_users.up.sql"))
}

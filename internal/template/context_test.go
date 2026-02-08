package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_NewContext_CreatesNamespace(t *testing.T) {
	vars := map[string]any{
		"project_name": "my-project",
		"author":       "John Doe",
	}

	ctx := NewContext("my-project", vars, nil)

	// Check name is set
	assert.Equal(t, "my-project", ctx["name"])

	// Check vars namespace
	varsMap, ok := ctx["vars"].(map[string]any)
	require.True(t, ok, "vars should be a map")
	assert.Equal(t, "my-project", varsMap["project_name"])
	assert.Equal(t, "John Doe", varsMap["author"])
}

func TestUT_NewContext_CookiecutterAlias(t *testing.T) {
	vars := map[string]any{
		"project_name": "my-project",
	}

	ctx := NewContext("my-project", vars, nil)

	// Check that cookiecutter points to the same data as vars
	varsMap := ctx["vars"].(map[string]any)
	cookiecutterMap := ctx["cookiecutter"].(map[string]any)

	assert.Equal(t, varsMap["project_name"], cookiecutterMap["project_name"])

	// Verify they're the same map (aliased)
	varsMap["new_key"] = "new_value"
	assert.Equal(t, "new_value", cookiecutterMap["new_key"])
}

func TestUT_NewContext_NameOptions(t *testing.T) {
	nameOpts := &NameOptions{
		SnakeCase:  "my_project",
		PascalCase: "MyProject",
		CamelCase:  "myProject",
		KebabCase:  "my-project",
		LowerCase:  "my-project",
		UpperCase:  "MY-PROJECT",
	}

	ctx := NewContext("my-project", nil, nameOpts)

	nMap, ok := ctx["n"].(map[string]any)
	require.True(t, ok, "n should be a map")

	assert.Equal(t, "my_project", nMap["snake_case"])
	assert.Equal(t, "MyProject", nMap["pascal_case"])
	assert.Equal(t, "myProject", nMap["camel_case"])
	assert.Equal(t, "my-project", nMap["kebab_case"])
	assert.Equal(t, "my-project", nMap["lower_case"])
	assert.Equal(t, "MY-PROJECT", nMap["upper_case"])
}

func TestUT_NewContext_ComputesNameOptionsWhenNil(t *testing.T) {
	ctx := NewContext("MyProject", nil, nil)

	nMap, ok := ctx["n"].(map[string]any)
	require.True(t, ok, "n should be a map")

	// Verify computed values
	assert.Equal(t, "my_project", nMap["snake_case"])
	assert.Equal(t, "MyProject", nMap["pascal_case"])
	assert.Equal(t, "myProject", nMap["camel_case"])
	assert.Equal(t, "my-project", nMap["kebab_case"])
	assert.Equal(t, "myproject", nMap["lower_case"])
	assert.Equal(t, "MYPROJECT", nMap["upper_case"])
}

func TestUT_NewContext_NilVarsCreatesEmptyMap(t *testing.T) {
	ctx := NewContext("test", nil, nil)

	varsMap, ok := ctx["vars"].(map[string]any)
	require.True(t, ok, "vars should be a map")
	assert.NotNil(t, varsMap)
	assert.Len(t, varsMap, 0)
}

func TestUT_NewNameOptions(t *testing.T) {
	opts := NewNameOptions("MyProject")

	assert.Equal(t, "my_project", opts.SnakeCase)
	assert.Equal(t, "MyProject", opts.PascalCase)
	assert.Equal(t, "myProject", opts.CamelCase)
	assert.Equal(t, "my-project", opts.KebabCase)
	assert.Equal(t, "myproject", opts.LowerCase)
	assert.Equal(t, "MYPROJECT", opts.UpperCase)
}

func TestUT_Context_Set(t *testing.T) {
	ctx := make(Context)
	ctx.Set("key", "value")

	assert.Equal(t, "value", ctx["key"])
}

func TestUT_Context_Get(t *testing.T) {
	ctx := Context{"key": "value"}

	val, ok := ctx.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "value", val)

	_, ok = ctx.Get("nonexistent")
	assert.False(t, ok)
}

func TestUT_Context_Merge(t *testing.T) {
	ctx1 := Context{"a": "1", "b": "2"}
	ctx2 := Context{"b": "3", "c": "4"}

	ctx1.Merge(ctx2)

	assert.Equal(t, "1", ctx1["a"])
	assert.Equal(t, "3", ctx1["b"]) // overridden
	assert.Equal(t, "4", ctx1["c"])
}

func TestUT_Context_InTemplate(t *testing.T) {
	engine := MustNewEngine()

	t.Run("access vars namespace", func(t *testing.T) {
		tmpl := `{{ vars.project_name }}`
		vars := map[string]any{"project_name": "my-project"}
		ctx := NewContext("test", vars, nil)

		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "my-project", result)
	})

	t.Run("access cookiecutter namespace", func(t *testing.T) {
		tmpl := `{{ cookiecutter.project_name }}`
		vars := map[string]any{"project_name": "my-project"}
		ctx := NewContext("test", vars, nil)

		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "my-project", result)
	})

	t.Run("access n namespace", func(t *testing.T) {
		tmpl := `{{ n.pascal_case }}`
		ctx := NewContext("my-project", nil, nil)

		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "MyProject", result)
	})

	t.Run("access name directly", func(t *testing.T) {
		tmpl := `{{ name }}`
		ctx := NewContext("my-project", nil, nil)

		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "my-project", result)
	})
}

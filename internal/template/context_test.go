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

	ctx := NewContext("my-project", vars)

	// Check name is set
	assert.Equal(t, "my-project", ctx["name"])

	// Check vars namespace
	varsMap, ok := ctx["vars"].(map[string]any)
	require.True(t, ok, "vars should be a map")
	assert.Equal(t, "my-project", varsMap["project_name"])
	assert.Equal(t, "John Doe", varsMap["author"])
}

func TestUT_NewContext_ComputesNameOptions(t *testing.T) {
	ctx := NewContext("MyProject", nil)

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
	ctx := NewContext("test", nil)

	varsMap, ok := ctx["vars"].(map[string]any)
	require.True(t, ok, "vars should be a map")
	assert.NotNil(t, varsMap)
	assert.Len(t, varsMap, 0)
}

func TestUT_Context_InTemplate(t *testing.T) {
	engine := MustNewEngine()

	t.Run("access vars namespace", func(t *testing.T) {
		tmpl := `{{ vars.project_name }}`
		vars := map[string]any{"project_name": "my-project"}
		ctx := NewContext("test", vars)

		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "my-project", result)
	})

	t.Run("access n namespace", func(t *testing.T) {
		tmpl := `{{ n.pascal_case }}`
		ctx := NewContext("my-project", nil)

		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "MyProject", result)
	})

	t.Run("access name directly", func(t *testing.T) {
		tmpl := `{{ name }}`
		ctx := NewContext("my-project", nil)

		result, err := engine.ExecuteToString(tmpl, ctx)
		require.NoError(t, err)
		assert.Equal(t, "my-project", result)
	})
}

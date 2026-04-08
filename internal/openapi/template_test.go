package openapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/template"
)

// TestUT_OpenAPISpecTemplate_Render validates that the openapi-spec template
// renders correctly with sample variables.
func TestUT_OpenAPISpecTemplate_Render(t *testing.T) {
	tmplContent, err := os.ReadFile("testdata/openapi-spec.tmpl")
	require.NoError(t, err)

	// Extract metadata and body
	metaRaw, bodyRaw, err := template.ExtractMetadata(string(tmplContent))
	require.NoError(t, err)

	engine, err := template.NewEngine()
	require.NoError(t, err)

	ctx := template.NewContextBuilder().WithName("widget").WithVars(map[string]any{
		"domain": "admin",
		"fields": "name:string;priority:int;active:bool",
	}).Build()

	// Render metadata
	renderedMeta, err := engine.ExecuteToString(metaRaw, ctx)
	require.NoError(t, err)

	meta, err := template.ParseMetadata(renderedMeta)
	require.NoError(t, err)

	assert.Equal(t, template.ActionOpenAPI, meta.Action)
	assert.Equal(t, "spec/admin/openapi.yaml", meta.To)

	// Render body
	renderedBody, err := engine.ExecuteToString(bodyRaw, ctx)
	require.NoError(t, err)

	// Verify rendered output contains expected paths
	assert.Contains(t, renderedBody, "/admin/widgets")
	assert.Contains(t, renderedBody, "/admin/widgets/{id}")

	// Verify operation IDs
	assert.Contains(t, renderedBody, "operationId: listWidgets")
	assert.Contains(t, renderedBody, "operationId: createWidget")
	assert.Contains(t, renderedBody, "operationId: getWidget")
	assert.Contains(t, renderedBody, "operationId: updateWidget")
	assert.Contains(t, renderedBody, "operationId: deleteWidget")

	// Verify $ref references
	assert.Contains(t, renderedBody, "$ref: '#/components/schemas/Widget'")
	assert.Contains(t, renderedBody, "$ref: '#/components/schemas/CreateWidgetRequest'")
	assert.Contains(t, renderedBody, "$ref: '#/components/schemas/UpdateWidgetRequest'")
	assert.Contains(t, renderedBody, "$ref: '#/components/schemas/WidgetList'")

	// Verify schemas exist
	assert.Contains(t, renderedBody, "Widget:")
	assert.Contains(t, renderedBody, "CreateWidgetRequest:")
	assert.Contains(t, renderedBody, "UpdateWidgetRequest:")
	assert.Contains(t, renderedBody, "WidgetList:")

	// Verify to("openapi") type mapping
	assert.Contains(t, renderedBody, "type: string")  // name field
	assert.Contains(t, renderedBody, "type: integer") // priority field
	assert.Contains(t, renderedBody, "type: boolean") // active field
}

// TestUT_OpenAPISpecTemplate_MergeIntoSpec validates that the rendered template
// output merges correctly into an existing OpenAPI spec.
func TestUT_OpenAPISpecTemplate_MergeIntoSpec(t *testing.T) {
	tmplContent, err := os.ReadFile("testdata/openapi-spec.tmpl")
	require.NoError(t, err)

	_, bodyRaw, err := template.ExtractMetadata(string(tmplContent))
	require.NoError(t, err)

	engine, err := template.NewEngine()
	require.NoError(t, err)

	ctx := template.NewContextBuilder().WithName("widget").WithVars(map[string]any{
		"domain": "admin",
		"fields": "name:string;priority:int",
	}).Build()

	renderedBody, err := engine.ExecuteToString(bodyRaw, ctx)
	require.NoError(t, err)

	// Merge into a base spec
	baseSpec := `openapi: "3.0.3"
info:
  title: Admin API
  version: "1.0.0"
paths: {}
components:
  schemas: {}
`
	editor := NewEditor()
	out, result, err := editor.Merge([]byte(baseSpec), []byte(renderedBody), MergeOptions{})
	require.NoError(t, err)

	assert.True(t, result.Changed)
	assert.Len(t, result.AddedPaths, 2)           // /admin/widgets and /admin/widgets/{id}
	assert.True(t, len(result.AddedSchemas) >= 4) // Widget, CreateWidgetRequest, UpdateWidgetRequest, WidgetList

	// Verify output is valid YAML that can be re-parsed
	spec, err := ParseSpec(out)
	require.NoError(t, err)
	assert.NotNil(t, spec.Paths())
	assert.NotNil(t, spec.Schemas())

	// Verify idempotency: merge same fragment again
	_, result2, err := editor.Merge(out, []byte(renderedBody), MergeOptions{})
	require.NoError(t, err)
	assert.False(t, result2.Changed, "second merge should be idempotent")
}

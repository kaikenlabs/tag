package openapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ParseSpec_ValidOpenAPI30(t *testing.T) {
	data := []byte(`openapi: "3.0.3"
info:
  title: Test API
  version: "1.0.0"
paths:
  /users:
    get:
      summary: List users
components:
  schemas:
    User:
      type: object
`)
	spec, err := ParseSpec(data)
	require.NoError(t, err)
	assert.NotNil(t, spec.Root)
	assert.True(t, spec.HasPaths())
	assert.True(t, spec.HasSchemas())
	assert.Equal(t, 2, spec.Indent)
}

func TestUT_ParseSpec_ValidOpenAPI31(t *testing.T) {
	data := []byte(`openapi: "3.1.0"
info:
  title: Test API
  version: "1.0.0"
paths: {}
`)
	spec, err := ParseSpec(data)
	require.NoError(t, err)
	assert.NotNil(t, spec.Root)
	assert.True(t, spec.HasPaths())
}

func TestUT_ParseSpec_LocatePaths(t *testing.T) {
	data := []byte(`openapi: "3.0.3"
paths:
  /widgets:
    get:
      summary: List widgets
`)
	spec, err := ParseSpec(data)
	require.NoError(t, err)

	paths := spec.Paths()
	require.NotNil(t, paths)
	assert.Len(t, paths.Values, 1)
	assert.Equal(t, "/widgets", paths.Values[0].Key.String())
}

func TestUT_ParseSpec_LocateSchemas(t *testing.T) {
	data := []byte(`openapi: "3.0.3"
components:
  schemas:
    Widget:
      type: object
    User:
      type: object
`)
	spec, err := ParseSpec(data)
	require.NoError(t, err)

	schemas := spec.Schemas()
	require.NotNil(t, schemas)
	assert.Len(t, schemas.Values, 2)
}

func TestUT_ParseSpec_MissingPaths(t *testing.T) {
	data := []byte(`openapi: "3.0.3"
info:
  title: Test API
`)
	spec, err := ParseSpec(data)
	require.NoError(t, err)
	assert.False(t, spec.HasPaths())
	assert.Nil(t, spec.Paths())
}

func TestUT_ParseSpec_MissingComponents(t *testing.T) {
	data := []byte(`openapi: "3.0.3"
paths: {}
`)
	spec, err := ParseSpec(data)
	require.NoError(t, err)
	assert.False(t, spec.HasSchemas())
	assert.False(t, spec.HasComponents())
	assert.Nil(t, spec.Schemas())
}

func TestUT_ParseSpec_EmptyPaths(t *testing.T) {
	data := []byte(`openapi: "3.0.3"
paths: {}
`)
	spec, err := ParseSpec(data)
	require.NoError(t, err)
	assert.True(t, spec.HasPaths())
	// Empty flow mapping returns nil for Paths() since it's not a MappingNode
	// This is expected — empty {} doesn't have Values
}

func TestUT_ParseSpec_EmptyComponentsSchemas(t *testing.T) {
	data := []byte(`openapi: "3.0.3"
components:
  schemas: {}
`)
	spec, err := ParseSpec(data)
	require.NoError(t, err)
	assert.True(t, spec.HasComponents())
}

func TestUT_ParseSpec_MultiDocument_Error(t *testing.T) {
	data := []byte(`---
openapi: "3.0.3"
---
openapi: "3.1.0"
`)
	_, err := ParseSpec(data)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMultiDocument)
}

func TestUT_ParseSpec_InvalidYAML(t *testing.T) {
	data := []byte(`invalid: yaml: [`)
	_, err := ParseSpec(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse YAML")
}

func TestUT_ParseSpec_EmptyDocument(t *testing.T) {
	data := []byte(``)
	_, err := ParseSpec(data)
	require.Error(t, err)
	// Empty YAML parses to 1 doc with nil body → ErrNotMapping
	assert.ErrorIs(t, err, ErrNotMapping)
}

func TestUT_ParseSpec_IndentDetection_TwoSpace(t *testing.T) {
	data := []byte(`openapi: "3.0.3"
info:
  title: Test API
  version: "1.0.0"
paths:
  /users:
    get:
      summary: List
`)
	spec, err := ParseSpec(data)
	require.NoError(t, err)
	assert.Equal(t, 2, spec.Indent)
}

func TestUT_ParseSpec_IndentDetection_FourSpace(t *testing.T) {
	data := []byte(`openapi: "3.0.3"
info:
    title: Test API
    version: "1.0.0"
paths:
    /users:
        get:
            summary: List
`)
	spec, err := ParseSpec(data)
	require.NoError(t, err)
	assert.Equal(t, 4, spec.Indent)
}

func TestUT_ParseSpec_StringRoundTrip(t *testing.T) {
	data := []byte(`openapi: "3.0.3"
# This comment should survive
info:
  title: Test API
paths: {}
`)
	spec, err := ParseSpec(data)
	require.NoError(t, err)

	output := spec.String()
	assert.Contains(t, output, "# This comment should survive")
	assert.Contains(t, output, "openapi")
}

func TestUT_ParseSpec_NotMapping(t *testing.T) {
	// YAML that's a sequence at the root, not a mapping
	data := []byte(`- item1
- item2
`)
	_, err := ParseSpec(data)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotMapping)
}

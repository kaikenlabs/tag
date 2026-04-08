package openapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUT_RoundTripFidelity validates that parsing and re-serializing a spec
// preserves all comments (head, inline, foot).
func TestUT_RoundTripFidelity(t *testing.T) {
	spec := `# Top-level comment
openapi: "3.0.3"
info:
  title: Test API # inline comment
  version: "1.0.0"
# Section comment
paths: {}
components:
  schemas: {}
`
	editor := NewEditor()
	fragment := `paths: {}
components:
  schemas: {}
`
	out, result, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{})
	require.NoError(t, err)
	assert.False(t, result.Changed)

	outStr := string(out)
	assert.Contains(t, outStr, "# Top-level comment")
	assert.Contains(t, outStr, "# inline comment")
	assert.Contains(t, outStr, "# Section comment")
}

// TestUT_MapInsertion validates inserting a new key-value into an existing mapping.
func TestUT_MapInsertion(t *testing.T) {
	spec := `openapi: "3.0.3"
paths:
  /existing:
    get:
      summary: Existing endpoint
`
	fragment := `paths:
  /new-endpoint:
    post:
      summary: New endpoint
`
	editor := NewEditor()
	out, result, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{})
	require.NoError(t, err)

	assert.True(t, result.Changed)
	assert.Equal(t, []string{"/new-endpoint"}, result.AddedPaths)

	outStr := string(out)
	assert.Contains(t, outStr, "/new-endpoint")
	assert.Contains(t, outStr, "/existing")
	assert.Contains(t, outStr, "New endpoint")
	assert.Contains(t, outStr, "Existing endpoint")
}

// TestUT_NestedInsertion validates insertion under components.schemas (2 levels deep).
func TestUT_NestedInsertion(t *testing.T) {
	spec := `openapi: "3.0.3"
components:
  schemas:
    ExistingSchema:
      type: object
`
	fragment := `components:
  schemas:
    NewSchema:
      type: object
      properties:
        name:
          type: string
`
	editor := NewEditor()
	out, result, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{})
	require.NoError(t, err)

	assert.True(t, result.Changed)
	assert.Equal(t, []string{"NewSchema"}, result.AddedSchemas)

	outStr := string(out)
	assert.Contains(t, outStr, "NewSchema")
	assert.Contains(t, outStr, "ExistingSchema")
}

// TestUT_EmptySectionExpansion validates inserting into paths: {} or components.schemas: {}.
func TestUT_EmptySectionExpansion(t *testing.T) {
	spec := `openapi: "3.0.3"
paths: {}
components:
  schemas: {}
`
	fragment := `paths:
  /widgets:
    get:
      summary: List widgets
components:
  schemas:
    Widget:
      type: object
`
	editor := NewEditor()
	out, result, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{})
	require.NoError(t, err)

	assert.True(t, result.Changed)
	assert.Equal(t, []string{"/widgets"}, result.AddedPaths)
	assert.Equal(t, []string{"Widget"}, result.AddedSchemas)

	outStr := string(out)
	assert.Contains(t, outStr, "/widgets")
	assert.Contains(t, outStr, "Widget")
}

// TestUT_AnchorPreservation validates that anchors and aliases in untouched regions
// are preserved after insertion elsewhere.
func TestUT_AnchorPreservation(t *testing.T) {
	spec := `openapi: "3.0.3"
x-common: &common
  type: object
paths:
  /existing:
    get:
      summary: Existing
components:
  schemas:
    Base:
      <<: *common
      properties:
        id:
          type: string
`
	fragment := `paths:
  /new:
    get:
      summary: New path
`
	editor := NewEditor()
	out, result, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{})
	require.NoError(t, err)

	assert.True(t, result.Changed)
	outStr := string(out)
	assert.Contains(t, outStr, "&common")
	assert.Contains(t, outStr, "*common")
}

// TestUT_IdempotentSkip validates that inserting identical content is skipped silently.
func TestUT_IdempotentSkip(t *testing.T) {
	spec := `openapi: "3.0.3"
paths:
  /widgets:
    get:
      summary: List widgets
components:
  schemas:
    Widget:
      type: object
`
	fragment := `paths:
  /widgets:
    get:
      summary: List widgets
components:
  schemas:
    Widget:
      type: object
`
	editor := NewEditor()
	_, result, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{})
	require.NoError(t, err)

	assert.False(t, result.Changed)
	assert.Equal(t, []string{"/widgets"}, result.SkippedPaths)
	assert.Equal(t, []string{"Widget"}, result.SkippedSchemas)
}

// TestUT_ConflictError validates that conflicting content produces an error.
func TestUT_ConflictError(t *testing.T) {
	spec := `openapi: "3.0.3"
paths:
  /widgets:
    get:
      summary: Old summary
`
	fragment := `paths:
  /widgets:
    get:
      summary: New summary
`
	editor := NewEditor()
	_, _, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{ConflictPolicy: ConflictError})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict on key")
}

// TestUT_ConflictSkip validates that conflicting content is skipped with skip policy.
func TestUT_ConflictSkip(t *testing.T) {
	spec := `openapi: "3.0.3"
paths:
  /widgets:
    get:
      summary: Old summary
`
	fragment := `paths:
  /widgets:
    get:
      summary: New summary
`
	editor := NewEditor()
	_, result, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{ConflictPolicy: ConflictSkip})
	require.NoError(t, err)
	assert.False(t, result.Changed)
	assert.Equal(t, []string{"/widgets"}, result.SkippedPaths)
}

// TestUT_MissingPathsSection validates adding paths when spec has no paths section.
func TestUT_MissingPathsSection(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Test API
`
	fragment := `paths:
  /widgets:
    get:
      summary: List widgets
`
	editor := NewEditor()
	out, result, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{})
	require.NoError(t, err)

	assert.True(t, result.Changed)
	assert.Equal(t, []string{"/widgets"}, result.AddedPaths)
	assert.Contains(t, string(out), "/widgets")
}

// TestUT_MissingComponentsSection validates adding schemas when spec has no components section.
func TestUT_MissingComponentsSection(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Test API
paths: {}
`
	fragment := `components:
  schemas:
    Widget:
      type: object
`
	editor := NewEditor()
	out, result, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{})
	require.NoError(t, err)

	assert.True(t, result.Changed)
	assert.Equal(t, []string{"Widget"}, result.AddedSchemas)
	assert.Contains(t, string(out), "Widget")
}

// TestUT_MultiplePathsAndSchemas validates merging multiple paths and schemas at once.
func TestUT_MultiplePathsAndSchemas(t *testing.T) {
	spec := `openapi: "3.0.3"
paths:
  /existing:
    get:
      summary: Existing
components:
  schemas:
    Existing:
      type: object
`
	fragment := `paths:
  /new-a:
    get:
      summary: New A
  /new-b:
    post:
      summary: New B
components:
  schemas:
    SchemaA:
      type: object
    SchemaB:
      type: string
`
	editor := NewEditor()
	_, result, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{})
	require.NoError(t, err)

	assert.True(t, result.Changed)
	assert.Equal(t, []string{"/new-a", "/new-b"}, result.AddedPaths)
	assert.Equal(t, []string{"SchemaA", "SchemaB"}, result.AddedSchemas)
}

// TestUT_InvalidSpec validates error handling for invalid YAML.
func TestUT_InvalidSpec(t *testing.T) {
	editor := NewEditor()

	_, _, err := editor.Merge([]byte("invalid: yaml: ["), []byte("paths: {}"), MergeOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse spec")
}

// TestUT_InvalidFragment validates error handling for invalid fragment YAML.
func TestUT_InvalidFragment(t *testing.T) {
	editor := NewEditor()

	_, _, err := editor.Merge([]byte("openapi: '3.0.3'"), []byte("invalid: yaml: ["), MergeOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse fragment")
}

// TestUT_MethodLevelMerge validates adding a new HTTP method to an existing path.
func TestUT_MethodLevelMerge(t *testing.T) {
	// NOTE: In v1, method-level merge is NOT supported. If the path exists,
	// we check the whole path item. Different content = conflict.
	// This test validates the conflict behavior.
	spec := `openapi: "3.0.3"
paths:
  /widgets:
    get:
      summary: List widgets
`
	fragment := `paths:
  /widgets:
    get:
      summary: List widgets
    post:
      summary: Create widget
`
	editor := NewEditor()
	_, _, err := editor.Merge([]byte(spec), []byte(fragment), MergeOptions{ConflictPolicy: ConflictError})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

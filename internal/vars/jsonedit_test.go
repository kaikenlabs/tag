package vars

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_RenameJSONDeclarations_VarsKey(t *testing.T) {
	t.Parallel()

	src := `{
  "name": "demo",
  "vars": {
    "old": { "type": "string" },
    "keep": 1
  }
}`
	want := `{
  "name": "demo",
  "vars": {
    "new": { "type": "string" },
    "keep": 1
  }
}`

	got, n, err := renameJSONDeclarations([]byte(src), "old", "new")
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, want, string(got), "key order and formatting must be preserved")
}

func TestUT_RenameJSONDeclarations_RequiresEntries(t *testing.T) {
	t.Parallel()

	src := `{"requires": ["old", "other"], "vars": {"old": 1}}`
	want := `{"requires": ["new", "other"], "vars": {"new": 1}}`

	got, n, err := renameJSONDeclarations([]byte(src), "old", "new")
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, want, string(got))
}

func TestUT_RenameJSONDeclarations_EscapedKeySurvives(t *testing.T) {
	t.Parallel()

	// A \uXXXX-escaped key decodes to "old" and must be replaced as a whole
	// token — naive offset arithmetic would splice into the middle of it.
	src := "{\"vars\": {\"\\u006fld\": {\"type\": \"string\"}}}"
	want := `{"vars": {"new": {"type": "string"}}}`

	got, n, err := renameJSONDeclarations([]byte(src), "old", "new")
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, want, string(got))
}

func TestUT_RenameJSONDeclarations_LeavesUnrelatedStringsAlone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
	}{
		{
			name: "value that happens to equal the name",
			src:  `{"vars": {"other": "old"}}`,
		},
		{
			name: "prompt text containing the name",
			src:  `{"vars": {"other": {"prompt": "old"}}}`,
		},
		{
			name: "key of the same name nested deeper than vars",
			src:  `{"vars": {"other": {"options": {"old": 1}}}}`,
		},
		{
			name: "key of the same name outside vars",
			src:  `{"hooks": {"old": 1}}`,
		},
		{
			name: "nested config that merely contains a vars object",
			src:  `{"generators": {"api": {"vars": {"old": 1}}}}`,
		},
		{
			name: "requires nested deeper than the top level",
			src:  `{"vars": {"other": {"requires": ["old"]}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, n, err := renameJSONDeclarations([]byte(tt.src), "old", "new")
			require.NoError(t, err)
			assert.Equal(t, 0, n)
			assert.Equal(t, tt.src, string(got))
		})
	}
}

func TestUT_RenameJSONDeclarations_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, _, err := renameJSONDeclarations([]byte(`{"vars": `), "old", "new")
	require.Error(t, err)
}

func TestUT_RenameJSONDeclarations_NewNameIsEscapedSafely(t *testing.T) {
	t.Parallel()

	// The public command rejects names that are not identifiers, but the
	// splicer itself must still emit a well-formed JSON string for any input —
	// so feed it a name that genuinely needs escaping and confirm the quote and
	// backslash survive as JSON escapes rather than corrupting the document.
	src := `{"vars": {"old": 1}}`
	got, n, err := renameJSONDeclarations([]byte(src), "old", `a"b\c`)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.JSONEq(t, `{"vars": {"a\"b\\c": 1}}`, string(got))
}

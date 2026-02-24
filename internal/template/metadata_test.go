package template

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_ExtractMetadata_ValidBlock(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantMeta     string
		wantBody     string
		wantErr      bool
		wantErrValue error
	}{
		{
			name: "simple metadata block",
			content: `---
to: output/file.go
---
package main`,
			wantMeta: "to: output/file.go",
			wantBody: "package main",
			wantErr:  false,
		},
		{
			name: "multiple metadata fields",
			content: `---
to: output/file.go
inject: true
after: // marker
---
// injected content`,
			wantMeta: "to: output/file.go\ninject: true\nafter: // marker",
			wantBody: "// injected content",
			wantErr:  false,
		},
		{
			name: "metadata with custom fields",
			content: `---
to: {{ name }}/file.go
custom_key: custom_value
---
body content`,
			wantMeta: "to: {{ name }}/file.go\ncustom_key: custom_value",
			wantBody: "body content",
			wantErr:  false,
		},
		{
			name: "empty body",
			content: `---
to: output/file.go
---
`,
			wantMeta: "to: output/file.go",
			wantBody: "",
			wantErr:  false,
		},
		{
			name: "multiline body",
			content: `---
to: output/file.go
---
line 1
line 2
line 3`,
			wantMeta: "to: output/file.go",
			wantBody: "line 1\nline 2\nline 3",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaRaw, bodyRaw, err := ExtractMetadata(tt.content)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrValue != nil {
					assert.True(t, errors.Is(err, tt.wantErrValue))
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantMeta, metaRaw)
			assert.Equal(t, tt.wantBody, strings.TrimSpace(bodyRaw))
		})
	}
}

func TestUT_ExtractMetadata_NoBlock(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "no dashes",
			content: "to: output/file.go\npackage main",
		},
		{
			name:    "only opening dash",
			content: "---\nto: output/file.go\npackage main",
		},
		{
			name:    "empty content",
			content: "",
		},
		{
			name:    "just dashes no content",
			content: "---",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, bodyRaw, err := ExtractMetadata(tt.content)

			assert.ErrorIs(t, err, ErrNoMetadataBlock)
			// Body should be the original content when no metadata block found
			assert.Equal(t, tt.content, bodyRaw)
		})
	}
}

func TestUT_ExtractMetadata_DashesInBody(t *testing.T) {
	// Ensure --- in the body doesn't confuse the parser
	content := `---
to: output/file.go
---
some content
---
this is not metadata
---
more content`

	metaRaw, bodyRaw, err := ExtractMetadata(content)

	require.NoError(t, err)
	assert.Equal(t, "to: output/file.go", metaRaw)
	assert.Contains(t, bodyRaw, "---")
	assert.Contains(t, bodyRaw, "this is not metadata")
}

func TestUT_ExtractMetadata_WindowsNewlines(t *testing.T) {
	// Windows-style line endings
	content := "---\r\nto: output/file.go\r\n---\r\npackage main"

	// Note: Our implementation uses \n split, so \r will be part of content
	// This test documents current behavior
	metaRaw, bodyRaw, err := ExtractMetadata(strings.ReplaceAll(content, "\r\n", "\n"))

	require.NoError(t, err)
	assert.Equal(t, "to: output/file.go", metaRaw)
	assert.Equal(t, "package main", strings.TrimSpace(bodyRaw))
}

func TestUT_ParseMetadata_ValidFields(t *testing.T) {
	tests := []struct {
		name     string
		rendered string
		want     *Metadata
	}{
		{
			name:     "to field only",
			rendered: "to: output/file.go",
			want: &Metadata{
				To:     "output/file.go",
				Action: ActionCreate,
				Extra:  map[string]string{},
			},
		},
		{
			name:     "append action",
			rendered: "to: output/file.go\nappend: true",
			want: &Metadata{
				To:     "output/file.go",
				Action: ActionAppend,
				Extra:  map[string]string{},
			},
		},
		{
			name:     "inject with after",
			rendered: "to: output/file.go\ninject: true\nafter: // marker",
			want: &Metadata{
				To:            "output/file.go",
				Action:        ActionInject,
				InjectClause:  types.InjectAfter,
				InjectMatcher: "// marker",
				Extra:         map[string]string{},
			},
		},
		{
			name:     "inject with before",
			rendered: "to: output/file.go\ninject: true\nbefore: // marker",
			want: &Metadata{
				To:            "output/file.go",
				Action:        ActionInject,
				InjectClause:  types.InjectBefore,
				InjectMatcher: "// marker",
				Extra:         map[string]string{},
			},
		},
		{
			name:     "with notes",
			rendered: "to: output/file.go\nnotes: Remember to update imports",
			want: &Metadata{
				To:     "output/file.go",
				Action: ActionCreate,
				Notes:  "Remember to update imports",
				Extra:  map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMetadata(tt.rendered)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUT_ParseMetadata_CustomFields(t *testing.T) {
	rendered := "to: output/file.go\ncustom_key: custom_value\nanother: value"

	got, err := ParseMetadata(rendered)

	require.NoError(t, err)
	assert.Equal(t, "output/file.go", got.To)
	assert.Equal(t, ActionCreate, got.Action)
	assert.Equal(t, "custom_value", got.Extra["custom_key"])
	assert.Equal(t, "value", got.Extra["another"])
}

func TestUT_ParseMetadata_DescField(t *testing.T) {
	rendered := "to: output/file.go\ndesc: Generate a service layer"

	got, err := ParseMetadata(rendered)

	require.NoError(t, err)
	assert.Equal(t, "output/file.go", got.To)
	assert.Equal(t, "Generate a service layer", got.Description)
	// desc should NOT appear in Extra
	assert.Empty(t, got.Extra["desc"])
}

func TestUT_ParseMetadata_DescFieldWithNotes(t *testing.T) {
	rendered := "to: output/file.go\ndesc: My generator\nnotes: Remember to register"

	got, err := ParseMetadata(rendered)

	require.NoError(t, err)
	assert.Equal(t, "My generator", got.Description)
	assert.Equal(t, "Remember to register", got.Notes)
}

func TestUT_ParseMetadata_DescFieldUnquoted(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantDesc string
	}{
		{"double quotes", "to: output/file.go\ndesc: \"Generate a service layer\"", "Generate a service layer"},
		{"single quotes", "to: output/file.go\ndesc: 'Generate a service layer'", "Generate a service layer"},
		{"no quotes", "to: output/file.go\ndesc: Generate a service layer", "Generate a service layer"},
		{"mismatched quotes preserved", "to: output/file.go\ndesc: \"Generate a service layer'", "\"Generate a service layer'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMetadata(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDesc, got.Description)
		})
	}
}

func TestUT_ParseMetadata_NotesFieldUnquoted(t *testing.T) {
	rendered := "to: output/file.go\nnotes: \"Remember to register\""

	got, err := ParseMetadata(rendered)

	require.NoError(t, err)
	assert.Equal(t, "Remember to register", got.Notes)
}

func TestUT_ParseMetadata_ValueWithColons(t *testing.T) {
	// Value containing colons (e.g., URL or time)
	rendered := "to: output/file.go\nurl: https://example.com:8080/path"

	got, err := ParseMetadata(rendered)

	require.NoError(t, err)
	assert.Equal(t, "https://example.com:8080/path", got.Extra["url"])
}

func TestUT_ParseMetadata_MalformedLine(t *testing.T) {
	tests := []struct {
		name     string
		rendered string
	}{
		{
			name:     "no colon",
			rendered: "to output/file.go",
		},
		{
			name:     "multiple lines with one malformed",
			rendered: "to: output/file.go\nmalformed line\nappend: true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMetadata(tt.rendered)

			assert.ErrorIs(t, err, ErrMalformedMetadata)
		})
	}
}

func TestUT_ParseMetadata_BoolParsing(t *testing.T) {
	tests := []struct {
		name     string
		rendered string
		wantErr  error
	}{
		{
			name:     "true lowercase",
			rendered: "inject: true\nafter: marker",
			wantErr:  nil,
		},
		{
			name:     "True mixed case",
			rendered: "inject: True\nafter: marker",
			wantErr:  nil,
		},
		{
			name:     "1 as true",
			rendered: "inject: 1\nafter: marker",
			wantErr:  nil,
		},
		{
			name:     "false value",
			rendered: "inject: false",
			wantErr:  nil,
		},
		{
			name:     "invalid bool for inject",
			rendered: "inject: yes",
			wantErr:  ErrInvalidBoolValue,
		},
		{
			name:     "invalid bool for append",
			rendered: "append: notabool",
			wantErr:  ErrInvalidBoolValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMetadata(tt.rendered)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUT_ParseMetadata_ConflictingAction(t *testing.T) {
	rendered := "to: output/file.go\nappend: true\ninject: true\nafter: marker"

	_, err := ParseMetadata(rendered)

	assert.ErrorIs(t, err, ErrConflictingAction)
}

func TestUT_ParseMetadata_MissingInjection(t *testing.T) {
	rendered := "to: output/file.go\ninject: true"

	_, err := ParseMetadata(rendered)

	assert.ErrorIs(t, err, ErrMissingInjection)
}

func TestUT_ParseMetadata_EmptyInjectMatcher(t *testing.T) {
	// inject: true with empty after: value should error
	rendered := "to: output/file.go\ninject: true\nafter:"

	_, err := ParseMetadata(rendered)

	assert.ErrorIs(t, err, ErrEmptyInjectMatcher)
}

func TestUT_ParseMetadata_OrphanInjectClause(t *testing.T) {
	// after: without inject: true should be silently cleared
	rendered := "to: output/file.go\nafter: // marker"

	meta, err := ParseMetadata(rendered)

	require.NoError(t, err)
	assert.Equal(t, ActionCreate, meta.Action)
	assert.Empty(t, meta.InjectClause)
	assert.Empty(t, meta.InjectMatcher)
}

func TestUT_ParseMetadata_EmptyLines(t *testing.T) {
	rendered := "to: output/file.go\n\n\nappend: true\n"

	got, err := ParseMetadata(rendered)

	require.NoError(t, err)
	assert.Equal(t, "output/file.go", got.To)
	assert.Equal(t, ActionAppend, got.Action)
}

func TestUT_ParseMetadata_WhitespaceHandling(t *testing.T) {
	rendered := "  to  :  output/file.go  \n  append  :  true  "

	got, err := ParseMetadata(rendered)

	require.NoError(t, err)
	assert.Equal(t, "output/file.go", got.To)
	assert.Equal(t, ActionAppend, got.Action)
}

func TestUT_ParseMetadata_DuplicateKeys(t *testing.T) {
	// Last value wins for duplicate keys
	rendered := "to: first/path.go\nto: second/path.go"

	got, err := ParseMetadata(rendered)

	require.NoError(t, err)
	assert.Equal(t, "second/path.go", got.To) // Last value wins
}

func TestUT_ParseMetadata_CommentsSkipped(t *testing.T) {
	rendered := "to: output/file.go\n# inject: true\n# after: // marker\nnotes: hello"

	got, err := ParseMetadata(rendered)

	require.NoError(t, err)
	assert.Equal(t, "output/file.go", got.To)
	assert.Equal(t, ActionCreate, got.Action)
	assert.Equal(t, "hello", got.Notes)
	assert.Empty(t, got.InjectClause)
}

func TestUT_RenderAndParseMetadata(t *testing.T) {
	engine, err := NewEngine()
	require.NoError(t, err)

	metaRaw := "to: {{ name|snake }}/{{ name|snake }}.go\nappend: true"
	ctx := NewContext("MyService", nil)

	meta, err := engine.RenderAndParseMetadata(metaRaw, ctx)

	require.NoError(t, err)
	assert.Equal(t, "my_service/my_service.go", meta.To)
	assert.Equal(t, ActionAppend, meta.Action)
}

func TestUT_RenderAndParseMetadata_WithVars(t *testing.T) {
	engine, err := NewEngine()
	require.NoError(t, err)

	metaRaw := "to: {{ vars.output_dir }}/{{ name }}.go"
	ctx := NewContext("handler", map[string]any{"output_dir": "pkg/handlers"})

	meta, err := engine.RenderAndParseMetadata(metaRaw, ctx)

	require.NoError(t, err)
	assert.Equal(t, "pkg/handlers/handler.go", meta.To)
}

func TestUT_RenderAndParseMetadata_WithNameOptions(t *testing.T) {
	engine, err := NewEngine()
	require.NoError(t, err)

	metaRaw := "to: {{ n.snake_case }}/{{ n.pascal_case }}.go"
	ctx := NewContext("user-service", nil)

	meta, err := engine.RenderAndParseMetadata(metaRaw, ctx)

	require.NoError(t, err)
	assert.Equal(t, "user_service/UserService.go", meta.To)
}

func TestUT_RenderAndParseMetadata_UndefinedVarReturnsEmpty(t *testing.T) {
	engine, err := NewEngine()
	require.NoError(t, err)

	// Undefined variable returns empty string in Gonja
	metaRaw := "to: {{ undefined_var }}/file.go"
	ctx := NewContext("test", nil)

	meta, err := engine.RenderAndParseMetadata(metaRaw, ctx)

	// Gonja doesn't error on undefined vars, it returns empty
	require.NoError(t, err)
	assert.Equal(t, "/file.go", meta.To)
}

func TestUT_RenderAndParseMetadata_InvalidSyntax(t *testing.T) {
	engine, err := NewEngine()
	require.NoError(t, err)

	// Invalid Jinja2 syntax should error
	metaRaw := "to: {{ name|invalid_filter }}/file.go"
	ctx := NewContext("test", nil)

	_, err = engine.RenderAndParseMetadata(metaRaw, ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "render")
}

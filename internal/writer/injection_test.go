package writer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

func Test_mergeOutputs(t *testing.T) {
	type args struct {
		source []byte
		data   []byte
		inject Inject
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "inject before token",
			args: args{
				source: []byte("fall of  // token"),
				data:   []byte("fart"),
				inject: Inject{
					Matcher: "// token",
					Clause:  types.InjectBefore,
				},
			},
			want:    []byte("fart\nfall of  // token"),
			wantErr: false,
		},
		{
			name: "inject before token at start of source",
			args: args{
				source: []byte("// token rest"),
				data:   []byte("injected"),
				inject: Inject{
					Matcher: "// token",
					Clause:  types.InjectBefore,
				},
			},
			want:    []byte("injected\n// token rest"),
			wantErr: false,
		},
		{
			name: "inject after token",
			args: args{
				source: []byte("fall of // token"),
				data:   []byte("fart"),
				inject: Inject{
					Matcher: "// token",
					Clause:  types.InjectAfter,
				},
			},
			want:    []byte("fall of // token\nfart"),
			wantErr: false,
		},
		{
			name: "no token should return source",
			args: args{
				source: []byte("fall of "),
				data:   []byte("fart"),
				inject: Inject{
					Matcher: "",
					Clause:  "",
				},
			},
			want:    []byte("fall of "),
			wantErr: true,
		},
		{
			name: "no injection clauses should return source",
			args: args{
				source: []byte("fall of man"),
				data:   []byte("fart"),
				inject: Inject{
					Matcher: "",
					Clause:  "",
				},
			},
			want:    []byte("fall of man"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mergeInjection(tt.args.source, tt.args.data, tt.args.inject)
			assert.Equal(t, string(tt.want), string(got))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUT_InjectBefore_MatcherAtStart(t *testing.T) {
	source := []byte("// marker\nrest of file")
	data := []byte("injected\n")
	inject := Inject{Matcher: "// marker", Clause: types.InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	assert.Equal(t, "injected\n// marker\nrest of file", string(got))
}

func TestUT_InjectBefore_PreservesAllContent(t *testing.T) {
	source := []byte("hello world // marker")
	data := []byte("INJECTED")
	inject := Inject{Matcher: "// marker", Clause: types.InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// Inject before the entire line containing the marker
	assert.Equal(t, "INJECTED\nhello world // marker", string(got))
}

func TestUT_InjectBefore_MultipleMatchers(t *testing.T) {
	source := []byte("// marker first // marker second")
	data := []byte("BEFORE")
	inject := Inject{Matcher: "// marker", Clause: types.InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// Should inject before the first occurrence only
	assert.Equal(t, "BEFORE\n// marker first // marker second", string(got))
}

func TestUT_InjectAfter_SingleMatcher(t *testing.T) {
	source := []byte("prefix // marker suffix")
	data := []byte(" INJECTED")
	inject := Inject{Matcher: "// marker", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	assert.Equal(t, "prefix // marker INJECTED suffix", string(got))
}

func TestUT_InjectAfter_MultipleMatchers(t *testing.T) {
	source := []byte("// marker first // marker second")
	data := []byte("AFTER")
	inject := Inject{Matcher: "// marker", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// Should inject after the first occurrence only
	assert.Equal(t, "// markerAFTER first // marker second", string(got))
}

func TestUT_InjectAfter_MatcherAtEnd(t *testing.T) {
	source := []byte("some content // marker")
	data := []byte("\nnew line")
	inject := Inject{Matcher: "// marker", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	assert.Equal(t, "some content // marker\nnew line", string(got))
}

func TestUT_InjectBefore_MarkerWithNewline(t *testing.T) {
	source := []byte("some code\n// tag:wire-imports\nimport \"existing\"\n")
	data := []byte("import \"catalog\"\n")
	inject := Inject{Matcher: "// tag:wire-imports", Clause: types.InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	assert.Equal(t, "some code\nimport \"catalog\"\n// tag:wire-imports\nimport \"existing\"\n", string(got))
}

func TestUT_InjectBefore_DataWithoutTrailingNewline(t *testing.T) {
	source := []byte("header\n// marker\nfooter\n")
	data := []byte("injected")
	inject := Inject{Matcher: "// marker", Clause: types.InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// A newline is automatically inserted so the marker stays on its own line
	assert.Equal(t, "header\ninjected\n// marker\nfooter\n", string(got))
}

func TestUT_InjectAfter_MarkerWithNewline(t *testing.T) {
	source := []byte("// tag:wire-context\n    existing code\n")
	data := []byte("    injected line\n")
	inject := Inject{Matcher: "// tag:wire-context", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	assert.Equal(t, "// tag:wire-context\n    injected line\n    existing code\n", string(got))
}

func TestUT_InjectAfter_MarkerWithCRLF(t *testing.T) {
	source := []byte("// marker\r\nrest\r\n")
	data := []byte("injected\r\n")
	inject := Inject{Matcher: "// marker", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	assert.Equal(t, "// marker\r\ninjected\r\nrest\r\n", string(got))
}

func TestUT_InjectBefore_MatcherNotFound(t *testing.T) {
	source := []byte("no match here")
	data := []byte("INJECTED")
	inject := Inject{Matcher: "// missing", Clause: types.InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.Error(t, err)
	assert.Equal(t, ErrNoMatchingExpression, err)
	assert.Equal(t, string(source), string(got))
}

func TestUT_InjectAfter_MatcherNotFound(t *testing.T) {
	source := []byte("no match here")
	data := []byte("INJECTED")
	inject := Inject{Matcher: "// missing", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.Error(t, err)
	assert.Equal(t, ErrNoMatchingExpression, err)
	assert.Equal(t, string(source), string(got))
}

func TestUT_InjectBefore_MarkerWithCRLF(t *testing.T) {
	source := []byte("header\r\n// marker\r\nfooter\r\n")
	data := []byte("injected")
	inject := Inject{Matcher: "// marker", Clause: types.InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// CRLF source should use \r\n as separator
	assert.Equal(t, "header\r\ninjected\r\n// marker\r\nfooter\r\n", string(got))
}

func TestUT_InjectBefore_EmptyData(t *testing.T) {
	source := []byte("header\n// marker\nfooter\n")
	data := []byte("")
	inject := Inject{Matcher: "// marker", Clause: types.InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// Empty data is a no-op (marker stays in place, no extra newline)
	assert.Equal(t, "header\n// marker\nfooter\n", string(got))
}

func TestUT_InjectAfter_EmptyData(t *testing.T) {
	source := []byte("// marker\ncontent\n")
	data := []byte("")
	inject := Inject{Matcher: "// marker", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	assert.Equal(t, "// marker\ncontent\n", string(got))
}

func TestUT_InjectAfter_MarkerAtEOF_DataNoNewline(t *testing.T) {
	source := []byte("some content\n// marker")
	data := []byte("appended")
	inject := Inject{Matcher: "// marker", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// When marker is at EOF without trailing newline, a newline separator is inserted
	assert.Equal(t, "some content\n// marker\nappended", string(got))
}

func TestUT_InjectAfter_MarkerAtEOF_CRLF(t *testing.T) {
	source := []byte("content\r\n// marker")
	data := []byte("appended")
	inject := Inject{Matcher: "// marker", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// CRLF source at EOF should use \r\n separator
	assert.Equal(t, "content\r\n// marker\r\nappended", string(got))
}

// --- Enhancement 3: Indentation-Aware Injection tests ---

func TestUT_InjectAfter_IndentationAware_Spaces(t *testing.T) {
	source := []byte("components:\n  schemas:\n    User:\n      type: object\n    # TAG:SCHEMAS\n")
	data := []byte("Widget:\n  type: object\n  properties:\n    id:\n      type: string\n")
	inject := Inject{Matcher: "# TAG:SCHEMAS", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	expected := "components:\n  schemas:\n    User:\n      type: object\n    # TAG:SCHEMAS\n    Widget:\n      type: object\n      properties:\n        id:\n          type: string\n"
	assert.Equal(t, expected, string(got))
}

func TestUT_InjectAfter_IndentationAware_Tabs(t *testing.T) {
	source := []byte("func main() {\n\t// TAG:INIT\n\tfmt.Println(\"done\")\n}\n")
	data := []byte("setup()\nconfig()\n")
	inject := Inject{Matcher: "// TAG:INIT", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	expected := "func main() {\n\t// TAG:INIT\n\tsetup()\n\tconfig()\n\tfmt.Println(\"done\")\n}\n"
	assert.Equal(t, expected, string(got))
}

func TestUT_InjectAfter_IndentationAware_ColumnZero(t *testing.T) {
	// Marker at column 0: behavior should be identical to pre-indentation code.
	source := []byte("// TAG:IMPORTS\npackage main\n")
	data := []byte("import \"fmt\"\n")
	inject := Inject{Matcher: "// TAG:IMPORTS", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	assert.Equal(t, "// TAG:IMPORTS\nimport \"fmt\"\npackage main\n", string(got))
}

func TestUT_InjectAfter_IndentationAware_EmptyLines(t *testing.T) {
	source := []byte("  // TAG:BLOCK\n  rest\n")
	data := []byte("line1\n\nline2\n")
	inject := Inject{Matcher: "// TAG:BLOCK", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// Empty lines should not be padded with whitespace.
	expected := "  // TAG:BLOCK\n  line1\n\n  line2\n  rest\n"
	assert.Equal(t, expected, string(got))
}

func TestUT_InjectAfter_IndentationAware_BaseIndentStripped(t *testing.T) {
	// Template itself has 2-space base indent; marker has 4-space indent.
	// Base indent should be stripped, then marker indent applied.
	source := []byte("    // TAG:HERE\n    existing\n")
	data := []byte("  foo:\n    bar: baz\n")
	inject := Inject{Matcher: "// TAG:HERE", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	expected := "    // TAG:HERE\n    foo:\n      bar: baz\n    existing\n"
	assert.Equal(t, expected, string(got))
}

func TestUT_InjectBefore_IndentationAware(t *testing.T) {
	source := []byte("items:\n    # TAG:ITEMS\n    existing: value\n")
	data := []byte("new_item: added\n")
	inject := Inject{Matcher: "# TAG:ITEMS", Clause: types.InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	expected := "items:\n    new_item: added\n    # TAG:ITEMS\n    existing: value\n"
	assert.Equal(t, expected, string(got))
}

func TestUT_InjectAfter_IndentationAware_MixedIndentLevels(t *testing.T) {
	// Relative indentation within the template should be preserved.
	source := []byte("  // TAG:CODE\n  done()\n")
	data := []byte("if true {\n  inner()\n  if nested {\n    deep()\n  }\n}\n")
	inject := Inject{Matcher: "// TAG:CODE", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	expected := "  // TAG:CODE\n  if true {\n    inner()\n    if nested {\n      deep()\n    }\n  }\n  done()\n"
	assert.Equal(t, expected, string(got))
}

func TestUT_InjectAfter_IndentationAware_CRLF(t *testing.T) {
	source := []byte("  // TAG:MARKER\r\n  rest\r\n")
	data := []byte("line1\r\nline2\r\n")
	inject := Inject{Matcher: "// TAG:MARKER", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	expected := "  // TAG:MARKER\r\n  line1\r\n  line2\r\n  rest\r\n"
	assert.Equal(t, expected, string(got))
}

func TestUT_InjectAfter_IndentationAware_LeadingNewline(t *testing.T) {
	// Data starts with an empty line — indentation should apply to subsequent lines.
	source := []byte("  // TAG:BLOCK\n  rest\n")
	data := []byte("\nfoo\nbar\n")
	inject := Inject{Matcher: "// TAG:BLOCK", Clause: types.InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	expected := "  // TAG:BLOCK\n\n  foo\n  bar\n  rest\n"
	assert.Equal(t, expected, string(got))
}

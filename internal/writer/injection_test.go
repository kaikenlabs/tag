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
			want:    []byte("fall of  fart\n// token"),
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
	// All content before the matcher must be preserved (no dropped characters)
	assert.Equal(t, "hello world INJECTED\n// marker", string(got))
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

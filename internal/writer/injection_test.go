package writer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
					Clause:  InjectBefore,
				},
			},
			want:    []byte("fall of  fart// token"),
			wantErr: false,
		},
		{
			name: "inject before token at start of source",
			args: args{
				source: []byte("// token rest"),
				data:   []byte("injected"),
				inject: Inject{
					Matcher: "// token",
					Clause:  InjectBefore,
				},
			},
			want:    []byte("injected// token rest"),
			wantErr: false,
		},
		{
			name: "inject after token",
			args: args{
				source: []byte("fall of // token"),
				data:   []byte("fart"),
				inject: Inject{
					Matcher: "// token",
					Clause:  InjectAfter,
				},
			},
			want:    []byte("fall of // tokenfart"),
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
	inject := Inject{Matcher: "// marker", Clause: InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	assert.Equal(t, "injected\n// marker\nrest of file", string(got))
}

func TestUT_InjectBefore_PreservesAllContent(t *testing.T) {
	source := []byte("hello world // marker")
	data := []byte("INJECTED")
	inject := Inject{Matcher: "// marker", Clause: InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// All content before the matcher must be preserved (no dropped characters)
	assert.Equal(t, "hello world INJECTED// marker", string(got))
}

func TestUT_InjectBefore_MultipleMatchers(t *testing.T) {
	source := []byte("// marker first // marker second")
	data := []byte("BEFORE")
	inject := Inject{Matcher: "// marker", Clause: InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// Should inject before the first occurrence only
	assert.Equal(t, "BEFORE// marker first // marker second", string(got))
}

func TestUT_InjectAfter_SingleMatcher(t *testing.T) {
	source := []byte("prefix // marker suffix")
	data := []byte(" INJECTED")
	inject := Inject{Matcher: "// marker", Clause: InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	assert.Equal(t, "prefix // marker INJECTED suffix", string(got))
}

func TestUT_InjectAfter_MultipleMatchers(t *testing.T) {
	source := []byte("// marker first // marker second")
	data := []byte("AFTER")
	inject := Inject{Matcher: "// marker", Clause: InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	// Should inject after the first occurrence only
	assert.Equal(t, "// markerAFTER first // marker second", string(got))
}

func TestUT_InjectAfter_MatcherAtEnd(t *testing.T) {
	source := []byte("some content // marker")
	data := []byte("\nnew line")
	inject := Inject{Matcher: "// marker", Clause: InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.NoError(t, err)
	assert.Equal(t, "some content // marker\nnew line", string(got))
}

func TestUT_InjectBefore_MatcherNotFound(t *testing.T) {
	source := []byte("no match here")
	data := []byte("INJECTED")
	inject := Inject{Matcher: "// missing", Clause: InjectBefore}

	got, err := mergeInjection(source, data, inject)

	require.Error(t, err)
	assert.Equal(t, ErrNoMatchingExpression, err)
	assert.Equal(t, string(source), string(got))
}

func TestUT_InjectAfter_MatcherNotFound(t *testing.T) {
	source := []byte("no match here")
	data := []byte("INJECTED")
	inject := Inject{Matcher: "// missing", Clause: InjectAfter}

	got, err := mergeInjection(source, data, inject)

	require.Error(t, err)
	assert.Equal(t, ErrNoMatchingExpression, err)
	assert.Equal(t, string(source), string(got))
}

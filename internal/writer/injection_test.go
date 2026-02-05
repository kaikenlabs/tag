package writer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_mergeOutputs(t *testing.T) {
	type args struct {
		name   string
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
				name:   "",
				source: []byte("fall of  // token"),
				data:   []byte("fart"),
				inject: Inject{
					Matcher: "// token",
					Clause:  InjectBefore,
				},
			},
			want:    []byte("fall of fart\n// token"),
			wantErr: false,
		},
		{
			name: "inject after token",
			args: args{
				name:   "",
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
				name:   "",
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
				name:   "",
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
			}
		})
	}
}

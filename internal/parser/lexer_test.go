package parser

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_hydrateData(t *testing.T) {
	type args struct {
		meta []string
		data TemplateData
	}
	tests := []struct {
		name    string
		args    args
		want    TemplateData
		wantErr bool
		err     error
	}{
		{
			name: "should return inject before",
			args: args{
				meta: []string{
					"inject: true",
					"before: // deepak",
				},
				data: TemplateData{},
			},
			want: TemplateData{
				Name: "",
				To:   "",
				ParseData: ParseData{
					Action:        ActionInject,
					InjectClause:  InjectBefore,
					InjectMatcher: "// deepak",
					Meta:          map[string]string{},
				},
			},
			wantErr: false,
		},
		{
			name: "should return inject after",
			args: args{
				meta: []string{
					"inject: true",
					"after: // deepak",
				},
				data: TemplateData{},
			},
			want: TemplateData{
				Name: "",
				To:   "",
				ParseData: ParseData{
					Action:        ActionInject,
					InjectClause:  InjectAfter,
					InjectMatcher: "// deepak",
					Meta:          map[string]string{},
				},
				Output: nil,
			},
			wantErr: false,
		},
		{
			name: "should return append",
			args: args{
				meta: []string{
					"append: true",
				},
				data: TemplateData{},
			},
			want: TemplateData{
				ParseData: ParseData{
					Action: ActionAppend,
					Meta:   map[string]string{},
				},
				Output: nil,
			},
			wantErr: false,
		},
		{
			name: "should return to",
			args: args{
				meta: []string{
					"to: example/screen/foo",
				},
				data: TemplateData{},
			},
			want: TemplateData{
				Name: "",
				To:   "example/screen/foo",
				ParseData: ParseData{
					Action: ActionCreate,
					Meta:   map[string]string{},
				},
				Output: nil,
			},
			wantErr: false,
		},
		{
			name: "should return to with meta",
			args: args{
				meta: []string{
					"block: steel",
				},
				data: TemplateData{},
			},
			want: TemplateData{
				Name: "",
				To:   "",
				ParseData: ParseData{
					Action:        ActionCreate,
					InjectClause:  "",
					InjectMatcher: "",
					Meta: map[string]string{
						"block": "steel",
					},
				},

				Output: nil,
			},
			wantErr: false,
		},
		{
			name: "should return to and remove white spaces",
			args: args{
				meta: []string{
					"  to  : steel  ",
				},
				data: TemplateData{},
			},
			want: TemplateData{
				Name: "",
				To:   "steel",
				ParseData: ParseData{
					Action: ActionCreate,
					Meta:   map[string]string{},
				},
				Output: nil,
			},
			wantErr: false,
		},
		{
			name: "should return malformed template",
			args: args{
				meta: []string{
					"  to  steel  ",
				},
				data: TemplateData{},
			},
			want: TemplateData{
				Name: "",
				To:   "",
				ParseData: ParseData{
					Action:        ActionCreate,
					InjectClause:  "",
					InjectMatcher: "",
				},
				Output: nil,
			},
			wantErr: true,
			err:     ErrMalformedTemplate,
		},
		{
			name: "should skip parse and return with error",
			args: args{
				meta: []string{
					"  to  steel  ",
				},
				data: TemplateData{
					ParseData: ParseData{
						Meta: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			want: TemplateData{
				Name:   "",
				To:     "",
				Output: nil,
				ParseData: ParseData{
					Action: ActionCreate,
					Meta: map[string]string{
						"foo": "bar",
					},
				},
			},
			wantErr: true,
			err:     ErrMalformedTemplate,
		},
		{
			name: "should return err parsing bool",
			args: args{
				meta: []string{
					"inject: flash gordon",
				},
				data: TemplateData{
					ParseData: ParseData{
						Meta: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			want: TemplateData{
				Name:   "",
				To:     "",
				Output: nil,
				ParseData: ParseData{
					Action: ActionInject,
					Meta: map[string]string{
						"foo": "bar",
					},
				},
			},
			wantErr: true,
			err:     ErrParsingBool,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hydrateData(tt.args.meta, tt.args.data)
			if tt.wantErr {
				assert.True(t, errors.Is(err, tt.err))
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_extractMeta(t *testing.T) {
	type args struct {
		output string
	}
	tests := []struct {
		name   string
		args   args
		meta   []string
		output string
	}{
		{
			name: "should return meta block",
			args: args{
				output: `---
				to: foo
				---
				`,
			},
			meta:   []string{"to: foo"},
			output: "",
		},
		{
			name: "should empty if no block",
			args: args{
				output: `
				to: foo
				`,
			},
			meta:   []string{},
			output: "",
		},
		{
			name: "should return meta and block",
			args: args{
				output: `---
				append: true
				---
				blah
				`,
			},
			meta:   []string{"append: true"},
			output: "blah",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := extractMetaDataFromTemplate(tt.args.output)
			require.Equal(t, tt.meta, got)
			assert.Equal(t, tt.output, strings.TrimSpace(got1))
		})
	}
}

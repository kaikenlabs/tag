package parser

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/code-gorilla-au/odize"
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
				odize.AssertTrue(t, errors.Is(err, tt.err))
			}
			gotJSON, err := json.Marshal(&got)
			odize.AssertNoError(t, err)
			wantJSON, err := json.Marshal(tt.want)
			odize.AssertNoError(t, err)
			odize.AssertEqual(t, string(wantJSON), string(gotJSON))
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
			if !reflect.DeepEqual(got, tt.meta) {
				t.Errorf("extractMeta() got = %v, want %v", got, tt.meta)
			}
			if strings.TrimSpace(got1) != tt.output {
				t.Errorf("extractMeta() got1 = %v, want %v", got1, tt.output)
			}
		})
	}
}

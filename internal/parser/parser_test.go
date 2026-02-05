package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_withTemplates(t *testing.T) {
	type args struct {
		fileSuffix string
		dirPath    string
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "should return inject_after.tmpl",
			args: args{
				fileSuffix: "tmpl",
				dirPath:    "../../example/_templates/fakr",
			},
			want:    7,
			wantErr: false,
		},
		{
			name: "should return error if not exists",
			args: args{
				fileSuffix: "tmpl",
				dirPath:    "../../example/_templates/flat",
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := withTemplates(tt.args.dirPath, tt.args.fileSuffix)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, len(got))
			for _, tmp := range got {
				assert.True(t, len(tmp) > 0)
			}
		})
	}
}

func TestTmplEngine_Parse_should_render(t *testing.T) {
	strTmp := `---
	to: elo
	---
	blah
	`
	expected := TemplateData{
		Name: "hello",
	}
	te := &TemplateEngine{
		templates: map[string]string{"tmp": strTmp},
		funcs:     defaultFuncs,
	}
	data, err := te.Parse(expected)
	require.NoError(t, err)
	assert.Equal(t, expected.Name, data[0].Name)
	assert.Equal(t, "elo", data[0].To)
	assert.Equal(t, "blah", strings.TrimSpace(string(data[0].Output)))
}

func TestTmplEngine_Parse_missing_funcs_should_fail_on_meta_parse(t *testing.T) {
	strTmp := `---
	to: {{ "elo" | caseSnake }}
	---
	blah
	`
	expected := TemplateData{
		Name: "hello",
	}
	te := &TemplateEngine{
		templates: map[string]string{"tmp": strTmp},
	}
	_, err := te.Parse(expected)
	assert.Error(t, err)
}

func TestTmplEngine_Parse_missing_funcs_should_fail_on_template_parse(t *testing.T) {
	strTmp := `---
	to: elo
	---
	blah {{ "foo" | caseSnake }}
	`
	expected := TemplateData{
		Name: "hello",
	}
	te := &TemplateEngine{
		templates: map[string]string{"tmp": strTmp},
	}
	_, err := te.Parse(expected)
	assert.Error(t, err)
}

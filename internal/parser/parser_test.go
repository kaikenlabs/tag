package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_withTemplates(t *testing.T) {
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
				dirPath:    "../../example/.tag.templates/fakr",
			},
			want:    7,
			wantErr: false,
		},
		{
			name: "should return error if not exists",
			args: args{
				fileSuffix: "tmpl",
				dirPath:    "../../example/.tag.templates/flat",
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

func TestUT_Parse_SimpleTemplate(t *testing.T) {
	// Simple template with Jinja2 syntax
	strTmp := `---
to: output/file.go
---
hello {{ name }}
`
	te, err := New(".", "", "")
	require.NoError(t, err)

	// Override with test template
	te.templates = map[string]string{"tmp": strTmp}

	input := InputData{
		Name: "world",
	}
	data, err := te.Parse(input)
	require.NoError(t, err)
	require.Len(t, data, 1)
	assert.Equal(t, "output/file.go", data[0].To)
	assert.Equal(t, "hello world", strings.TrimSpace(string(data[0].Output)))
}

func TestUT_Parse_MetadataRendering(t *testing.T) {
	// Template with dynamic path using filter
	strTmp := `---
to: {{ name|snake }}/{{ name|snake }}.go
---
package {{ name|snake }}
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	input := InputData{
		Name: "MyService",
	}
	data, err := te.Parse(input)
	require.NoError(t, err)
	require.Len(t, data, 1)
	assert.Equal(t, "my_service/my_service.go", data[0].To)
	assert.Contains(t, string(data[0].Output), "package my_service")
}

func TestUT_Parse_FilterUsage(t *testing.T) {
	strTmp := `---
to: output.go
---
snake: {{ name|snake }}
pascal: {{ name|pascal }}
camel: {{ name|camel }}
kebab: {{ name|kebab }}
plural: {{ name|plural }}
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	input := InputData{
		Name: "UserService",
	}
	data, err := te.Parse(input)
	require.NoError(t, err)
	require.Len(t, data, 1)

	output := string(data[0].Output)
	assert.Contains(t, output, "snake: user_service")
	assert.Contains(t, output, "pascal: UserService")
	assert.Contains(t, output, "camel: userService")
	assert.Contains(t, output, "kebab: user-service")
	assert.Contains(t, output, "plural: UserServices")
}

func TestUT_Parse_VarsAccess(t *testing.T) {
	strTmp := `---
to: {{ vars.output_dir }}/file.go
---
// Module: {{ vars.module_name }}
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	input := InputData{
		Name: "test",
		Meta: map[string]string{
			"output_dir":  "pkg/handlers",
			"module_name": "my_module",
		},
	}
	data, err := te.Parse(input)
	require.NoError(t, err)
	require.Len(t, data, 1)
	assert.Equal(t, "pkg/handlers/file.go", data[0].To)
	assert.Contains(t, string(data[0].Output), "Module: my_module")
}

func TestUT_Parse_NameOptions(t *testing.T) {
	strTmp := `---
to: output.go
---
n.snake_case: {{ n.snake_case }}
n.pascal_case: {{ n.pascal_case }}
n.camel_case: {{ n.camel_case }}
n.kebab_case: {{ n.kebab_case }}
n.lower_case: {{ n.lower_case }}
n.upper_case: {{ n.upper_case }}
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	input := InputData{
		Name: "MyService",
	}
	data, err := te.Parse(input)
	require.NoError(t, err)
	require.Len(t, data, 1)

	output := string(data[0].Output)
	assert.Contains(t, output, "n.snake_case: my_service")
	assert.Contains(t, output, "n.pascal_case: MyService")
	assert.Contains(t, output, "n.camel_case: myService")
	assert.Contains(t, output, "n.kebab_case: my-service")
	assert.Contains(t, output, "n.lower_case: myservice")
	assert.Contains(t, output, "n.upper_case: MYSERVICE")
}

func TestUT_Parse_ActionCreate(t *testing.T) {
	strTmp := `---
to: output.go
---
content
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	assert.Equal(t, ActionCreate, data[0].Action)
}

func TestUT_Parse_ActionAppend(t *testing.T) {
	strTmp := `---
to: output.go
append: true
---
appended content
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	assert.Equal(t, ActionAppend, data[0].Action)
}

func TestUT_Parse_ActionInjectAfter(t *testing.T) {
	strTmp := `---
to: output.go
inject: true
after: // marker
---
injected content
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	assert.Equal(t, ActionInject, data[0].Action)
	assert.Equal(t, InjectAfter, data[0].InjectClause)
	assert.Equal(t, "// marker", data[0].InjectMatcher)
}

func TestUT_Parse_ActionInjectBefore(t *testing.T) {
	strTmp := `---
to: output.go
inject: true
before: // marker
---
injected content
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	assert.Equal(t, ActionInject, data[0].Action)
	assert.Equal(t, InjectBefore, data[0].InjectClause)
	assert.Equal(t, "// marker", data[0].InjectMatcher)
}

func TestUT_Parse_TemplateDefinedMeta(t *testing.T) {
	// Template defines its own metadata that can be used in body
	strTmp := `---
to: output.go
custom_key: custom_value
---
value: {{ vars.custom_key }}
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	assert.Contains(t, string(data[0].Output), "value: custom_value")
	assert.Equal(t, "custom_value", data[0].Meta["custom_key"])
}

func TestUT_Parse_CLIMetaOverridesTemplate(t *testing.T) {
	strTmp := `---
to: output.go
value: template_default
---
value: {{ vars.value }}
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	input := InputData{
		Name: "test",
		Meta: map[string]string{
			"value": "cli_override",
		},
	}
	data, err := te.Parse(input)
	require.NoError(t, err)
	// CLI value takes precedence in output
	assert.Contains(t, string(data[0].Output), "value: cli_override")
}

func TestUT_Parse_InvalidFilterErrors(t *testing.T) {
	strTmp := `---
to: output.go
---
{{ name|invalid_filter_xyz }}
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	_, err = te.Parse(InputData{Name: "test"})
	assert.Error(t, err)
}

func TestUT_Parse_OrderingByAction(t *testing.T) {
	templates := map[string]string{
		"append.tmpl": `---
to: output.go
append: true
---
append`,
		"inject.tmpl": `---
to: output.go
inject: true
after: marker
---
inject`,
		"create.tmpl": `---
to: output.go
---
create`,
	}
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = templates

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	require.Len(t, data, 3)

	// Should be ordered: Create, Inject, Append
	assert.Equal(t, ActionCreate, data[0].Action)
	assert.Equal(t, ActionInject, data[1].Action)
	assert.Equal(t, ActionAppend, data[2].Action)
}

func TestUT_Parse_JinjaConditional(t *testing.T) {
	strTmp := `---
to: output.go
---
{% if vars.use_feature %}
feature enabled
{% else %}
feature disabled
{% endif %}
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	// With feature enabled
	input := InputData{
		Name: "test",
		Meta: map[string]string{"use_feature": "true"},
	}
	data, err := te.Parse(input)
	require.NoError(t, err)
	assert.Contains(t, string(data[0].Output), "feature enabled")
	assert.NotContains(t, string(data[0].Output), "feature disabled")
}

func TestUT_Parse_JinjaLoop(t *testing.T) {
	strTmp := `---
to: output.go
---
{% for i in range(3) %}
item {{ i }}
{% endfor %}
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	output := string(data[0].Output)
	assert.Contains(t, output, "item 0")
	assert.Contains(t, output, "item 1")
	assert.Contains(t, output, "item 2")
}

func TestUT_Parse_MissingToFieldErrors(t *testing.T) {
	// Template without 'to' field should error
	strTmp := `---
append: true
---
content without destination
`
	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	_, err = te.Parse(InputData{Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "to")
}

func TestUT_Parse_NoMetadataBlockErrors(t *testing.T) {
	// Template without metadata block should error (no 'to' field)
	strTmp := `just content without metadata block`

	te, err := New(".", "", "")
	require.NoError(t, err)
	te.templates = map[string]string{"tmp": strTmp}

	_, err = te.Parse(InputData{Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "to")
}

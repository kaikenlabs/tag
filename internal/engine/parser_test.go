package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/types"
)

// mockExecutor implements template.TemplateExecutor for testing NewParserWithExecutor.
type mockExecutor struct {
	executeToStringResult string
	executeToStringErr    error
	renderMetadataResult  *template.Metadata
	renderMetadataErr     error
	parseStringTemplate   template.Template
	parseStringErr        error
}

func (m *mockExecutor) ParseString(_ string) (template.Template, error) {
	return m.parseStringTemplate, m.parseStringErr
}

func (m *mockExecutor) ParseStringNamed(_, _ string) (template.Template, error) {
	return m.parseStringTemplate, m.parseStringErr
}

func (m *mockExecutor) ExecuteToString(_ string, _ template.Context) (string, error) {
	return m.executeToStringResult, m.executeToStringErr
}

func (m *mockExecutor) RenderAndParseMetadata(_ string, _ template.Context) (*template.Metadata, error) {
	return m.renderMetadataResult, m.renderMetadataErr
}

var _ template.TemplateExecutor = (*mockExecutor)(nil)

// mockTemplate implements template.Template for testing.
type mockTemplate struct {
	result string
	err    error
}

func (m *mockTemplate) Execute(_ template.Context) (string, error) {
	return m.result, m.err
}

var _ template.Template = (*mockTemplate)(nil)

// newTestParser creates a TemplateParser for tests using a real template engine.
// This replaces the old parser.New() / newParserLegacy() in tests.
func newTestParser(t *testing.T) TemplateParser {
	t.Helper()
	eng, err := template.NewEngine()
	require.NoError(t, err)
	return NewParserWithExecutor(eng, nil, nil)
}

func TestUT_LoadTemplateFiles(t *testing.T) {
	tests := []struct {
		name    string
		dirPath string
		want    int
		wantErr bool
	}{
		{
			name:    "should load all templates from directory",
			dirPath: "testdata/generators",
			want:    7,
			wantErr: false,
		},
		{
			name:    "should return error if not exists",
			dirPath: "testdata/nonexistent",
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadTemplateFiles(tt.dirPath)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, len(got))
			for _, tmp := range got {
				assert.True(t, tmp != "")
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
	te := newTestParser(t)

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
	te := newTestParser(t)
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
	te := newTestParser(t)
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
	te := newTestParser(t)
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
	te := newTestParser(t)
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
	te := newTestParser(t)
	te.templates = map[string]string{"tmp": strTmp}

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	assert.Equal(t, template.ActionCreate, data[0].Action)
}

func TestUT_Parse_ActionAppend(t *testing.T) {
	strTmp := `---
to: output.go
append: true
---
appended content
`
	te := newTestParser(t)
	te.templates = map[string]string{"tmp": strTmp}

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	assert.Equal(t, template.ActionAppend, data[0].Action)
}

func TestUT_Parse_ActionInjectAfter(t *testing.T) {
	strTmp := `---
to: output.go
inject: true
after: // marker
---
injected content
`
	te := newTestParser(t)
	te.templates = map[string]string{"tmp": strTmp}

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	assert.Equal(t, template.ActionInject, data[0].Action)
	assert.Equal(t, types.InjectAfter, data[0].InjectClause)
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
	te := newTestParser(t)
	te.templates = map[string]string{"tmp": strTmp}

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	assert.Equal(t, template.ActionInject, data[0].Action)
	assert.Equal(t, types.InjectBefore, data[0].InjectClause)
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
	te := newTestParser(t)
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
	te := newTestParser(t)
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
	te := newTestParser(t)
	te.templates = map[string]string{"tmp": strTmp}

	_, err := te.Parse(InputData{Name: "test"})
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
	te := newTestParser(t)
	te.templates = templates

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	require.Len(t, data, 3)

	// Should be ordered: Create, Inject, Append
	assert.Equal(t, template.ActionCreate, data[0].Action)
	assert.Equal(t, template.ActionInject, data[1].Action)
	assert.Equal(t, template.ActionAppend, data[2].Action)
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
	te := newTestParser(t)
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
	te := newTestParser(t)
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
	te := newTestParser(t)
	te.templates = map[string]string{"tmp": strTmp}

	_, err := te.Parse(InputData{Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "to")
}

func TestUT_Parse_NoMetadataBlockErrors(t *testing.T) {
	// Template without metadata block should error (no 'to' field)
	strTmp := `just content without metadata block`

	te := newTestParser(t)
	te.templates = map[string]string{"tmp": strTmp}

	_, err := te.Parse(InputData{Name: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "to")
}

func TestUT_NewParserWithExecutor_ParsesTemplate(t *testing.T) {
	// Verify NewParserWithExecutor wires the mock executor correctly.
	mock := &mockExecutor{
		renderMetadataResult: &template.Metadata{
			To:     "output/service.go",
			Action: template.ActionCreate,
			Extra:  map[string]string{},
		},
		parseStringTemplate: &mockTemplate{result: "generated code"},
	}

	tmplContent := "---\nto: output/service.go\n---\ngenerated code\n"
	te := NewParserWithExecutor(mock, map[string]string{"svc.tmpl": tmplContent}, nil)

	data, err := te.Parse(InputData{Name: "MyService"})
	require.NoError(t, err)
	require.Len(t, data, 1)
	assert.Equal(t, "output/service.go", data[0].To)
	assert.Equal(t, template.ActionCreate, data[0].Action)
	assert.Equal(t, "generated code", string(data[0].Output))
}

func TestUT_NewParserWithExecutor_MetadataError(t *testing.T) {
	// Verify that metadata rendering errors propagate correctly.
	mock := &mockExecutor{
		renderMetadataErr: errors.New("mock metadata error"),
	}

	tmplContent := "---\nto: output.go\n---\nbody\n"
	te := NewParserWithExecutor(mock, map[string]string{"tmpl": tmplContent}, nil)

	_, err := te.Parse(InputData{Name: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mock metadata error")
}

func TestUT_NewParserWithExecutor_BodyRenderError(t *testing.T) {
	// Verify that body rendering errors propagate correctly.
	mock := &mockExecutor{
		renderMetadataResult: &template.Metadata{
			To:     "output.go",
			Action: template.ActionCreate,
			Extra:  map[string]string{},
		},
		parseStringErr: errors.New("mock parse error"),
	}

	tmplContent := "---\nto: output.go\n---\nbody\n"
	te := NewParserWithExecutor(mock, map[string]string{"tmpl": tmplContent}, nil)

	_, err := te.Parse(InputData{Name: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mock parse error")
}

func TestUT_Parse_SharedTemplateInclude(t *testing.T) {
	// Verify {% include %} resolves shared templates when wired through a real engine.
	eng, err := template.NewEngine()
	require.NoError(t, err)

	// Shared templates keyed by name (LoadTemplateFiles returns path-keyed map;
	// CreateMemoryLoaderFromMap normalises keys with leading "/").
	shared := map[string]string{
		"header.tmpl": "// AUTO-GENERATED — DO NOT EDIT",
	}
	loader := template.CreateMemoryLoaderFromMap(shared)
	eng.SetLoader(loader)
	eng.SetSharedContent(shared)

	primary := map[string]string{
		"component.tmpl": "---\nto: app/component.go\n---\n{% include \"header.tmpl\" %}\npackage app\n",
	}
	te := NewParserWithExecutor(eng, primary, shared)

	data, err := te.Parse(InputData{Name: "widget"})
	require.NoError(t, err)
	require.Len(t, data, 1)
	assert.Equal(t, "app/component.go", data[0].To)
	assert.Contains(t, string(data[0].Output), "// AUTO-GENERATED")
	assert.Contains(t, string(data[0].Output), "package app")
}

func TestUT_Parse_MalformedYAMLMetadata(t *testing.T) {
	// Template with metadata that has no colon separator.
	strTmp := "---\nthis is not valid metadata\n---\nbody\n"
	te := newTestParser(t)
	te.templates = map[string]string{"bad.tmpl": strTmp}

	_, err := te.Parse(InputData{Name: "test"})
	require.Error(t, err)
}

func TestUT_Parse_TemplateSyntaxErrorInBody(t *testing.T) {
	// Unclosed if block should cause a parse/render error.
	strTmp := "---\nto: output.go\n---\n{% if vars.x %}\nmissing endif\n"
	te := newTestParser(t)
	te.templates = map[string]string{"bad.tmpl": strTmp}

	_, err := te.Parse(InputData{Name: "test"})
	require.Error(t, err)
}

func TestUT_Parse_EmptyGeneratorDir(t *testing.T) {
	dir := t.TempDir()
	templates, err := LoadTemplateFiles(dir)
	require.NoError(t, err)

	te := newTestParser(t)
	te.templates = templates

	data, err := te.Parse(InputData{Name: "test"})
	require.NoError(t, err)
	assert.Empty(t, data, "empty directory should produce no parsed templates")
}

// TestUT_LoadTemplateFiles_SkipsTemplateConfigFile pins #335: a generator that
// ships its own tag.template.json must still generate. The config file is skipped
// as a template while every real template — including near-miss filenames — loads.
func TestUT_LoadTemplateFiles_SkipsTemplateConfigFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	write(types.TemplateConfigFile, `{"vars": {"port": {"type": "string", "default": "8080"}}}`)
	write("handler.tmpl", "---\nto: internal/{{ name | snake_case }}.go\n---\npackage internal\n")
	// Near-miss names are ordinary templates; the skip must key on the exact base name.
	write("tag.template.json.tmpl", "---\nto: near_miss_suffix.go\n---\nbody\n")
	write("my.tag.template.json", "---\nto: near_miss_prefix.go\n---\nbody\n")

	templates, err := LoadTemplateFiles(dir)
	require.NoError(t, err)

	loaded := make([]string, 0, len(templates))
	for path := range templates {
		loaded = append(loaded, filepath.Base(path))
	}
	assert.ElementsMatch(t, []string{"handler.tmpl", "tag.template.json.tmpl", "my.tag.template.json"}, loaded)

	// The whole point of #335: Parse no longer aborts on the config file.
	te := newTestParser(t)
	te.templates = templates
	data, err := te.Parse(InputData{Name: "UserService"})
	require.NoError(t, err)

	tos := make([]string, 0, len(data))
	for _, d := range data {
		tos = append(tos, d.To)
	}
	assert.ElementsMatch(t, []string{"internal/user_service.go", "near_miss_suffix.go", "near_miss_prefix.go"}, tos)
}

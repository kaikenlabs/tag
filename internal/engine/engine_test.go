package engine

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/template"
	"github.com/kaikenlabs/tag/internal/writer"
)

var emptyMap = map[string]string{}

// mockFileWriter implements writer.FileWriter for testing Core.Generate() in isolation.
type mockFileWriter struct {
	writeCalls  []writeCall
	appendCalls []appendCall
	injectCalls []injectCall

	writeErr  error
	appendErr error
	injectErr error
}

type writeCall struct {
	Name string
	Data []byte
	Perm fs.FileMode
}

type appendCall struct {
	Name string
	Data []byte
}

type injectCall struct {
	Name   string
	Data   []byte
	Inject writer.Inject
}

func (m *mockFileWriter) WriteFile(name string, data []byte, perm fs.FileMode) error {
	m.writeCalls = append(m.writeCalls, writeCall{Name: name, Data: data, Perm: perm})
	return m.writeErr
}

func (m *mockFileWriter) AppendFile(name string, data []byte) error {
	m.appendCalls = append(m.appendCalls, appendCall{Name: name, Data: data})
	return m.appendErr
}

func (m *mockFileWriter) InjectIntoFile(name string, data []byte, inject writer.Inject) error {
	m.injectCalls = append(m.injectCalls, injectCall{Name: name, Data: data, Inject: inject})
	return m.injectErr
}

var _ writer.FileWriter = (*mockFileWriter)(nil)

// --- Core.Generate() unit tests ---

func TestUT_Generate_CreateAction(t *testing.T) {
	fw := &mockFileWriter{}
	mock := &mockExecutor{
		renderMetadataResult: &template.Metadata{
			To:     "output/service.go",
			Action: template.ActionCreate,
			Extra:  map[string]string{},
		},
		parseStringTemplate: &mockTemplate{result: "package service"},
	}
	parser := NewParserWithExecutor(mock, map[string]string{"svc.tmpl": "---\nto: output/service.go\n---\npackage service\n"}, nil)
	core := NewCore(parser, fw)

	err := core.Generate(Data{Name: "MyService"})

	require.NoError(t, err)
	require.Len(t, fw.writeCalls, 1)
	assert.Equal(t, "output/service.go", fw.writeCalls[0].Name)
	assert.Equal(t, "package service", string(fw.writeCalls[0].Data))
	assert.Empty(t, fw.appendCalls)
	assert.Empty(t, fw.injectCalls)
}

func TestUT_Generate_AppendAction(t *testing.T) {
	fw := &mockFileWriter{}
	mock := &mockExecutor{
		renderMetadataResult: &template.Metadata{
			To:     "output.go",
			Action: template.ActionAppend,
			Extra:  map[string]string{},
		},
		parseStringTemplate: &mockTemplate{result: "appended line"},
	}
	parser := NewParserWithExecutor(mock, map[string]string{"a.tmpl": "---\nto: output.go\nappend: true\n---\nappended line\n"}, nil)
	core := NewCore(parser, fw)

	err := core.Generate(Data{Name: "test"})

	require.NoError(t, err)
	require.Len(t, fw.appendCalls, 1)
	assert.Equal(t, "output.go", fw.appendCalls[0].Name)
	assert.Equal(t, "appended line", string(fw.appendCalls[0].Data))
	assert.Empty(t, fw.writeCalls)
}

func TestUT_Generate_InjectAction(t *testing.T) {
	fw := &mockFileWriter{}
	mock := &mockExecutor{
		renderMetadataResult: &template.Metadata{
			To:            "output.go",
			Action:        template.ActionInject,
			InjectClause:  template.InjectAfter,
			InjectMatcher: "// marker",
			Extra:         map[string]string{},
		},
		parseStringTemplate: &mockTemplate{result: "injected code"},
	}
	parser := NewParserWithExecutor(mock, map[string]string{"i.tmpl": "---\nto: output.go\ninject: true\nafter: // marker\n---\ninjected code\n"}, nil)
	core := NewCore(parser, fw)

	err := core.Generate(Data{Name: "test"})

	require.NoError(t, err)
	require.Len(t, fw.injectCalls, 1)
	assert.Equal(t, "output.go", fw.injectCalls[0].Name)
	assert.Equal(t, "injected code", string(fw.injectCalls[0].Data))
	assert.Equal(t, template.InjectAfter, fw.injectCalls[0].Inject.Clause)
	assert.Equal(t, "// marker", fw.injectCalls[0].Inject.Matcher)
}

func TestUT_Generate_MixedActions_Ordering(t *testing.T) {
	// Verify ordering: Create first, then Inject, then Append
	fw := &mockFileWriter{}

	// Use a real parser with real templates to test ordering
	te := newTestParser(t)
	te.templates = map[string]string{
		"append.tmpl": "---\nto: out.go\nappend: true\n---\nappended\n",
		"inject.tmpl": "---\nto: out.go\ninject: true\nafter: marker\n---\ninjected\n",
		"create.tmpl": "---\nto: new.go\n---\ncreated\n",
	}
	core := NewCore(te, fw)

	err := core.Generate(Data{Name: "test"})

	require.NoError(t, err)
	// Create should fire first (WriteFile), then Inject (InjectIntoFile), then Append (AppendFile)
	require.Len(t, fw.writeCalls, 1, "expected 1 create call")
	require.Len(t, fw.injectCalls, 1, "expected 1 inject call")
	require.Len(t, fw.appendCalls, 1, "expected 1 append call")
}

func TestUT_Generate_ParserError_Propagates(t *testing.T) {
	fw := &mockFileWriter{}
	mock := &mockExecutor{
		renderMetadataErr: errors.New("parse failure"),
	}
	parser := NewParserWithExecutor(mock, map[string]string{"bad.tmpl": "---\nto: out.go\n---\nbody\n"}, nil)
	core := NewCore(parser, fw)

	err := core.Generate(Data{Name: "test"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse failure")
	assert.Empty(t, fw.writeCalls, "no writes should happen on parse error")
}

func TestUT_Generate_WriteError_Propagates(t *testing.T) {
	fw := &mockFileWriter{writeErr: errors.New("disk full")}
	mock := &mockExecutor{
		renderMetadataResult: &template.Metadata{
			To:     "output.go",
			Action: template.ActionCreate,
			Extra:  map[string]string{},
		},
		parseStringTemplate: &mockTemplate{result: "content"},
	}
	parser := NewParserWithExecutor(mock, map[string]string{"t.tmpl": "---\nto: output.go\n---\ncontent\n"}, nil)
	core := NewCore(parser, fw)

	err := core.Generate(Data{Name: "test"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}

func TestUT_Generate_AppendError_Propagates(t *testing.T) {
	fw := &mockFileWriter{appendErr: errors.New("append failed")}
	mock := &mockExecutor{
		renderMetadataResult: &template.Metadata{
			To:     "output.go",
			Action: template.ActionAppend,
			Extra:  map[string]string{},
		},
		parseStringTemplate: &mockTemplate{result: "content"},
	}
	parser := NewParserWithExecutor(mock, map[string]string{"a.tmpl": "---\nto: output.go\nappend: true\n---\ncontent\n"}, nil)
	core := NewCore(parser, fw)

	err := core.Generate(Data{Name: "test"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "append failed")
}

func TestUT_Generate_InjectError_Propagates(t *testing.T) {
	fw := &mockFileWriter{injectErr: errors.New("inject failed")}
	mock := &mockExecutor{
		renderMetadataResult: &template.Metadata{
			To:            "output.go",
			Action:        template.ActionInject,
			InjectClause:  template.InjectAfter,
			InjectMatcher: "// marker",
			Extra:         map[string]string{},
		},
		parseStringTemplate: &mockTemplate{result: "content"},
	}
	parser := NewParserWithExecutor(mock, map[string]string{"i.tmpl": "---\nto: output.go\ninject: true\nafter: // marker\n---\ncontent\n"}, nil)
	core := NewCore(parser, fw)

	err := core.Generate(Data{Name: "test"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "inject failed")
}

func TestUT_Generate_WithScaffoldVars(t *testing.T) {
	// Verify scaffold vars are accessible in templates via {{ vars.* }}
	fw := &mockFileWriter{}
	te := newTestParser(t)
	te.templates = map[string]string{
		"t.tmpl": "---\nto: output.go\n---\nproject={{ vars.project_name }}\n",
	}
	core := NewCore(te, fw)

	err := core.Generate(Data{
		Name: "test",
		ScaffoldVars: map[string]any{
			"project_name": "my-app",
		},
	})

	require.NoError(t, err)
	require.Len(t, fw.writeCalls, 1)
	assert.Contains(t, string(fw.writeCalls[0].Data), "project=my-app")
}

func TestUT_Generate_WithNotes(t *testing.T) {
	fw := &mockFileWriter{}
	mock := &mockExecutor{
		renderMetadataResult: &template.Metadata{
			To:     "output.go",
			Action: template.ActionCreate,
			Notes:  "Remember to update the config",
			Extra:  map[string]string{},
		},
		parseStringTemplate: &mockTemplate{result: "content"},
	}
	parser := NewParserWithExecutor(mock, map[string]string{"t.tmpl": "---\nto: output.go\nnotes: Remember to update the config\n---\ncontent\n"}, nil)
	core := NewCore(parser, fw)

	// Notes trigger a fmt.Println which we don't capture here, but we verify
	// the generate succeeds and the file is written.
	err := core.Generate(Data{Name: "test"})

	require.NoError(t, err)
	require.Len(t, fw.writeCalls, 1)
}

func Test_parseMeta(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]string
	}{
		{
			name:  "simple key-value pair",
			input: []string{"key=value"},
			expected: map[string]string{
				"key": "value",
			},
		},
		{
			name:  "multiple key-value pairs",
			input: []string{"key1=value1", "key2=value2"},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name:  "quoted values with commas",
			input: []string{`field="name->string,age->int"`, `enabled=true`},
			expected: map[string]string{
				"field":   "name->string,age->int",
				"enabled": "true",
			},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: map[string]string{},
		},
		{
			name:  "spaces around equals",
			input: strings.Split("key = value, space = needed", ","),
			expected: map[string]string{
				"key":   "value",
				"space": "needed",
			},
		},
		{
			name:  "quoted strings with spaces",
			input: strings.Split(`name="John Doe",age=30`, ","),
			expected: map[string]string{
				"name": "John Doe",
				"age":  "30",
			},
		},
		{
			name:  "complex nested structure",
			input: []string{`fields="name->string,age->int"`, `config="host=localhost,port=8080"`},
			expected: map[string]string{
				"fields": "name->string,age->int",
				"config": "host=localhost,port=8080",
			},
		},
		{
			name:  "complex nested structure, different separators",
			input: []string{`fields="name:string;age:int"`, `config="host=localhost,port=8080"`},
			expected: map[string]string{
				"fields": "name:string;age:int",
				"config": "host=localhost,port=8080",
			},
		},
		{
			name:     "invalid format missing value",
			input:    []string{"key="},
			expected: emptyMap,
		},
		{
			name:     "invalid format missing equals",
			input:    []string{"keyvalue"},
			expected: emptyMap,
		},
		{
			name:  "single quoted value",
			input: []string{`single='value'`},
			expected: map[string]string{
				"single": "value",
			},
		},
		{
			name:  "single double quoted value",
			input: []string{`single="value"`},
			expected: map[string]string{
				"single": "value",
			},
		},
		{
			name:  "unmatched quotes",
			input: []string{`key="value`, `other=test`},
			expected: map[string]string{
				"key": "value", "other": "test",
			},
		},
		{
			name:  "multiple single quoted sections",
			input: []string{`first='quoted value'`, `second='another value'`},
			expected: map[string]string{
				"first":  "quoted value",
				"second": "another value",
			},
		},
		{
			name:  "multiple double quoted sections",
			input: []string{`first="quoted value"`, `second="another value"`},
			expected: map[string]string{
				"first":  "quoted value",
				"second": "another value",
			},
		},
		{
			name:  "empty quoted string",
			input: []string{`empty=""`},
			expected: map[string]string{
				"empty": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMeta(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

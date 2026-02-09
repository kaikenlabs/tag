package scaffold

import (
	"fmt"
	"testing"

	"github.com/kaikenlabs/tag/internal/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTemplateExecutor implements template.TemplateExecutor for testing.
type mockTemplateExecutor struct {
	parseStringResult     template.Template
	parseStringErr        error
	executeToStringResult string
	executeToStringErr    error
	renderMetadataResult  *template.Metadata
	renderMetadataErr     error
}

func (m *mockTemplateExecutor) ParseString(_ string) (template.Template, error) {
	return m.parseStringResult, m.parseStringErr
}

func (m *mockTemplateExecutor) ParseStringNamed(_, _ string) (template.Template, error) {
	return m.parseStringResult, m.parseStringErr
}

func (m *mockTemplateExecutor) ExecuteToString(_ string, _ template.Context) (string, error) {
	return m.executeToStringResult, m.executeToStringErr
}

func (m *mockTemplateExecutor) RenderAndParseMetadata(_ string, _ template.Context) (*template.Metadata, error) {
	return m.renderMetadataResult, m.renderMetadataErr
}

// Compile-time check: mockTemplateExecutor satisfies TemplateExecutor.
var _ template.TemplateExecutor = (*mockTemplateExecutor)(nil)

func mustNewPathProcessor(t *testing.T) *DefaultPathProcessor {
	t.Helper()
	engine, err := template.NewEngine()
	require.NoError(t, err)
	return NewPathProcessor(engine)
}

func TestUT_PathProcessor_SimpleVar(t *testing.T) {
	processor := mustNewPathProcessor(t)
	vars := map[string]any{
		"project_name": "my_project",
		"module_name":  "users",
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "single placeholder",
			path:     "{{ vars.project_name }}",
			expected: "my_project",
		},
		{
			name:     "placeholder in path",
			path:     "{{ vars.project_name }}/cmd/main.go",
			expected: "my_project/cmd/main.go",
		},
		{
			name:     "placeholder in filename",
			path:     "internal/{{ vars.module_name }}.go",
			expected: "internal/users.go",
		},
		{
			name:     "multiple placeholders",
			path:     "{{ vars.project_name }}/internal/{{ vars.module_name }}/service.go",
			expected: "my_project/internal/users/service.go",
		},
		{
			name:     "no placeholders",
			path:     "cmd/main.go",
			expected: "cmd/main.go",
		},
		{
			name:     "python __init__.py not a placeholder",
			path:     "{{ vars.project_name }}/src/__init__.py",
			expected: "my_project/src/__init__.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPath(tt.path, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_PathProcessor_WithFilter(t *testing.T) {
	processor := mustNewPathProcessor(t)
	vars := map[string]any{
		"project_name": "MyProject",
		"model":        "user",
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "snake filter",
			path:     "{{ vars.project_name | snake }}",
			expected: "my_project",
		},
		{
			name:     "pascal filter",
			path:     "{{ vars.project_name | pascal }}",
			expected: "MyProject",
		},
		{
			name:     "camel filter",
			path:     "{{ vars.project_name | camel }}",
			expected: "myProject",
		},
		{
			name:     "kebab filter",
			path:     "{{ vars.project_name | kebab }}",
			expected: "my-project",
		},
		{
			name:     "lower filter",
			path:     "{{ vars.project_name | lower }}",
			expected: "myproject",
		},
		{
			name:     "upper filter",
			path:     "{{ vars.project_name | upper }}",
			expected: "MYPROJECT",
		},
		{
			name:     "plural filter",
			path:     "{{ vars.model | plural }}",
			expected: "users",
		},
		{
			name:     "singular filter from plural",
			path:     "users/{{ vars.model | singular }}",
			expected: "users/user",
		},
		{
			name:     "filter without spaces",
			path:     "{{vars.project_name|snake}}",
			expected: "my_project",
		},
		{
			name:     "filter in complex path",
			path:     "{{ vars.project_name | snake }}/internal/{{ vars.model | plural }}/service.go",
			expected: "my_project/internal/users/service.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPath(tt.path, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_PathProcessor_UndefinedVar(t *testing.T) {
	processor := mustNewPathProcessor(t)
	vars := map[string]any{
		"project_name": "my_project",
	}

	// Gonja returns empty string for undefined variables (consistent with content templates)
	result, err := processor.ProcessPath("{{ vars.unknown_var }}/file.go", vars)
	require.NoError(t, err)
	assert.Equal(t, "file.go", result) // Empty segment is skipped
}

func TestUT_PathProcessor_InvalidFilter(t *testing.T) {
	processor := mustNewPathProcessor(t)
	vars := map[string]any{
		"project_name": "my_project",
	}

	_, err := processor.ProcessPath("{{ vars.project_name | invalid_filter }}", vars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filter")
	assert.Contains(t, err.Error(), "invalid_filter")
}

func TestUT_PathProcessor_EmptyValue(t *testing.T) {
	processor := mustNewPathProcessor(t)
	vars := map[string]any{
		"project_name": "",
		"module_name":  "users",
	}

	// Empty value in path segment should collapse that segment
	result, err := processor.ProcessPath("{{ vars.project_name }}/{{ vars.module_name }}/service.go", vars)
	require.NoError(t, err)
	// Note: empty segment is skipped, so result doesn't start with "/"
	assert.Equal(t, "users/service.go", result)
}

func TestUT_PathProcessor_MultiplePlaceholdersInSegment(t *testing.T) {
	processor := mustNewPathProcessor(t)
	vars := map[string]any{
		"prefix": "api",
		"suffix": "v1",
	}

	result, err := processor.ProcessPath("{{ vars.prefix }}-{{ vars.suffix }}/handler.go", vars)
	require.NoError(t, err)
	assert.Equal(t, "api-v1/handler.go", result)
}

func TestUT_PathProcessor_ComplexExpressions(t *testing.T) {
	processor := mustNewPathProcessor(t)
	vars := map[string]any{
		"package_display_name": "My Cool Package",
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "lower method",
			path:     "{{ vars.package_display_name.lower() }}",
			expected: "my cool package",
		},
		{
			name:     "replace filter",
			path:     "{{ vars.package_display_name | replace(' ', '_') }}",
			expected: "My_Cool_Package",
		},
		{
			name:     "chained filters",
			path:     "{{ vars.package_display_name | lower | replace(' ', '_') | replace('-', '_') }}",
			expected: "my_cool_package",
		},
		{
			name:     "upper method",
			path:     "{{ vars.package_display_name.upper() }}",
			expected: "MY COOL PACKAGE",
		},
		{
			name:     "replace method with 2 args (Python style)",
			path:     "{{ vars.package_display_name.replace(' ', '_') }}",
			expected: "My_Cool_Package",
		},
		{
			name:     "chained methods",
			path:     "{{ vars.package_display_name.lower().replace(' ', '_').replace('-', '_') }}",
			expected: "my_cool_package",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPath(tt.path, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_ExtractPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "single placeholder",
			path:     "{{ vars.project_name }}",
			expected: []string{"project_name"},
		},
		{
			name:     "multiple placeholders",
			path:     "{{ vars.project_name }}/{{ vars.module_name }}",
			expected: []string{"project_name", "module_name"},
		},
		{
			name:     "placeholder with filter",
			path:     "{{ vars.project_name | snake }}",
			expected: []string{"project_name"},
		},
		{
			name:     "no placeholders",
			path:     "cmd/main.go",
			expected: []string{},
		},
		{
			name:     "duplicate placeholders",
			path:     "{{ vars.name }}/{{ vars.name }}",
			expected: []string{"name"},
		},
		{
			name:     "python dunder not a placeholder",
			path:     "__init__.py",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPlaceholders(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_HasPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"has placeholder", "{{ vars.name }}", true},
		{"has placeholder with filter", "{{ vars.name | snake }}", true},
		{"vars with filter", "{{ vars.name | snake }}", true},
		{"has conditional block", `{% if vars.feature %}file.go{% endif %}`, true},
		{"no placeholder", "cmd/main.go", false},
		{"python dunder not a placeholder", "__init__.py", false},
		{"python main not a placeholder", "__main__.py", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasPlaceholders(tt.path))
		})
	}
}

func TestUT_PathProcessor_ConditionalFilename(t *testing.T) {
	processor := mustNewPathProcessor(t)

	tests := []struct {
		name     string
		path     string
		vars     map[string]any
		expected string
	}{
		{
			name:     "conditional true",
			path:     `{% if vars.use_http == "yes" %}http.go{% endif %}`,
			vars:     map[string]any{"use_http": "yes"},
			expected: "http.go",
		},
		{
			name:     "conditional false - file excluded",
			path:     `{% if vars.use_http == "yes" %}http.go{% endif %}`,
			vars:     map[string]any{"use_http": "no"},
			expected: "",
		},
		{
			name:     "conditional in subdirectory - true",
			path:     `handlers/{% if vars.use_http == "yes" %}http.go{% endif %}`,
			vars:     map[string]any{"use_http": "yes"},
			expected: "handlers/http.go",
		},
		{
			name:     "conditional in subdirectory - false",
			path:     `handlers/{% if vars.use_http == "yes" %}http.go{% endif %}`,
			vars:     map[string]any{"use_http": "no"},
			expected: "",
		},
		{
			name:     "no-space comparison operator",
			path:     `{% if vars.use_http=="yes" %}http.go{% endif %}`,
			vars:     map[string]any{"use_http": "yes"},
			expected: "http.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPath(tt.path, tt.vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_PathProcessor_NestedTemplates(t *testing.T) {
	// Test derived variables where a variable's value is itself a template expression.
	// These must be marked as derived for the SSTI protection to allow rendering.
	processor := mustNewPathProcessor(t)
	processor.SetDerivedVarNames(map[string]bool{"package_name": true})
	vars := map[string]any{
		"package_display_name": "My Cool Package",
		// package_name's value is a template expression (like Cookiecutter derived variables)
		"package_name": "{{ vars.package_display_name.lower().replace(' ', '_').replace('-', '_') }}",
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple variable with template value",
			path:     "{{ vars.package_name }}",
			expected: "my_cool_package",
		},
		{
			name:     "nested template in path",
			path:     "src/{{ vars.package_name }}/main.py",
			expected: "src/my_cool_package/main.py",
		},
		{
			name:     "multiple nested references",
			path:     "{{ vars.package_name }}/src/{{ vars.package_name }}/__init__.py",
			expected: "my_cool_package/src/my_cool_package/__init__.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPath(tt.path, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_PathProcessor_WhitespaceAroundFilter(t *testing.T) {
	processor := mustNewPathProcessor(t)
	vars := map[string]any{
		"project_name": "MyProject",
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"no spaces", "{{vars.project_name|snake}}", "my_project"},
		{"space before pipe", "{{ vars.project_name |snake }}", "my_project"},
		{"space after pipe", "{{ vars.project_name| snake }}", "my_project"},
		{"spaces both sides", "{{ vars.project_name | snake }}", "my_project"},
		{"multiple spaces", "{{  vars.project_name  |  snake  }}", "my_project"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.ProcessPath(tt.path, vars)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_PathProcessor_WithMockExecutor(t *testing.T) {
	// Verify PathProcessor works against the TemplateExecutor interface, not just *template.Engine.
	mock := &mockTemplateExecutor{
		executeToStringResult: "rendered_value",
	}
	processor := NewPathProcessor(mock)

	result, err := processor.ProcessPath("{{ vars.name }}", map[string]any{"name": "test"})
	require.NoError(t, err)
	assert.Equal(t, "rendered_value", result)
}

func TestUT_PathProcessor_MockExecutorError(t *testing.T) {
	mock := &mockTemplateExecutor{
		executeToStringErr: fmt.Errorf("mock render error"),
	}
	processor := NewPathProcessor(mock)

	_, err := processor.ProcessPath("{{ vars.name }}", map[string]any{"name": "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mock render error")
}

// ============================================================================
// SSTI PROTECTION TESTS (Ticket #65, #71)
//
// These tests verify that user-provided variable values containing template
// syntax are NOT interpreted as template code (SSTI prevention). Derived
// variables (whose defaults are template expressions) are still rendered.
//
// Use --allow-recursive-render to restore the old (insecure) behavior.
// ============================================================================

func TestUT_ProcessSegment_UserInput_NoRecursiveRender(t *testing.T) {
	// SSTI FIX: User-provided variable values containing template syntax
	// are NOT rendered when allowRecursiveRender is false (default).
	processor := mustNewPathProcessor(t)

	vars := map[string]any{
		"project_name": "myproject",
		// User-controlled value that contains template syntax
		"user_input": "{{ vars.project_name }}",
	}

	// With SSTI protection: user_input's template syntax is escaped
	result, err := processor.ProcessPath("{{ vars.user_input }}", vars)
	require.NoError(t, err)
	// The template syntax in user_input should NOT be rendered
	assert.NotEqual(t, "myproject", result,
		"SSTI protection: user-provided template syntax should not be rendered")
}

func TestUT_ProcessSegment_UserInput_NestedTemplates_NoRender(t *testing.T) {
	// SSTI FIX: Multi-level nested template injection should NOT resolve.
	processor := mustNewPathProcessor(t)

	vars := map[string]any{
		"level_c": "final_value",
		"level_b": "{{ vars.level_c }}",
		"level_a": "{{ vars.level_b }}",
	}

	result, err := processor.ProcessPath("{{ vars.level_a }}", vars)
	require.NoError(t, err)
	// Should NOT resolve through the chain
	assert.NotEqual(t, "final_value", result,
		"SSTI protection: nested template injection should not resolve")
}

func TestUT_ProcessSegment_AllowRecursiveRender_UserInput(t *testing.T) {
	// When allowRecursiveRender is true, old (insecure) behavior is restored.
	processor := mustNewPathProcessor(t)
	processor.SetAllowRecursiveRender(true)

	vars := map[string]any{
		"project_name": "myproject",
		"user_input":   "{{ vars.project_name }}",
	}

	result, err := processor.ProcessPath("{{ vars.user_input }}", vars)
	require.NoError(t, err)
	assert.Equal(t, "myproject", result,
		"with --allow-recursive-render, template syntax in values IS rendered")
}

func TestUT_ProcessSegment_DerivedVariables_Render(t *testing.T) {
	// Derived variables (template expressions as defaults) should still resolve
	// even when allowRecursiveRender is false, because they are marked as derived.
	processor := mustNewPathProcessor(t)
	processor.SetDerivedVarNames(map[string]bool{"package_name": true})

	vars := map[string]any{
		"package_display_name": "My Cool Package",
		"package_name":         "{{ vars.package_display_name.lower().replace(' ', '_').replace('-', '_') }}",
	}

	result, err := processor.ProcessPath("{{ vars.package_name }}", vars)
	require.NoError(t, err)
	assert.Equal(t, "my_cool_package", result,
		"derived variables should still be rendered through recursive rendering")
}

func TestUT_ProcessSegment_DerivedVariables_Nested_Render(t *testing.T) {
	// Derived variables that reference other derived variables should resolve.
	processor := mustNewPathProcessor(t)
	processor.SetDerivedVarNames(map[string]bool{"derived_a": true, "derived_b": true})

	vars := map[string]any{
		"base_name": "MyProject",
		"derived_b": "{{ vars.base_name | lower }}",
		"derived_a": "{{ vars.derived_b }}",
	}

	result, err := processor.ProcessPath("{{ vars.derived_a }}", vars)
	require.NoError(t, err)
	assert.Equal(t, "myproject", result,
		"nested derived variables should resolve through recursive rendering")
}

func TestUT_ProcessSegment_RecursionLimit(t *testing.T) {
	// Verify the recursion limit (maxRenderIterations = 5) prevents infinite loops
	// when allowRecursiveRender is true.
	processor := mustNewPathProcessor(t)
	processor.SetAllowRecursiveRender(true)

	// Build a chain: v1 → v2 → v3 → v4 → v5 → v6 → "done"
	// With 5 iterations max, the chain should stop before fully resolving.
	vars := map[string]any{
		"v6": "done",
		"v5": "{{ vars.v6 }}",
		"v4": "{{ vars.v5 }}",
		"v3": "{{ vars.v4 }}",
		"v2": "{{ vars.v3 }}",
		"v1": "{{ vars.v2 }}",
	}

	result, err := processor.ProcessPath("{{ vars.v1 }}", vars)
	require.NoError(t, err)

	assert.NotEqual(t, "done", result,
		"recursion limit should prevent full resolution of a 6-level chain")
	assert.Contains(t, result, "v6",
		"result should still contain unresolved reference to v6")

	// Also verify a 5-level chain DOES fully resolve (proving the limit is exactly 5)
	vars5 := map[string]any{
		"v5": "resolved",
		"v4": "{{ vars.v5 }}",
		"v3": "{{ vars.v4 }}",
		"v2": "{{ vars.v3 }}",
		"v1": "{{ vars.v2 }}",
	}
	result5, err := processor.ProcessPath("{{ vars.v1 }}", vars5)
	require.NoError(t, err)
	assert.Equal(t, "resolved", result5,
		"a 5-level chain should fully resolve within the recursion limit")
}

func TestUT_ProcessSegment_CyclicReference(t *testing.T) {
	// Verify that cyclic references (A→B, B→A) don't cause infinite loops
	// when allowRecursiveRender is true.
	processor := mustNewPathProcessor(t)
	processor.SetAllowRecursiveRender(true)

	vars := map[string]any{
		"a": "{{ vars.b }}",
		"b": "{{ vars.a }}",
	}

	result, err := processor.ProcessPath("{{ vars.a }}", vars)
	require.NoError(t, err)
	// Cyclic references should terminate without panic or infinite loop.
	assert.NotEmpty(t, result, "cyclic reference should produce some output")
	t.Logf("Cyclic reference test result: %q", result)
}

func TestUT_ProcessSegment_MaliciousValue_Escaped(t *testing.T) {
	// SSTI FIX: Template syntax in non-derived variable values is escaped.
	processor := mustNewPathProcessor(t)

	vars := map[string]any{
		"safe_name": "hello",
		// Attacker tries to reference another variable through injection
		"malicious": "{{ vars.safe_name }}_injected",
	}

	result, err := processor.ProcessPath("{{ vars.malicious }}", vars)
	require.NoError(t, err)
	// Template expressions in malicious should NOT be rendered
	assert.NotEqual(t, "hello_injected", result,
		"SSTI protection: template expressions in variable values should not be rendered")
}

func TestUT_ProcessSegment_InvalidTemplateSyntax_InValue(t *testing.T) {
	// Values with malformed template syntax (e.g., {{{ or { { ) should remain literal.
	processor := mustNewPathProcessor(t)

	vars := map[string]any{
		"weird": "{{{ not valid template",
	}

	result, err := processor.ProcessPath("{{ vars.weird }}", vars)
	require.NoError(t, err)
	assert.NotEmpty(t, result, "malformed template syntax should not cause errors")
}

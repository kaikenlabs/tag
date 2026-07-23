package vars

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTemplate creates a temporary template directory with a tag.template.json
// and optional template files. Returns the root path.
func setupTemplate(t *testing.T, configJSON string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()

	require.NoError(t, os.WriteFile(
		filepath.Join(root, "tag.template.json"),
		[]byte(configJSON),
		0o644,
	))

	for path, content := range files {
		absPath := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o755))
		require.NoError(t, os.WriteFile(absPath, []byte(content), 0o644))
	}

	return root
}

func TestUT_AnalyzeAllDeclaredUsed(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp", "author": {"type": "string", "prompt": "Author?"}}}`,
		map[string]string{
			"README.md": "# {{ vars.project_name }}\nBy {{ vars.author }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	assert.Empty(t, report.Root.Undeclared)
	assert.Empty(t, report.Root.Unused)
	assert.Len(t, report.Root.Declared, 2)

	// Check reference counts.
	for _, dv := range report.Root.Declared {
		assert.Greater(t, dv.ReferenceCount, 0, "expected references for %s", dv.Name)
		assert.Greater(t, dv.FileCount, 0, "expected file count for %s", dv.Name)
	}
}

func TestUT_AnalyzeUndeclaredVariable(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp"}}`,
		map[string]string{
			"README.md":  "# {{ vars.project_name }}",
			"LICENSE.md": "Licensed to {{ vars.license }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Root.Undeclared, 1)
	assert.Equal(t, "license", report.Root.Undeclared[0].Name)
	assert.Len(t, report.Root.Undeclared[0].References, 1)
	assert.Equal(t, "LICENSE.md", report.Root.Undeclared[0].References[0].File)
}

func TestUT_AnalyzeUnusedVariable(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp", "unused_var": {"type": "string", "prompt": "Not used"}}}`,
		map[string]string{
			"README.md": "# {{ vars.project_name }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Root.Unused, 1)
	assert.Equal(t, "unused_var", report.Root.Unused[0])
}

func TestUT_AnalyzeRawBlockNotCounted(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp", "pod": {"type": "string", "prompt": "Pod"}}}`,
		map[string]string{
			"grafana.json": "{% raw %}legendFormat: {{ vars.pod }}{% endraw %}\n" +
				"{% raw %}{{ vars.undeclared_in_raw }}\nspanning two lines{% endraw %}\n" +
				"# {{ vars.project_name }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	// A raw-block mention is literal output, not a reference: it neither declares
	// usage nor introduces an undeclared variable.
	assert.Equal(t, []string{"pod"}, report.Root.Unused)
	assert.Empty(t, report.Root.Undeclared)

	// Masking is newline-preserving, so the one real reference keeps its line.
	var refs []Reference
	for _, dv := range report.Root.Declared {
		if dv.Name == "project_name" {
			refs = dv.References
		}
	}
	require.Len(t, refs, 1)
	assert.Equal(t, 4, refs[0].Line)
}

func TestUT_AnalyzeStringLiteralNotCounted(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp", "ghost": {"type": "string", "default": "x"}}}`,
		map[string]string{
			// defect 1: "ghost" only appears inside a string literal argument.
			"README.md": `{{ replace("{{ vars.ghost }}") }}` + "\n# {{ vars.project_name }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	for _, dv := range report.Root.Declared {
		if dv.Name == "ghost" {
			assert.Equal(t, 0, dv.ReferenceCount, "a mention inside a string literal is not a reference")
			assert.Equal(t, 0, dv.FileCount)
		}
		if dv.Name == "project_name" {
			assert.Positive(t, dv.ReferenceCount)
		}
	}
}

func TestUT_AnalyzeVarOnlyInStringLiteralIsUnused(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"ghost": {"type": "string", "default": "x"}}}`,
		map[string]string{
			"README.md": `{{ replace("{{ vars.ghost }}") }}`,
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	// A variable mentioned only inside a string literal is never referenced,
	// so it is reported unused — a deliberate behaviour change from the old
	// line-oriented regex scanner, which mistook the literal for a use.
	assert.Equal(t, []string{"ghost"}, report.Root.Unused)
}

func TestUT_AnalyzeReferenceExpressionIsOriginalLine(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"real": {"type": "string", "prompt": "Real"}}}`,
		map[string]string{
			// The literal delimiter "}}"  inside the string must not truncate
			// the block, and the reported Expression must be the ORIGINAL
			// source line, not a masked/blanked copy of it.
			"README.md": "line one\n" + `{{ f("}}") ~ vars.real }}`,
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	var refs []Reference
	for _, dv := range report.Root.Declared {
		if dv.Name == "real" {
			refs = dv.References
		}
	}
	require.Len(t, refs, 1)
	assert.Equal(t, 2, refs[0].Line)
	assert.Equal(t, `{{ f("}}") ~ vars.real }}`, refs[0].Expression)
}

// TestUT_AnalyzeReferenceExpressionStripsCarriageReturn covers the CRLF path
// through scanFileContent: ScanRefs documents itself as CRLF-safe, so a
// reference in a CRLF-terminated file must report the right line AND an
// Expression with no trailing \r left on it.
func TestUT_AnalyzeReferenceExpressionStripsCarriageReturn(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"real": {"type": "string", "prompt": "Real"}}}`,
		map[string]string{
			"README.md": "line one\r\n{{ vars.real }}\r\nline three\r\n",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	var refs []Reference
	for _, dv := range report.Root.Declared {
		if dv.Name == "real" {
			refs = dv.References
		}
	}
	require.Len(t, refs, 1)
	assert.Equal(t, 2, refs[0].Line)
	assert.Equal(t, "{{ vars.real }}", refs[0].Expression)
	assert.NotContains(t, refs[0].Expression, "\r")
}

func TestUT_AnalyzeDerivedVariable(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp", "_slug": {"type": "string", "default": "{{ vars.project_name | lower }}"}}}`,
		map[string]string{
			"README.md": "# {{ vars.project_name }}\nSlug: {{ vars._slug }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	assert.Empty(t, report.Root.Undeclared)
	assert.Empty(t, report.Root.Unused)

	// Find the _slug variable and verify it's marked as derived.
	var found bool
	for _, dv := range report.Root.Declared {
		if dv.Name == "_slug" {
			found = true
			assert.True(t, dv.Derived)
			assert.True(t, dv.Private)
		}
	}
	assert.True(t, found, "_slug not found in declared vars")
}

func TestUT_AnalyzePathPlaceholders(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp"}}`,
		map[string]string{
			"{{ vars.project_name }}/main.go": "package main",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	assert.Empty(t, report.Root.Undeclared)

	// project_name should have a path reference.
	for _, dv := range report.Root.Declared {
		if dv.Name == "project_name" {
			assert.Greater(t, dv.ReferenceCount, 0)
		}
	}
}

func TestUT_AnalyzeConditionals(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"use_docker": true}}`,
		map[string]string{
			"setup.sh": "#!/bin/bash\n{% if vars.use_docker %}docker build .{% endif %}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	assert.Empty(t, report.Root.Undeclared)
	assert.Empty(t, report.Root.Unused)
}

func TestUT_AnalyzeGeneratorScope(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp"}}`,
		map[string]string{
			"README.md":                              "# {{ vars.project_name }}",
			"_generators/endpoint/tag.template.json": `{"vars": {"endpoint_name": {"type": "string", "prompt": "Name?"}}}`,
			"_generators/endpoint/handler.go":        "// {{ vars.endpoint_name }} handler for {{ vars.project_name }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	assert.Empty(t, report.Root.Undeclared)
	assert.Empty(t, report.Root.Unused)

	require.Len(t, report.Generators, 1)
	gen := report.Generators[0]
	assert.Equal(t, "_generators/endpoint", gen.Scope)
	assert.Empty(t, gen.Undeclared)
	assert.Empty(t, gen.Unused)
	require.Len(t, gen.Declared, 1)
	assert.Equal(t, "endpoint_name", gen.Declared[0].Name)
}

func TestUT_AnalyzeRootVarsInGenerator(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp"}}`,
		map[string]string{
			"README.md":                              "# {{ vars.project_name }}",
			"_generators/endpoint/tag.template.json": `{"vars": {}}`,
			"_generators/endpoint/handler.go":        "// part of {{ vars.project_name }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	// Root var used in generator should not be "undeclared" in generator scope.
	require.Len(t, report.Generators, 1)
	assert.Empty(t, report.Generators[0].Undeclared)
}

func TestUT_AnalyzeBinaryFileSkipped(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp"}}`,
		map[string]string{
			"README.md": "# {{ vars.project_name }}",
		},
	)

	// Write a binary file containing "vars.secret".
	binaryContent := []byte{0x00, 0x01, 0x02}
	binaryContent = append(binaryContent, []byte("vars.secret")...)
	require.NoError(t, os.WriteFile(filepath.Join(root, "image.png"), binaryContent, 0o644))

	report, err := Analyze(root)
	require.NoError(t, err)
	// "secret" should not appear as undeclared.
	for _, uv := range report.Root.Undeclared {
		assert.NotEqual(t, "secret", uv.Name)
	}
}

func TestUT_AnalyzeTagignoreRespected(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp"}}`,
		map[string]string{
			"README.md":  "# {{ vars.project_name }}",
			"ignored.md": "{{ vars.should_be_ignored }}",
			".tagignore": "ignored.md",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	// should_be_ignored must not appear.
	for _, uv := range report.Root.Undeclared {
		assert.NotEqual(t, "should_be_ignored", uv.Name)
	}
}

func TestUT_AnalyzeMissingConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	_, err := Analyze(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag.template.json")
}

func TestUT_AnalyzeEmptyTemplate(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t, `{"vars": {}}`, nil)

	report, err := Analyze(root)
	require.NoError(t, err)
	assert.Empty(t, report.Root.Declared)
	assert.Empty(t, report.Root.Undeclared)
	assert.Empty(t, report.Root.Unused)
}

func TestUT_AnalyzeDerivedDefaultReferences(t *testing.T) {
	t.Parallel()

	// _slug references project_name via its default expression.
	// Both should count as used even if _slug is only used in files.
	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp", "_slug": {"type": "string", "default": "{{ vars.project_name | lower }}"}}}`,
		map[string]string{
			"README.md": "Slug: {{ vars._slug }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	assert.Empty(t, report.Root.Undeclared)
	assert.Empty(t, report.Root.Unused, "project_name should count as used via derived default")
}

func TestUT_AnalyzeMultipleReferencesAcrossFiles(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp"}}`,
		map[string]string{
			"README.md":  "# {{ vars.project_name }}",
			"go.mod":     "module {{ vars.project_name }}",
			"main.go":    "// {{ vars.project_name }}",
			"Dockerfile": "LABEL name={{ vars.project_name }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	for _, dv := range report.Root.Declared {
		if dv.Name == "project_name" {
			assert.Equal(t, 4, dv.FileCount)
			assert.Equal(t, 4, dv.ReferenceCount)
		}
	}
}

func TestUT_AnalyzeGeneratorWithoutConfig(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp"}}`,
		map[string]string{
			"README.md":                      "# {{ vars.project_name }}",
			"_generators/simple/template.go": "// {{ vars.project_name }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	// Generator without its own config should still work.
	require.Len(t, report.Generators, 1)
	assert.Empty(t, report.Generators[0].Undeclared)
	assert.Empty(t, report.Generators[0].Declared) // No generator-level vars.
}

func TestUT_AnalyzeCommentsStripped(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"project_name": "myapp"}}`,
		map[string]string{
			"README.md": "{# {{ vars.commented_out }} #}\n# {{ vars.project_name }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	// commented_out should not appear as undeclared.
	for _, uv := range report.Root.Undeclared {
		assert.NotEqual(t, "commented_out", uv.Name)
	}
}

func TestUT_AnalyzeDeterministicOrdering(t *testing.T) {
	t.Parallel()

	root := setupTemplate(t,
		`{"vars": {"zebra": "z", "alpha": "a", "middle": "m"}}`,
		map[string]string{
			"a.txt": "{{ vars.alpha }}",
			"z.txt": "{{ vars.zebra }}",
			"m.txt": "{{ vars.middle }}",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	// Declared should be sorted alphabetically.
	require.Len(t, report.Root.Declared, 3)
	assert.Equal(t, "alpha", report.Root.Declared[0].Name)
	assert.Equal(t, "middle", report.Root.Declared[1].Name)
	assert.Equal(t, "zebra", report.Root.Declared[2].Name)
}

func TestUT_ExtractVarNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "expression",
			input:    "{{ vars.project_name }}",
			expected: []string{"project_name"},
		},
		{
			name:     "expression with filter",
			input:    "{{ vars.name | upper }}",
			expected: []string{"name"},
		},
		{
			name:     "statement if",
			input:    "{% if vars.use_docker %}yes{% endif %}",
			expected: []string{"use_docker"},
		},
		{
			name:     "statement for",
			input:    "{% for item in vars.items %}{{ item }}{% endfor %}",
			expected: []string{"items"},
		},
		{
			name:     "multiple in one line",
			input:    "{{ vars.first }} and {{ vars.second }}",
			expected: []string{"first", "second"},
		},
		{
			name:     "no match",
			input:    "hello world",
			expected: nil,
		},
		{
			name:     "plain text with vars word",
			input:    "these are variables",
			expected: nil,
		},
		{
			name:     "deduplication",
			input:    "{{ vars.x }} {{ vars.x }}",
			expected: []string{"x"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := extractVarNames(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestUT_HasIssues(t *testing.T) {
	t.Parallel()

	t.Run("no issues", func(t *testing.T) {
		t.Parallel()
		r := &Report{Root: ScopeResult{}}
		assert.False(t, r.HasIssues())
	})

	t.Run("root undeclared", func(t *testing.T) {
		t.Parallel()
		r := &Report{Root: ScopeResult{
			Undeclared: []UndeclaredVar{{Name: "x"}},
		}}
		assert.True(t, r.HasIssues())
	})

	t.Run("root unused", func(t *testing.T) {
		t.Parallel()
		r := &Report{Root: ScopeResult{
			Unused: []string{"x"},
		}}
		assert.True(t, r.HasIssues())
	})

	t.Run("generator issues", func(t *testing.T) {
		t.Parallel()
		r := &Report{
			Root: ScopeResult{},
			Generators: []ScopeResult{
				{Undeclared: []UndeclaredVar{{Name: "y"}}},
			},
		}
		assert.True(t, r.HasIssues())
	})
}

func TestUT_WriteJSON(t *testing.T) {
	t.Parallel()

	report := &Report{
		Root: ScopeResult{
			Scope:   "root",
			Summary: Summary{Declared: 1},
			Declared: []DeclaredVar{
				{Name: "x", Type: "string", References: []Reference{}},
			},
		},
	}

	var buf strings.Builder
	err := WriteJSON(&buf, report)
	require.NoError(t, err)

	// Parse back and verify structure.
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &parsed))

	root, ok := parsed["root"].(map[string]any)
	require.True(t, ok)

	// Verify empty slices are [] not null.
	decl, ok := root["declared"].([]any)
	require.True(t, ok)
	require.Len(t, decl, 1)

	undecl, ok := root["undeclared"].([]any)
	require.True(t, ok)
	assert.Empty(t, undecl)

	unused, ok := root["unused"].([]any)
	require.True(t, ok)
	assert.Empty(t, unused)

	// Generators should be empty array.
	gens, ok := parsed["generators"].([]any)
	require.True(t, ok)
	assert.Empty(t, gens)
}

func TestUT_WriteText(t *testing.T) {
	t.Parallel()

	report := &Report{
		Root: ScopeResult{
			Scope: "root",
			Declared: []DeclaredVar{
				{
					Name:           "project_name",
					Type:           "string",
					Required:       true,
					FileCount:      2,
					ReferenceCount: 3,
					References:     []Reference{},
				},
			},
			Undeclared: []UndeclaredVar{
				{
					Name: "license",
					References: []Reference{
						{File: "LICENSE.md", Line: 1},
					},
				},
			},
			Unused: []string{"go_module"},
			Summary: Summary{
				Declared:   1,
				Undeclared: 1,
				Unused:     1,
			},
		},
	}

	var buf strings.Builder
	WriteText(&buf, report)
	output := buf.String()

	assert.Contains(t, output, "project_name")
	assert.Contains(t, output, "string, required")
	assert.Contains(t, output, "used in 2 file(s)")
	assert.Contains(t, output, "vars.license")
	assert.Contains(t, output, "LICENSE.md:1")
	assert.Contains(t, output, "go_module")
	assert.Contains(t, output, "declared but not referenced")
	assert.Contains(t, output, "1 declared, 1 undeclared, 1 unused")
}

package lint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helper ---

// createTemplate creates a minimal template directory for testing.
func createTemplate(t *testing.T, dir, config string, files map[string]string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(config), 0o644))
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

const validConfig = `{
  "name": "test-template",
  "description": "A test template",
  "vars": {
    "project_name": "my-project",
    "use_docker": {
      "type": "boolean",
      "prompt": "Use Docker?",
      "default": false
    },
    "_private_var": "internal",
    "derived_var": "{{ vars.project_name | snake }}"
  }
}`

// --- Schema Validation Tests ---

func TestUT_LintSchema_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, nil)

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.False(t, result.HasErrors())
}

func TestUT_LintSchema_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	// Invalid: vars must be an object, not an array
	config := `{"name": "test", "vars": [1, 2, 3]}`
	createTemplate(t, dir, config, nil)

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.True(t, result.HasErrors())

	hasSchemaRule := false
	for _, issue := range result.Issues {
		if issue.Rule == "schema-validation" || issue.Rule == "config-parse" {
			hasSchemaRule = true
			break
		}
	}
	assert.True(t, hasSchemaRule, "expected schema-validation or config-parse rule in issues")
}

func TestUT_LintSchema_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(`{not json`), 0o644))

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.True(t, result.HasErrors())
}

func TestUT_LintSchema_MissingConfig(t *testing.T) {
	dir := t.TempDir()

	_, err := NewLinter(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag.template.json not found")
}

// --- Template Syntax Tests ---

func TestUT_LintTemplateFiles_ValidSyntax(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"README.md": "# {{ vars.project_name }}\nHello world",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.False(t, result.HasErrors())
}

func TestUT_LintTemplateFiles_InvalidSyntax(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"broken.txt": "Hello {{ vars.project_name }\nWorld",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.True(t, result.HasErrors())

	hasSyntaxRule := false
	for _, issue := range result.Issues {
		if issue.Rule == "template-syntax" {
			hasSyntaxRule = true
			assert.Equal(t, "broken.txt", issue.File)
			break
		}
	}
	assert.True(t, hasSyntaxRule, "expected template-syntax rule in issues")
}

func TestUT_LintTemplateFiles_BinarySkipped(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, nil)
	// Write a binary file with null bytes
	require.NoError(t, os.WriteFile(filepath.Join(dir, "image.png"), []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00}, 0o644))

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.False(t, result.HasErrors())
}

func TestUT_LintTemplateFiles_TagignoreRespected(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		".tagignore":      "ignored/",
		"ignored/bad.txt": "{{ vars.undefined_var }}",
		"good.txt":        "{{ vars.project_name }}",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	// The ignored file with undefined var should not cause an error
	assert.False(t, result.HasErrors())
}

func TestUT_LintTemplateFiles_SkipsConfigFiles(t *testing.T) {
	dir := t.TempDir()
	// tag.template.json itself uses valid Gonja syntax in the JSON, but we should
	// NOT lint it as a template file. If we did, the JSON syntax would fail Gonja parsing.
	createTemplate(t, dir, validConfig, map[string]string{
		"hello.txt": "{{ vars.project_name }}",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	// If tag.template.json were linted as template, it would produce syntax errors
	for _, issue := range result.Issues {
		assert.NotEqual(t, "tag.template.json", issue.File, "should not lint config file as template")
	}
}

// Variable extraction itself (block walking, quote-awareness, comment/raw
// skipping) is exercised exhaustively in internal/vars/scan_test.go, which
// lintVariableRefs and lintDerivedDefaults/lintPath delegate to directly.

// --- Cross-Reference Tests ---

func TestUT_CrossReference_AllDefined(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"readme.md": "# {{ vars.project_name }}\n{% if vars.use_docker %}Docker{% endif %}",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.False(t, result.HasErrors())
}

func TestUT_CrossReference_UndefinedVar(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"readme.md": "{{ vars.nonexistent }}",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.True(t, result.HasErrors())

	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" && issue.File == "readme.md" {
			found = true
			assert.Equal(t, 1, issue.Line)
			assert.Contains(t, issue.Message, "nonexistent")
			break
		}
	}
	assert.True(t, found, "expected undefined-variable issue for readme.md")
}

func TestUT_CrossReference_PrivateVar(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"test.txt": "{{ vars._private_var }}",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	// _private_var is declared, so no undefined-variable errors
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" {
			t.Errorf("unexpected undefined-variable issue: %s", issue.Message)
		}
	}
}

func TestUT_CrossReference_DerivedVar(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"test.txt": "{{ vars.derived_var }}",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	// derived_var is declared, so no undefined-variable errors
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" && issue.File == "test.txt" {
			t.Errorf("unexpected undefined-variable issue: %s", issue.Message)
		}
	}
}

func TestUT_CrossReference_DerivedVarDefault_UndefinedRef(t *testing.T) {
	dir := t.TempDir()
	config := `{
  "name": "test",
  "vars": {
    "bad_derived": "{{ vars.nonexistent | snake }}"
  }
}`
	createTemplate(t, dir, config, nil)

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.True(t, result.HasErrors())

	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" && issue.File == "tag.template.json" {
			found = true
			assert.Contains(t, issue.Message, "nonexistent")
			break
		}
	}
	assert.True(t, found, "expected undefined-variable issue for derived var default")
}

func TestUT_CrossReference_PathPlaceholders(t *testing.T) {
	dir := t.TempDir()
	config := `{
  "name": "test",
  "vars": {
    "name": "myproject"
  }
}`
	createTemplate(t, dir, config, map[string]string{
		"{{ vars.unknown_path }}/file.txt": "content",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.True(t, result.HasErrors())

	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" {
			found = true
			assert.Contains(t, issue.Message, "unknown_path")
			break
		}
	}
	assert.True(t, found, "expected undefined-variable issue for path placeholder")
}

func TestUT_CrossReference_VarInComment_Ignored(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"test.txt": "{# {{ vars.nonexistent }} #}\n{{ vars.project_name }}",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" {
			t.Errorf("unexpected undefined-variable issue: %s", issue.Message)
		}
	}
}

func TestUT_CrossReference_MultilineComment_Ignored(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"test.txt": "{# This comment\nspans {{ vars.nonexistent }}\nmultiple lines #}\n{{ vars.project_name }}",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	// The var in the comment should be ignored
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" {
			t.Errorf("unexpected undefined-variable issue: %s", issue.Message)
		}
	}
}

func TestUT_CrossReference_VarInRawBlock_Ignored(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"test.txt": "{% raw %}legendFormat: {{ vars.pod }}\nspans {{ vars.also_undefined }}\nlines{% endraw %}\n{{ vars.nonexistent }}",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)

	var found []Issue
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" {
			found = append(found, issue)
		}
	}

	// Only the reference outside the raw block is a real reference, and masking
	// the raw body must not shift the line it is reported on.
	require.Len(t, found, 1, "issues: %+v", found)
	assert.Contains(t, found[0].Message, "nonexistent")
	assert.Equal(t, 4, found[0].Line)
}

func TestUT_CrossReference_VarInStringLiteral_Ignored(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		// defect 1: "ghost" only ever appears inside a string literal, so it
		// must not be reported as an undefined reference.
		"test.txt": `{{ replace("{{ vars.ghost }}") }}`,
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" {
			t.Errorf("unexpected undefined-variable issue: %s", issue.Message)
		}
	}
}

// TestUT_CrossReference_RepeatedUndefinedVarOnOneLine_ReportedOnce pins that a
// name repeated on a single line produces ONE issue, not one per occurrence.
// The scanner deliberately reports every occurrence — `tag template variables`
// needs them all for its reference counts — so the linter is what collapses
// them, otherwise a repeated name emits the identical message twice at the same
// file:line and inflates the error count that drives the exit code.
func TestUT_CrossReference_RepeatedUndefinedVarOnOneLine_ReportedOnce(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"same_line.txt": `{{ vars.ghost }} and {{ vars.ghost }}`,
		"two_lines.txt": "{{ vars.spook }}\n{{ vars.spook }}\n",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)

	var sameLine, twoLines int
	for _, issue := range result.Issues {
		if issue.Rule != "undefined-variable" {
			continue
		}
		switch issue.File {
		case "same_line.txt":
			sameLine++
		case "two_lines.txt":
			twoLines++
		}
	}

	assert.Equal(t, 1, sameLine, "one issue per distinct name per line")
	// Distinct lines remain distinct findings: each points at a real location.
	assert.Equal(t, 2, twoLines, "one issue per line the name appears on")
}

func TestUT_CrossReference_DelimiterInStringLiteral_StillReported(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"test.txt": "line one\n" + `{{ f("}}") ~ vars.nonexistent }}`,
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)

	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" {
			found = true
			assert.Contains(t, issue.Message, "nonexistent")
			assert.Equal(t, 2, issue.Line,
				"a closing delimiter inside a literal must not truncate the block early")
		}
	}
	assert.True(t, found, "expected undefined-variable issue for vars.nonexistent")
}

func TestUT_CrossReference_DerivedDefaultStringLiteral_Ignored(t *testing.T) {
	dir := t.TempDir()
	config := `{
  "name": "test",
  "vars": {
    "bad_derived": "{{ replace(\"vars.ghost\") }}"
  }
}`
	createTemplate(t, dir, config, nil)

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" {
			t.Errorf("unexpected undefined-variable issue: %s", issue.Message)
		}
	}
}

// --- Multiple Errors Tests ---

func TestUT_MultipleErrorsReported(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, map[string]string{
		"file1.txt": "{{ vars.undefined_a }}",
		"file2.txt": "{{ vars.undefined_b }}",
	})

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)

	undefinedCount := 0
	for _, issue := range result.Issues {
		if issue.Rule == "undefined-variable" {
			undefinedCount++
		}
	}
	assert.GreaterOrEqual(t, undefinedCount, 2, "should report errors in multiple files")
}

// --- Result Tests ---

func TestUT_Result_HasErrors(t *testing.T) {
	r := &Result{}
	assert.False(t, r.HasErrors())

	r.Add(Issue{Severity: SeverityError, Message: "test"})
	assert.True(t, r.HasErrors())
}

func TestUT_Result_HasErrors_WarningsOnly(t *testing.T) {
	r := &Result{}
	r.Add(Issue{Severity: SeverityWarning, Message: "just a warning"})
	assert.False(t, r.HasErrors())
}

func TestUT_Result_Counts(t *testing.T) {
	r := &Result{}
	r.Add(Issue{Severity: SeverityError, Message: "e1"})
	r.Add(Issue{Severity: SeverityError, Message: "e2"})
	r.Add(Issue{Severity: SeverityWarning, Message: "w1"})
	assert.Equal(t, 2, r.ErrorCount())
	assert.Equal(t, 1, r.WarningCount())
}

// --- NewLinter Tests ---

func TestUT_NewLinter_NonexistentPath(t *testing.T) {
	_, err := NewLinter("/nonexistent/path")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestUT_NewLinter_FileNotDir(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("test"), 0o644))

	_, err := NewLinter(f)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

// --- Format Tests ---

func TestUT_WriteJSON_ValidOutput(t *testing.T) {
	r := &Result{}
	r.Add(Issue{File: "test.txt", Line: 5, Severity: SeverityError, Message: "bad", Rule: "test-rule"})

	var buf strings.Builder
	err := WriteJSON(&buf, r)
	require.NoError(t, err)

	var decoded Result
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &decoded))
	require.Len(t, decoded.Issues, 1)
	assert.Equal(t, "test.txt", decoded.Issues[0].File)
	assert.Equal(t, 5, decoded.Issues[0].Line)
}

func TestUT_WriteJSON_EmptyResult(t *testing.T) {
	r := &Result{}

	var buf strings.Builder
	err := WriteJSON(&buf, r)
	require.NoError(t, err)

	var decoded Result
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &decoded))
	assert.NotNil(t, decoded.Issues, "should be [] not null")
	assert.Empty(t, decoded.Issues)
}

func TestUT_WriteText_Output(t *testing.T) {
	r := &Result{}
	r.Add(Issue{File: "test.txt", Line: 5, Severity: SeverityError, Message: "bad var", Rule: "test-rule"})

	var buf strings.Builder
	WriteText(&buf, r)

	output := buf.String()
	assert.Contains(t, output, "test.txt:5")
	assert.Contains(t, output, "ERROR")
	assert.Contains(t, output, "bad var")
	assert.Contains(t, output, "test-rule")
	assert.Contains(t, output, "1 error(s)")
}

// --- Reserved Generator Name Tests ---

func TestUT_LintGeneratorNames_ReservedName(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, nil)

	// Create a generator directory with a reserved name
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "_generators", "list"), 0o755))

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.True(t, result.HasErrors())

	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "reserved-name" {
			found = true
			assert.Contains(t, issue.Message, "list")
			assert.Equal(t, filepath.Join("_generators", "list"), issue.File)
			break
		}
	}
	assert.True(t, found, "expected reserved-name issue for generator named 'list'")
}

func TestUT_LintGeneratorNames_ValidName(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, nil)

	// Create a generator directory with a valid name
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "_generators", "my-model"), 0o755))

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)

	for _, issue := range result.Issues {
		if issue.Rule == "reserved-name" {
			t.Errorf("unexpected reserved-name issue: %s", issue.Message)
		}
	}
}

func TestUT_LintGeneratorNames_NoGeneratorsDir(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, nil)

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)

	for _, issue := range result.Issues {
		if issue.Rule == "reserved-name" {
			t.Errorf("unexpected reserved-name issue: %s", issue.Message)
		}
	}
}

func TestUT_LintBundleNames_ReservedName(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, nil)

	// Create a bundle with a reserved name
	bundleDir := filepath.Join(dir, "_generators", "_bundles", "mybundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	bundleJSON := `{"name": "list", "generators": [{"name": "model"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "mybundle.json"), []byte(bundleJSON), 0o644))

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.True(t, result.HasErrors())

	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "reserved-name" && strings.Contains(issue.Message, "bundle name") {
			found = true
			assert.Contains(t, issue.Message, "list")
			break
		}
	}
	assert.True(t, found, "expected reserved-name issue for bundle named 'list'")
}

func TestUT_LintBundleNames_ReservedGeneratorRef(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, nil)

	// Create a bundle with a generator ref that has a reserved name
	bundleDir := filepath.Join(dir, "_generators", "_bundles", "mybundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	bundleJSON := `{"name": "mybundle", "generators": [{"name": "info"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "mybundle.json"), []byte(bundleJSON), 0o644))

	linter, err := NewLinter(dir)
	require.NoError(t, err)

	result, err := linter.Run()
	require.NoError(t, err)
	assert.True(t, result.HasErrors())

	found := false
	for _, issue := range result.Issues {
		if issue.Rule == "reserved-name" && strings.Contains(issue.Message, "generator reference") {
			found = true
			assert.Contains(t, issue.Message, "info")
			break
		}
	}
	assert.True(t, found, "expected reserved-name issue for generator ref named 'info'")
}

// --- Empty Generator Directory Tests ---

func TestUT_LintGeneratorDirs_EmptyGenerator(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		subdirs []string
		want    bool
	}{
		{name: "empty directory", want: true},
		{
			name:  "config only",
			files: map[string]string{"tag.template.json": `{"to": "x.go"}`},
			want:  true,
		},
		{
			name:    "subdirectories only",
			subdirs: []string{"nested"},
			want:    true,
		},
		{
			name:  "populated generator",
			files: map[string]string{"model.tmpl": "package {{ name }}\n"},
			want:  false,
		},
		{
			name: "config plus template file",
			files: map[string]string{
				"tag.template.json": `{"to": "x.go"}`,
				"model.tmpl":        "package {{ name }}\n",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			createTemplate(t, dir, validConfig, nil)
			genDir := filepath.Join(dir, "_generators", "mygen")
			require.NoError(t, os.MkdirAll(genDir, 0o755))
			for _, sub := range tt.subdirs {
				require.NoError(t, os.MkdirAll(filepath.Join(genDir, sub), 0o755))
			}
			for name, content := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(genDir, name), []byte(content), 0o644))
			}

			linter, err := NewLinter(dir)
			require.NoError(t, err)
			result, err := linter.Run()
			require.NoError(t, err)

			var found *Issue
			for i := range result.Issues {
				if result.Issues[i].Rule == "empty-generator" {
					found = &result.Issues[i]
					break
				}
			}

			if !tt.want {
				assert.Nil(t, found, "unexpected empty-generator issue")
				return
			}
			require.NotNil(t, found, "expected an empty-generator issue")
			assert.Equal(t, SeverityWarning, found.Severity)
			assert.Equal(t, filepath.Join("_generators", "mygen"), found.File)
			assert.Contains(t, found.Message, "mygen")
			assert.False(t, result.HasErrors(), "an empty generator is a warning, not an error")
		})
	}
}

func TestUT_LintGeneratorDirs_ReservedDirsAreNotEmptyGenerators(t *testing.T) {
	dir := t.TempDir()
	createTemplate(t, dir, validConfig, nil)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "_generators", "_bundles"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "_generators", "_shared"), 0o755))

	linter, err := NewLinter(dir)
	require.NoError(t, err)
	result, err := linter.Run()
	require.NoError(t, err)

	for _, issue := range result.Issues {
		assert.NotEqual(t, "empty-generator", issue.Rule, "unexpected issue: %s", issue.Message)
	}
}

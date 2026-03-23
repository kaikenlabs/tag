package vars

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_WriteText_RootOnly(t *testing.T) {
	t.Parallel()
	report := &Report{
		Root: ScopeResult{
			Scope: "root",
			Declared: []DeclaredVar{
				{Name: "name", Type: "string", Required: true, ReferenceCount: 2, FileCount: 1},
				{Name: "port", Type: "number", Default: 8080},
			},
			Undeclared: nil,
			Unused:     nil,
			Summary:    Summary{Declared: 2, Undeclared: 0, Unused: 0},
		},
	}

	var buf bytes.Buffer
	WriteText(&buf, report)

	output := buf.String()
	assert.Contains(t, output, "Variables declared in tag.template.json:")
	assert.Contains(t, output, "name")
	assert.Contains(t, output, "port")
	assert.Contains(t, output, "No undeclared variables.")
	assert.Contains(t, output, "No unused variables.")
	assert.Contains(t, output, "Summary: 2 declared, 0 undeclared, 0 unused")
}

func TestUT_WriteText_WithGenerators(t *testing.T) {
	t.Parallel()
	report := &Report{
		Root: ScopeResult{
			Scope:   "root",
			Summary: Summary{Declared: 0},
		},
		Generators: []ScopeResult{
			{
				Scope: "model",
				Declared: []DeclaredVar{
					{Name: "entity", Type: "string"},
				},
				Summary: Summary{Declared: 1},
			},
		},
	}

	var buf bytes.Buffer
	WriteText(&buf, report)

	output := buf.String()
	assert.Contains(t, output, "Variables declared in model/tag.template.json:")
	assert.Contains(t, output, "entity")
}

func TestUT_WriteText_WithUndeclaredAndUnused(t *testing.T) {
	t.Parallel()
	report := &Report{
		Root: ScopeResult{
			Scope: "root",
			Declared: []DeclaredVar{
				{Name: "unused_var", Type: "string"},
			},
			Undeclared: []UndeclaredVar{
				{Name: "mystery", References: []Reference{{File: "main.go", Line: 10}}},
			},
			Unused:  []string{"unused_var"},
			Summary: Summary{Declared: 1, Undeclared: 1, Unused: 1},
		},
	}

	var buf bytes.Buffer
	WriteText(&buf, report)

	output := buf.String()
	assert.Contains(t, output, "Undeclared variables found in templates:")
	assert.Contains(t, output, "vars.mystery")
	assert.Contains(t, output, "main.go:10")
	assert.Contains(t, output, "Declared but unused:")
	assert.Contains(t, output, "unused_var")
}

func TestUT_WriteText_EmptyDeclared(t *testing.T) {
	t.Parallel()
	report := &Report{
		Root: ScopeResult{
			Scope:   "root",
			Summary: Summary{},
		},
	}

	var buf bytes.Buffer
	WriteText(&buf, report)
	assert.Contains(t, buf.String(), "(none)")
}

func TestUT_FormatTypeInfo_Derived(t *testing.T) {
	t.Parallel()
	dv := DeclaredVar{Derived: true}
	assert.Equal(t, "(derived)", formatTypeInfo(dv))
}

func TestUT_FormatTypeInfo_WithOptions(t *testing.T) {
	t.Parallel()
	dv := DeclaredVar{Type: "string", Options: []string{"a", "b", "c"}}
	result := formatTypeInfo(dv)
	assert.Contains(t, result, "[a b c]")
}

func TestUT_FormatTypeInfo_Required(t *testing.T) {
	t.Parallel()
	dv := DeclaredVar{Type: "string", Required: true}
	result := formatTypeInfo(dv)
	assert.Contains(t, result, "required")
}

func TestUT_FormatTypeInfo_WithDefault(t *testing.T) {
	t.Parallel()
	dv := DeclaredVar{Type: "number", Default: 42}
	result := formatTypeInfo(dv)
	assert.Contains(t, result, "default: 42")
}

func TestUT_FormatUsage_Zero(t *testing.T) {
	t.Parallel()
	dv := DeclaredVar{ReferenceCount: 0}
	assert.Empty(t, formatUsage(dv))
}

func TestUT_FormatUsage_NonZero(t *testing.T) {
	t.Parallel()
	dv := DeclaredVar{ReferenceCount: 5, FileCount: 3}
	result := formatUsage(dv)
	assert.Contains(t, result, "3 file(s)")
	assert.Contains(t, result, "5 reference(s)")
}

func TestUT_FormatLocations(t *testing.T) {
	t.Parallel()
	refs := []Reference{
		{File: "a.go", Line: 10},
		{File: "b.go", Line: 0},
	}
	result := formatLocations(refs)
	assert.Equal(t, "a.go:10, b.go", result)
}

func TestUT_WriteJSON_Success(t *testing.T) {
	t.Parallel()
	report := &Report{
		Root: ScopeResult{
			Scope:   "root",
			Summary: Summary{Declared: 1},
		},
	}

	var buf bytes.Buffer
	err := WriteJSON(&buf, report)
	require.NoError(t, err)

	var decoded Report
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, "root", decoded.Root.Scope)
}

func TestUT_EnsureNonNilSlices(t *testing.T) {
	t.Parallel()
	report := &Report{
		Root: ScopeResult{
			Scope: "root",
			// all slices nil
		},
	}

	ensureNonNilSlices(report)

	assert.NotNil(t, report.Root.Declared)
	assert.NotNil(t, report.Root.Undeclared)
	assert.NotNil(t, report.Root.Unused)
	assert.NotNil(t, report.Generators)
}

func TestUT_EnsureNonNilSlices_WithReferences(t *testing.T) {
	t.Parallel()
	report := &Report{
		Root: ScopeResult{
			Scope:    "root",
			Declared: []DeclaredVar{{Name: "x", References: nil}},
			Undeclared: []UndeclaredVar{
				{Name: "y", References: nil},
			},
		},
	}

	ensureNonNilSlices(report)
	assert.NotNil(t, report.Root.Declared[0].References)
	assert.NotNil(t, report.Root.Undeclared[0].References)
}

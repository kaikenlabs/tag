package graph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupGraphTemplate creates a temporary template directory with .tag/
// generators and optional scaffold files. Returns the root path.
func setupGraphTemplate(t *testing.T, generators map[string]map[string]string, scaffoldFiles map[string]string) string {
	t.Helper()
	root := t.TempDir()

	for genName, files := range generators {
		genDir := filepath.Join(root, ".tag", genName)
		require.NoError(t, os.MkdirAll(genDir, 0o755))
		for fileName, content := range files {
			require.NoError(t, os.WriteFile(filepath.Join(genDir, fileName), []byte(content), 0o644))
		}
	}

	for path, content := range scaffoldFiles {
		absPath := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o755))
		require.NoError(t, os.WriteFile(absPath, []byte(content), 0o644))
	}

	return root
}

func setupBundle(t *testing.T, root, name, content string) {
	t.Helper()
	bundleDir := filepath.Join(root, ".tag", "_bundles", name)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(bundleDir, name+".json"),
		[]byte(content),
		0o644,
	))
}

func TestUT_AnalyzeCreateActions(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"model": {
				"model.go": "---\nto: internal/models/{{ name }}.go\n---\npackage models\n",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Generators, 1)
	assert.Equal(t, "model", report.Generators[0].Name)
	require.Len(t, report.Generators[0].Actions, 1)
	assert.Equal(t, "create", report.Generators[0].Actions[0].Type)
	assert.Equal(t, "internal/models/{{ name }}.go", report.Generators[0].Actions[0].Target)
}

func TestUT_AnalyzeInjectActions(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"route": {
				"route.go": "---\nto: internal/router.go\ninject: true\nafter: // TAG:routes\n---\nrouter.Handle()\n",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Generators, 1)
	require.Len(t, report.Generators[0].Actions, 1)

	action := report.Generators[0].Actions[0]
	assert.Equal(t, "inject", action.Type)
	assert.Equal(t, "internal/router.go", action.Target)
	assert.Equal(t, "// TAG:routes", action.Marker)
	assert.Equal(t, "after", action.Position)
}

func TestUT_AnalyzeInjectBefore(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"import": {
				"import.go": "---\nto: internal/main.go\ninject: true\nbefore: // TAG:end-imports\n---\nimport \"pkg\"\n",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Generators[0].Actions, 1)

	action := report.Generators[0].Actions[0]
	assert.Equal(t, "inject", action.Type)
	assert.Equal(t, "before", action.Position)
	assert.Equal(t, "// TAG:end-imports", action.Marker)
}

func TestUT_AnalyzeAppendActions(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"migration": {
				"migration.sql": "---\nto: migrations/latest.sql\nappend: true\n---\nALTER TABLE;\n",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Generators, 1)
	require.Len(t, report.Generators[0].Actions, 1)
	assert.Equal(t, "append", report.Generators[0].Actions[0].Type)
	assert.Equal(t, "migrations/latest.sql", report.Generators[0].Actions[0].Target)
}

func TestUT_AnalyzeBundleOrder(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"model": {
				"model.go": "---\nto: internal/models/user.go\n---\npackage models\n",
			},
			"route": {
				"route.go": "---\nto: internal/models/user.go\ninject: true\nafter: // TAG:fields\n---\nName string\n",
			},
		},
		nil,
	)

	bundleJSON := `{"name":"crud","generators":[{"name":"model"},{"name":"route"}]}`
	setupBundle(t, root, "crud", bundleJSON)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Bundles, 1)
	assert.Equal(t, "crud", report.Bundles[0].Name)
	assert.Equal(t, []string{"model", "route"}, report.Bundles[0].Order)
	assert.True(t, report.Bundles[0].ValidOrder, "create before inject should be valid")
}

func TestUT_AnalyzeBundleOrderViolation(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"model": {
				"model.go": "---\nto: internal/models/user.go\n---\npackage models\n",
			},
			"route": {
				"route.go": "---\nto: internal/models/user.go\ninject: true\nafter: // TAG:fields\n---\nName string\n",
			},
		},
		nil,
	)

	// Bad order: inject before create.
	bundleJSON := `{"name":"crud","generators":[{"name":"route"},{"name":"model"}]}`
	setupBundle(t, root, "crud", bundleJSON)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Bundles, 1)
	assert.False(t, report.Bundles[0].ValidOrder, "inject before create should be invalid")

	// Should have an order_violation warning.
	var found bool
	for _, w := range report.Warnings {
		if w.Code == "order_violation" {
			found = true
		}
	}
	assert.True(t, found, "expected order_violation warning")
}

func TestUT_AnalyzeMarkerFound(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"route": {
				"route.go": "---\nto: internal/router.go\ninject: true\nafter: // TAG:routes\n---\nrouter.Handle()\n",
			},
		},
		map[string]string{
			"internal/router.go": "package internal\n\n// TAG:routes\n",
		},
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Markers, 1)
	assert.Equal(t, "internal/router.go", report.Markers[0].File)
	assert.Equal(t, 3, report.Markers[0].Line)
	assert.Equal(t, "// TAG:routes", report.Markers[0].Text)
	assert.Contains(t, report.Markers[0].UsedBy, "route")
}

func TestUT_AnalyzeMissingInjectTarget(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"route": {
				"route.go": "---\nto: internal/router.go\ninject: true\nafter: // TAG:routes\n---\nrouter.Handle()\n",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	var found bool
	for _, w := range report.Warnings {
		if w.Code == "missing_target" && strings.Contains(w.Message, "internal/router.go") {
			found = true
		}
	}
	assert.True(t, found, "expected missing_target warning for internal/router.go")
}

func TestUT_AnalyzeFileConflict(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"gen1": {
				"file.go": "---\nto: output.go\n---\npackage main\n",
			},
			"gen2": {
				"file.go": "---\nto: output.go\n---\npackage main\n",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	var found bool
	for _, w := range report.Warnings {
		if w.Code == "file_conflict" && strings.Contains(w.Message, "output.go") {
			found = true
		}
	}
	assert.True(t, found, "expected file_conflict warning for output.go")
}

func TestUT_AnalyzeEmptyTemplate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	report, err := Analyze(root)
	require.NoError(t, err)
	assert.Empty(t, report.Generators)
	assert.Empty(t, report.Bundles)
	assert.Empty(t, report.Markers)
	assert.Empty(t, report.Warnings)
}

func TestUT_AnalyzeMalformedMetadata(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"broken": {
				"bad.go": "---\nnot a valid line\n---\npackage broken\n",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	// Generator should still appear.
	require.Len(t, report.Generators, 1)
	assert.Equal(t, "broken", report.Generators[0].Name)

	// Should have a malformed_metadata warning.
	var found bool
	for _, w := range report.Warnings {
		if w.Code == "malformed_metadata" {
			found = true
		}
	}
	assert.True(t, found, "expected malformed_metadata warning")
}

func TestUT_AnalyzeMultipleGenerators(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"handler": {
				"handler.go": "---\nto: internal/handler.go\n---\npackage internal\n",
			},
			"model": {
				"model.go": "---\nto: internal/model.go\n---\npackage internal\n",
			},
			"route": {
				"route.go": "---\nto: internal/handler.go\ninject: true\nafter: // TAG:routes\n---\nroute\n",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	assert.Len(t, report.Generators, 3)

	// Verify sorted order.
	assert.Equal(t, "handler", report.Generators[0].Name)
	assert.Equal(t, "model", report.Generators[1].Name)
	assert.Equal(t, "route", report.Generators[2].Name)
}

func TestUT_AnalyzeSkipsReservedDirs(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"model": {
				"model.go": "---\nto: internal/model.go\n---\npackage internal\n",
			},
			"_shared": {
				"shared.go": "---\nto: should/not/appear.go\n---\nshared\n",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)

	// _shared should be skipped.
	assert.Len(t, report.Generators, 1)
	assert.Equal(t, "model", report.Generators[0].Name)
}

func TestUT_AnalyzeNoMetadataBlock(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"simple": {
				"readme.md": "Just a readme with no frontmatter",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Generators, 1)
	assert.Empty(t, report.Generators[0].Actions) // No actions from a file without metadata.
}

func TestUT_AnalyzeMultipleTemplatesInGenerator(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"fullstack": {
				"handler.go":      "---\nto: internal/handler.go\n---\npackage internal\n",
				"handler_test.go": "---\nto: internal/handler_test.go\n---\npackage internal\n",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Generators, 1)
	assert.Len(t, report.Generators[0].Actions, 2)
}

func TestUT_AnalyzeSkipsTagTemplateJSON(t *testing.T) {
	t.Parallel()

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"model": {
				"tag.template.json": `{"vars": {"name": {"type": "string"}}}`,
				"model.go":          "---\nto: internal/model.go\n---\npackage internal\n",
			},
		},
		nil,
	)

	report, err := Analyze(root)
	require.NoError(t, err)
	require.Len(t, report.Generators, 1)
	// Only model.go action, not tag.template.json. The warning assertion is the
	// load-bearing half: the config file is dropped by the loader, so the only way
	// its presence could surface in graph output is a spurious malformed_metadata.
	assert.Len(t, report.Generators[0].Actions, 1)
	assert.Empty(t, report.Warnings)
}

func TestUT_FormatJSON(t *testing.T) {
	t.Parallel()

	report := &GraphReport{
		Generators: []GeneratorNode{
			{
				Name: "model",
				Actions: []ActionInfo{
					{Type: "create", Target: "internal/model.go"},
				},
			},
		},
	}

	var buf strings.Builder
	err := WriteJSON(&buf, report)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &parsed))

	// Verify generators exist.
	gens, ok := parsed["generators"].([]any)
	require.True(t, ok)
	require.Len(t, gens, 1)

	// Verify empty arrays not null.
	bundles, ok := parsed["bundles"].([]any)
	require.True(t, ok)
	assert.Empty(t, bundles)

	markers, ok := parsed["markers"].([]any)
	require.True(t, ok)
	assert.Empty(t, markers)

	warnings, ok := parsed["warnings"].([]any)
	require.True(t, ok)
	assert.Empty(t, warnings)
}

func TestUT_FormatDOT(t *testing.T) {
	t.Parallel()

	report := &GraphReport{
		Generators: []GeneratorNode{
			{
				Name: "model",
				Actions: []ActionInfo{
					{Type: "create", Target: "internal/model.go"},
				},
			},
			{
				Name: "route",
				Actions: []ActionInfo{
					{Type: "inject", Target: "internal/router.go", Marker: "// TAG:routes", Position: "after"},
				},
			},
		},
		Bundles: []BundleInfo{
			{
				Name:       "crud",
				Order:      []string{"model", "route"},
				ValidOrder: true,
			},
		},
	}

	var buf strings.Builder
	WriteDOT(&buf, report)
	output := buf.String()

	assert.Contains(t, output, "digraph generators {")
	assert.Contains(t, output, `"model" [shape=box]`)
	assert.Contains(t, output, `"route" [shape=box]`)
	assert.Contains(t, output, `"internal/model.go" [shape=ellipse]`)
	assert.Contains(t, output, `"model" -> "internal/model.go"`)
	assert.Contains(t, output, `"route" -> "internal/router.go"`)
	assert.Contains(t, output, "subgraph cluster_crud")
	assert.Contains(t, output, `"model" -> "route"`)
	assert.Contains(t, output, "}")
}

func TestUT_FormatText(t *testing.T) {
	t.Parallel()

	report := &GraphReport{
		Generators: []GeneratorNode{
			{
				Name: "model",
				Actions: []ActionInfo{
					{Type: "create", Target: "internal/model.go"},
				},
			},
			{
				Name: "route",
				Actions: []ActionInfo{
					{Type: "inject", Target: "internal/router.go", Marker: "// TAG:routes", Position: "after"},
				},
			},
		},
		Bundles: []BundleInfo{
			{
				Name:       "crud",
				Order:      []string{"model", "route"},
				ValidOrder: true,
			},
		},
		Markers: []MarkerInfo{
			{File: "internal/router.go", Line: 10, Text: "// TAG:routes", UsedBy: []string{"route"}},
		},
		Warnings: []Warning{
			{Code: "missing_target", Generator: "route", Message: "generator injects into non-existent file"},
		},
	}

	var buf strings.Builder
	WriteText(&buf, report)
	output := buf.String()

	assert.Contains(t, output, "Generators:")
	assert.Contains(t, output, "model")
	assert.Contains(t, output, "internal/model.go")
	assert.Contains(t, output, "[create]")
	assert.Contains(t, output, "route")
	assert.Contains(t, output, "[inject after")
	assert.Contains(t, output, "// TAG:routes")
	assert.Contains(t, output, "Bundles:")
	assert.Contains(t, output, "crud")
	assert.Contains(t, output, "Injection markers found:")
	assert.Contains(t, output, "internal/router.go:10")
	assert.Contains(t, output, "Warnings:")
	assert.Contains(t, output, "[missing_target]")
	assert.Contains(t, output, "2 generator(s)")
}

func TestUT_AnalyzeNonexistentPath(t *testing.T) {
	t.Parallel()

	_, err := Analyze("/nonexistent/path/that/does/not/exist")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestUT_AnalyzeNotADirectory(t *testing.T) {
	t.Parallel()

	tmpFile := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(tmpFile, []byte("hello"), 0o644))

	_, err := Analyze(tmpFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestUT_ValidateBundleOrderEmpty(t *testing.T) {
	t.Parallel()

	result := validateBundleOrder(nil, nil)
	assert.True(t, result)
}

func TestUT_FormatTextEmptyReport(t *testing.T) {
	t.Parallel()

	report := &GraphReport{}
	var buf strings.Builder
	WriteText(&buf, report)
	output := buf.String()

	assert.Contains(t, output, "Generators:")
	assert.Contains(t, output, "(none)")
	assert.Contains(t, output, "0 generator(s)")
}

func TestUT_FormatJSONEmptyReport(t *testing.T) {
	t.Parallel()

	report := &GraphReport{}
	var buf strings.Builder
	err := WriteJSON(&buf, report)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &parsed))

	// All arrays should be [] not null.
	for _, key := range []string{"generators", "bundles", "markers", "warnings"} {
		arr, ok := parsed[key].([]any)
		require.True(t, ok, "expected %s to be array", key)
		assert.Empty(t, arr)
	}
}

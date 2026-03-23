package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_ParseRawFrontmatter_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want fileInfoJSON
	}{
		{
			name: "defaults to create",
			raw:  "to: internal/app.go",
			want: fileInfoJSON{To: "internal/app.go", Action: actionCreate},
		},
		{
			name: "append true",
			raw:  "to: routes.go\nappend: true",
			want: fileInfoJSON{To: "routes.go", Action: actionAppend},
		},
		{
			name: "inject true with marker",
			raw:  "to: main.go\ninject: true\nafter: // marker",
			want: fileInfoJSON{To: "main.go", Action: actionInject, After: "// marker"},
		},
		{
			name: "inject wins when both set",
			raw:  "to: main.go\nappend: true\ninject: true\nbefore: // done",
			want: fileInfoJSON{To: "main.go", Action: actionInject, Before: "// done"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseRawFrontmatter(tt.raw)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUT_BuildBundleInfo_SelfContainedIncludedWhenTrue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.json")
	bundleJSON := `{
  "name": "api-pack",
  "description": "Bundle",
  "self_contained": true,
  "generators": [{"name": "handler"}, {"name": "service"}]
}`
	require.NoError(t, os.WriteFile(bundlePath, []byte(bundleJSON), 0o644))

	info, err := buildBundleInfo("api-pack", bundlePath)
	require.NoError(t, err)

	require.NotNil(t, info.SelfContained)
	assert.True(t, *info.SelfContained)
	assert.Equal(t, []string{"handler", "service"}, info.Generators)
}

func TestUT_DetermineSource_NoTemplateOrigin_ReturnsLocal(t *testing.T) {
	t.Parallel()

	cfg := createTestConfig(t, t.TempDir())
	assert.Equal(t, sourceLocal, determineSource(cfg, "/tmp/any"))
}

func TestUT_DetermineSource_GeneratorInsideLibraryTemplate_ReturnsTemplate(t *testing.T) {
	// setupFakeLibrary mutates package-level var, so no t.Parallel.
	templateDir := setupFakeLibrary(t, "my-template")
	cfg := createTestConfigWithLib(t, t.TempDir(), "my-template")
	genDir := filepath.Join(templateDir, types.GeneratorsDir, "model")
	require.NoError(t, os.MkdirAll(genDir, 0o755))

	assert.Equal(t, "template", determineSource(cfg, genDir))
}

func TestUT_DetermineSource_LibraryLookupFails_ReturnsLocal(t *testing.T) {
	// newLocalLibrary is package-level mutable state; no t.Parallel.
	orig := newLocalLibrary
	newLocalLibrary = func() (*library.Library, error) {
		return nil, assert.AnError
	}
	t.Cleanup(func() {
		newLocalLibrary = orig
	})

	cfg := &config.Config{Template: &config.TemplateOrigin{Name: "x"}}
	assert.Equal(t, sourceLocal, determineSource(cfg, "/tmp/whatever"))
}

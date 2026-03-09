package templateupdate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

// mockFetcher implements CommitFetcher for testing by copying a fixture
// directory into the requested destination.
type mockFetcher struct {
	fixtureDir string
	err        error
	called     bool
	calledURL  string
	calledSHA  string
}

func (m *mockFetcher) FetchAtCommit(_ context.Context, url, commitSHA, destDir string) (string, error) {
	m.called = true
	m.calledURL = url
	m.calledSHA = commitSHA
	if m.err != nil {
		return "", m.err
	}
	// Copy fixture into destDir.
	return destDir, copyDir(m.fixtureDir, destDir)
}

// copyDir recursively copies src into dst.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, types.DirMode)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// setupFixture creates a template fixture directory with the given files.
// files is a map of relative path → content (string for text, []byte for binary).
func setupFixture(t *testing.T, files map[string]any) string {
	t.Helper()
	dir := t.TempDir()

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), types.DirMode))

		switch v := content.(type) {
		case string:
			require.NoError(t, os.WriteFile(fullPath, []byte(v), 0o644))
		case []byte:
			require.NoError(t, os.WriteFile(fullPath, v, 0o644))
		default:
			t.Fatalf("unsupported content type for %s", path)
		}
	}
	return dir
}

// minimalConfig returns a minimal valid tag.template.json content.
func minimalConfig() string {
	return `{"name":"test-template","vars":{}}`
}

func TestUT_HistoricalRenderer_RenderAt_Success(t *testing.T) {
	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile:           minimalConfig(),
		"README.md":                        "# {{ vars.project_name }}",
		"src/main.go":                      "package {{ vars.project_name }}",
		filepath.Join("docs", "guide.txt"): "Guide for {{ vars.project_name }}",
	})

	fetcher := &mockFetcher{fixtureDir: fixture}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{"project_name": "myapp"},
	)

	require.NoError(t, err)
	assert.True(t, fetcher.called)
	assert.Equal(t, "https://github.com/example/template.git", fetcher.calledURL)

	assert.Len(t, files, 3)
	assert.Equal(t, "# myapp", string(files["README.md"].Content))
	assert.Equal(t, "package myapp", string(files["src/main.go"].Content))
	assert.Equal(t, "Guide for myapp", string(files["docs/guide.txt"].Content))
	assert.False(t, files["README.md"].IsBinary)
}

func TestUT_HistoricalRenderer_RenderAt_FetchError(t *testing.T) {
	fetcher := &mockFetcher{err: assert.AnError}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{},
	)

	require.Error(t, err)
	assert.Nil(t, files)
	assert.Contains(t, err.Error(), "fetch template at commit")
}

func TestUT_HistoricalRenderer_RenderAt_MissingTemplateConfig(t *testing.T) {
	// Fixture with no tag.template.json.
	fixture := setupFixture(t, map[string]any{
		"README.md": "hello",
	})

	fetcher := &mockFetcher{fixtureDir: fixture}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{},
	)

	require.Error(t, err)
	assert.Nil(t, files)
	assert.Contains(t, err.Error(), types.TemplateConfigFile)
}

func TestUT_HistoricalRenderer_RenderAt_InvalidTemplateConfig(t *testing.T) {
	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: "not valid json{{{",
	})

	fetcher := &mockFetcher{fixtureDir: fixture}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{},
	)

	require.Error(t, err)
	assert.Nil(t, files)
	assert.Contains(t, err.Error(), "parse")
}

func TestUT_HistoricalRenderer_RenderAt_RendersTextFiles(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		vars     map[string]any
		expected string
	}{
		{
			name:     "simple variable",
			content:  "Hello {{ vars.name }}!",
			vars:     map[string]any{"name": "world"},
			expected: "Hello world!",
		},
		{
			name:     "no template syntax",
			content:  "plain text content",
			vars:     map[string]any{},
			expected: "plain text content",
		},
		{
			name:     "conditional block",
			content:  "{% if vars.enabled %}ON{% else %}OFF{% endif %}",
			vars:     map[string]any{"enabled": true},
			expected: "ON",
		},
		{
			name:     "for loop",
			content:  "{% for item in vars.items %}{{ item }} {% endfor %}",
			vars:     map[string]any{"items": []string{"a", "b", "c"}},
			expected: "a b c ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := setupFixture(t, map[string]any{
				types.TemplateConfigFile: minimalConfig(),
				"file.txt":               tt.content,
			})

			fetcher := &mockFetcher{fixtureDir: fixture}
			renderer := NewHistoricalRenderer(fetcher)

			files, err := renderer.RenderAt(
				context.Background(),
				"https://github.com/example/template.git",
				"abc123def456abc123def456abc123def456abc1",
				tt.vars,
			)

			require.NoError(t, err)
			require.Contains(t, files, "file.txt")
			assert.Equal(t, tt.expected, string(files["file.txt"].Content))
			assert.False(t, files["file.txt"].IsBinary)
		})
	}
}

func TestUT_HistoricalRenderer_RenderAt_BinaryPassthrough(t *testing.T) {
	// Create binary content with null bytes.
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00, 0x01, 0x02, 0x03}

	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: minimalConfig(),
		"image.png":              binaryContent,
	})

	fetcher := &mockFetcher{fixtureDir: fixture}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{},
	)

	require.NoError(t, err)
	require.Contains(t, files, "image.png")
	assert.Equal(t, binaryContent, files["image.png"].Content)
	assert.True(t, files["image.png"].IsBinary)
}

func TestUT_HistoricalRenderer_RenderAt_PathPlaceholders(t *testing.T) {
	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile:                     minimalConfig(),
		"{{ vars.pkg_name }}/main.go":                "package {{ vars.pkg_name }}",
		"{{ vars.pkg_name }}/{{ vars.pkg_name }}.go": "// {{ vars.pkg_name }} impl",
	})

	fetcher := &mockFetcher{fixtureDir: fixture}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{"pkg_name": "mylib"},
	)

	require.NoError(t, err)
	assert.Contains(t, files, "mylib/main.go")
	assert.Contains(t, files, "mylib/mylib.go")
	assert.Equal(t, "package mylib", string(files["mylib/main.go"].Content))
}

func TestUT_HistoricalRenderer_RenderAt_SkipsReservedPaths(t *testing.T) {
	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile:                          minimalConfig(),
		types.TagIgnoreFile:                               "*.bak",
		types.CacheMetaFile:                               "{}",
		filepath.Join(types.GeneratorsDir, "gen.go"):      "generator",
		filepath.Join(types.TemplatesDir, "installed.go"): "installed",
		"src/app.go":                                      "package app",
	})

	fetcher := &mockFetcher{fixtureDir: fixture}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{},
	)

	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files, "src/app.go")

	// Verify reserved entries are excluded.
	assert.NotContains(t, files, types.TemplateConfigFile)
	assert.NotContains(t, files, types.TagIgnoreFile)
	assert.NotContains(t, files, types.CacheMetaFile)
}

func TestUT_HistoricalRenderer_RenderAt_AppliesTagignore(t *testing.T) {
	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile:             minimalConfig(),
		types.TagIgnoreFile:                  "*.log\nbuild/",
		"src/app.go":                         "package app",
		"debug.log":                          "log data",
		filepath.Join("build", "output.bin"): "binary",
	})

	fetcher := &mockFetcher{fixtureDir: fixture}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{},
	)

	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files, "src/app.go")
	assert.NotContains(t, files, "debug.log")
	assert.NotContains(t, files, "build/output.bin")
}

func TestUT_HistoricalRenderer_RenderAt_EmptyTemplate(t *testing.T) {
	// Template with only config, no renderable files.
	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: minimalConfig(),
	})

	fetcher := &mockFetcher{fixtureDir: fixture}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{},
	)

	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestUT_HistoricalRenderer_RenderAt_ContextCancelled(t *testing.T) {
	t.Run("cancelled before fetch", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		fetcher := &mockFetcher{fixtureDir: t.TempDir()}
		renderer := NewHistoricalRenderer(fetcher)

		files, err := renderer.RenderAt(
			ctx,
			"https://github.com/example/template.git",
			"abc123def456abc123def456abc123def456abc1",
			map[string]any{},
		)

		require.Error(t, err)
		assert.Nil(t, files)
		assert.ErrorIs(t, err, context.Canceled)
		assert.False(t, fetcher.called)
	})
}

func TestUT_HistoricalRenderer_RenderAt_TemplateSyntaxError(t *testing.T) {
	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: minimalConfig(),
		"broken.txt":             "{% invalid syntax %}",
	})

	fetcher := &mockFetcher{fixtureDir: fixture}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{},
	)

	require.Error(t, err)
	assert.Nil(t, files)
	assert.Contains(t, err.Error(), "render file")
}

func TestUT_HistoricalRenderer_RenderAt_MixedTextAndBinary(t *testing.T) {
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE}

	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: minimalConfig(),
		"README.md":              "# {{ vars.name }}",
		"src/main.go":            "package {{ vars.name }}",
		"assets/logo.png":        binaryContent,
		"data/config.yml":        "name: {{ vars.name }}",
	})

	fetcher := &mockFetcher{fixtureDir: fixture}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{"name": "testapp"},
	)

	require.NoError(t, err)
	assert.Len(t, files, 4)

	// Text files are rendered.
	assert.Equal(t, "# testapp", string(files["README.md"].Content))
	assert.False(t, files["README.md"].IsBinary)

	assert.Equal(t, "package testapp", string(files["src/main.go"].Content))
	assert.False(t, files["src/main.go"].IsBinary)

	assert.Equal(t, "name: testapp", string(files["data/config.yml"].Content))
	assert.False(t, files["data/config.yml"].IsBinary)

	// Binary file is passed through.
	assert.Equal(t, binaryContent, files["assets/logo.png"].Content)
	assert.True(t, files["assets/logo.png"].IsBinary)
}

func TestUT_HistoricalRenderer_RenderAt_PreservesFileMode(t *testing.T) {
	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: minimalConfig(),
		"script.sh":              "#!/bin/bash\necho {{ vars.name }}",
	})
	// Make the script executable.
	require.NoError(t, os.Chmod(filepath.Join(fixture, "script.sh"), 0o755))

	fetcher := &mockFetcher{fixtureDir: fixture}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{"name": "world"},
	)

	require.NoError(t, err)
	require.Contains(t, files, "script.sh")
	// Verify executable bit is preserved.
	assert.NotZero(t, files["script.sh"].Mode&0o111)
}

func TestUT_HistoricalRenderer_RenderAt_RejectsPathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		vars     map[string]any
	}{
		{
			name:     "dot-dot traversal",
			filename: "{{ vars.path }}/evil.txt",
			vars:     map[string]any{"path": "../.."},
		},
		{
			name:     "absolute path",
			filename: "{{ vars.path }}",
			vars:     map[string]any{"path": "/etc/passwd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := setupFixture(t, map[string]any{
				types.TemplateConfigFile: minimalConfig(),
				tt.filename:              "malicious content",
			})

			fetcher := &mockFetcher{fixtureDir: fixture}
			renderer := NewHistoricalRenderer(fetcher)

			files, err := renderer.RenderAt(
				context.Background(),
				"https://github.com/example/template.git",
				"abc123def456abc123def456abc123def456abc1",
				tt.vars,
			)

			require.Error(t, err)
			assert.Nil(t, files)
			assert.Contains(t, err.Error(), "escapes output directory")
		})
	}
}

func TestUT_HistoricalRenderer_RenderAt_DuplicateRenderedPaths(t *testing.T) {
	// Two files that render to the same path.
	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: minimalConfig(),
		"a/file.txt":             "content A",
		"b/file.txt":             "content B",
	})

	// Create a vars setup where both paths render to same target.
	// Since paths don't use vars, they won't collide. Use a different approach:
	// two source files with template names that resolve to same output.
	fixture2 := setupFixture(t, map[string]any{
		types.TemplateConfigFile:  minimalConfig(),
		"{{ vars.name }}.txt":     "first",
		"{{ vars.alt_name }}.txt": "second",
	})

	fetcher := &mockFetcher{fixtureDir: fixture2}
	renderer := NewHistoricalRenderer(fetcher)

	files, err := renderer.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{"name": "same", "alt_name": "same"},
	)

	require.Error(t, err)
	assert.Nil(t, files)
	assert.Contains(t, err.Error(), "duplicate rendered path")

	// Non-colliding case should work fine.
	_ = fixture
	fetcher2 := &mockFetcher{fixtureDir: fixture2}
	renderer2 := NewHistoricalRenderer(fetcher2)

	files2, err2 := renderer2.RenderAt(
		context.Background(),
		"https://github.com/example/template.git",
		"abc123def456abc123def456abc123def456abc1",
		map[string]any{"name": "first", "alt_name": "second"},
	)

	require.NoError(t, err2)
	assert.Len(t, files2, 2)
}

package templateupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_IgnoreMatcher_BuiltinDefaults(t *testing.T) {
	t.Parallel()

	m, err := NewIgnoreMatcher(IgnoreMatcherOptions{})
	require.NoError(t, err)

	assert.True(t, m.ShouldSkip(".git", true), ".git should be skipped")
	assert.True(t, m.ShouldSkip(".git/config", false), ".git/config should be skipped")
	assert.True(t, m.ShouldSkip(types.TemplatesDir, true), ".tag should be skipped")
	assert.True(t, m.ShouldSkip(types.TagConfigFile, false), ".tagconfig.json should be skipped")
	assert.False(t, m.ShouldSkip("src/main.go", false), "normal files should not be skipped")
}

func TestUT_IgnoreMatcher_TagignorePatterns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, types.TagIgnoreFile, "# Comment\n\n*.log\nbuild/\n")

	m, err := NewIgnoreMatcher(IgnoreMatcherOptions{ProjectRoot: dir})
	require.NoError(t, err)

	assert.True(t, m.ShouldSkip("app.log", false), "*.log should match")
	assert.True(t, m.ShouldSkip("build", true), "build/ should match directory")
	assert.False(t, m.ShouldSkip("src/main.go", false), "unmatched file should pass")
}

func TestUT_IgnoreMatcher_TagconfigSkipPatterns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := tagconfigPartial{SkipPatterns: []string{"vendor/**", "*.generated.go"}}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	writeFile(t, dir, types.TagConfigFile, string(data))

	m, err := NewIgnoreMatcher(IgnoreMatcherOptions{ProjectRoot: dir})
	require.NoError(t, err)

	assert.True(t, m.ShouldSkip("vendor/lib/foo.go", false), "vendor/** should match")
	assert.True(t, m.ShouldSkip("models.generated.go", false), "*.generated.go should match")
	assert.False(t, m.ShouldSkip("main.go", false))
}

func TestUT_IgnoreMatcher_CLIPatterns(t *testing.T) {
	t.Parallel()

	m, err := NewIgnoreMatcher(IgnoreMatcherOptions{
		CLIPatterns: []string{"*.env", "secrets/**"},
	})
	require.NoError(t, err)

	assert.True(t, m.ShouldSkip("prod.env", false))
	assert.True(t, m.ShouldSkip("secrets/key.pem", false))
	assert.False(t, m.ShouldSkip("config.yaml", false))
}

func TestUT_IgnoreMatcher_PriorityOrder(t *testing.T) {
	t.Parallel()

	// .tagignore excludes *.env, CLI re-includes it with negation.
	m, err := NewIgnoreMatcher(IgnoreMatcherOptions{
		TagignorePatterns: []string{"*.env"},
		CLIPatterns:       []string{"!prod.env"},
	})
	require.NoError(t, err)

	assert.False(t, m.ShouldSkip("prod.env", false), "CLI negation should override .tagignore")
	assert.True(t, m.ShouldSkip("dev.env", false), "other .env files still excluded")
}

func TestUT_IgnoreMatcher_NegationPattern(t *testing.T) {
	t.Parallel()

	m, err := NewIgnoreMatcher(IgnoreMatcherOptions{
		TagignorePatterns: []string{"docs/**", "!docs/README.md"},
	})
	require.NoError(t, err)

	assert.True(t, m.ShouldSkip("docs/internal.md", false), "docs/** should be excluded")
	assert.False(t, m.ShouldSkip("docs/README.md", false), "negation should re-include")
}

func TestUT_IgnoreMatcher_DoublestarGlob(t *testing.T) {
	t.Parallel()

	m, err := NewIgnoreMatcher(IgnoreMatcherOptions{
		CLIPatterns: []string{"**/*.tmp"},
	})
	require.NoError(t, err)

	assert.True(t, m.ShouldSkip("a/b/c/file.tmp", false))
	assert.True(t, m.ShouldSkip("file.tmp", false))
	assert.False(t, m.ShouldSkip("file.go", false))
}

func TestUT_IgnoreMatcher_EmptyPatterns(t *testing.T) {
	t.Parallel()

	m, err := NewIgnoreMatcher(IgnoreMatcherOptions{})
	require.NoError(t, err)

	// Only builtins should be skipped.
	assert.True(t, m.ShouldSkip(".git", true))
	assert.False(t, m.ShouldSkip("README.md", false))
}

func TestUT_IgnoreMatcher_NoTagignoreFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// No .tagignore file — should not error.
	m, err := NewIgnoreMatcher(IgnoreMatcherOptions{ProjectRoot: dir})
	require.NoError(t, err)
	assert.NotNil(t, m)
}

func TestUT_IgnoreMatcher_NoTagconfigFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// No .tagconfig.json — should not error.
	m, err := NewIgnoreMatcher(IgnoreMatcherOptions{ProjectRoot: dir})
	require.NoError(t, err)
	assert.NotNil(t, m)
}

func TestUT_IgnoreMatcher_MalformedTagconfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, types.TagConfigFile, "not json{{{")

	_, err := NewIgnoreMatcher(IgnoreMatcherOptions{ProjectRoot: dir})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestUT_IgnoreMatcher_DirectSuppliedPatterns(t *testing.T) {
	t.Parallel()

	// When patterns are directly supplied, ProjectRoot is not needed.
	m, err := NewIgnoreMatcher(IgnoreMatcherOptions{
		TagignorePatterns: []string{"*.bak"},
		TagconfigPatterns: []string{"dist/**"},
		CLIPatterns:       []string{"*.tmp"},
	})
	require.NoError(t, err)

	assert.True(t, m.ShouldSkip("old.bak", false))
	assert.True(t, m.ShouldSkip("dist/bundle.js", false))
	assert.True(t, m.ShouldSkip("scratch.tmp", false))
}

// writeFile is a test helper that creates a file in the given directory.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

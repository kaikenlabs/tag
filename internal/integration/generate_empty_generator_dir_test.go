package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	emptyGenDirBundleMarker    = "empty-gen-dir-bundle-marker-3a7f"
	emptyGenDirGeneratorMarker = "empty-gen-dir-generator-marker-6c1e"
)

// emptyGenDirFixtureTemplate builds a source template where a generator and
// a bundle share the name "foo". realgen renders the bundle's distinct
// marker; when populateFoo is true, foo/foo.tmpl renders the generator's own
// distinct marker so a positive-control subtest can prove which path ran.
func emptyGenDirFixtureTemplate(t *testing.T, populateFoo, includeBundle bool) string {
	t.Helper()
	dir := t.TempDir()

	cfg := `{
  "name": "empty-gen-dir-fixture",
  "version": "1.0.0",
  "vars": {
    "project_name": {"type": "string", "default": "demo"}
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(cfg), 0o644))

	fooDir := filepath.Join(dir, "_generators", "foo")
	require.NoError(t, os.MkdirAll(fooDir, 0o755))
	if populateFoo {
		require.NoError(t, os.WriteFile(
			filepath.Join(fooDir, "foo.tmpl"),
			[]byte("---\nto: {{ name }}.go\n---\npackage generated\n\nconst Marker = \""+emptyGenDirGeneratorMarker+"\"\n"),
			0o644,
		))
	} else {
		entries, err := os.ReadDir(fooDir)
		require.NoError(t, err)
		require.Empty(t, entries, "fixture invariant: foo generator dir must actually be empty")
	}

	if includeBundle {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "_generators", "realgen"), 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "_generators", "realgen", "realgen.tmpl"),
			[]byte("---\nto: {{ name }}.go\n---\npackage generated\n\nconst Marker = \""+emptyGenDirBundleMarker+"\"\n"),
			0o644,
		))

		require.NoError(t, os.MkdirAll(filepath.Join(dir, "_generators", "_bundles", "foo"), 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "_generators", "_bundles", "foo", "foo.json"),
			[]byte(`{"name":"foo","generators":[{"name":"realgen"}]}`),
			0o644,
		))
	}

	return dir
}

// TestIT_Generate_EmptyGeneratorDirDoesNotShadowBundle pins #436 end-to-end:
// an empty generator directory must not silently win over a same-named,
// working bundle. The oracle is the exact rendered file bytes — exit code
// alone is worthless here, since unfixed code exits 0.
func TestIT_Generate_EmptyGeneratorDirDoesNotShadowBundle(t *testing.T) {
	t.Run("library _generators root: bundle bytes win", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := emptyGenDirFixtureTemplate(t, false, true)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"lib", "add", templateSrc, "--as", "emptygendir-lib")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err = runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", "emptygendir-lib", "generated", "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		resolvedOutDir := resolveOutDir(t, outDir)
		stdout, stderr, err = runTagSubprocessEnv(t, ctx, resolvedOutDir, sandbox.env(),
			"generate", "foo", "widget")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		content, readErr := os.ReadFile(filepath.Join(resolvedOutDir, "widget.go"))
		require.NoError(t, readErr)
		assert.Contains(t, string(content), emptyGenDirBundleMarker)
		assert.NotContains(t, string(content), emptyGenDirGeneratorMarker)
	})

	t.Run("project-local .tag: bundle bytes win", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := emptyGenDirFixtureTemplate(t, false, true)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", templateSrc, "generated", "--output", outDir, "--no-input", "--no-library")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		resolvedOutDir := resolveOutDir(t, outDir)

		fooDir := filepath.Join(resolvedOutDir, ".tag", "foo")
		entries, err := os.ReadDir(fooDir)
		require.NoError(t, err)
		require.Empty(t, entries, "fixture invariant: project-local .tag/foo must actually be empty")

		stdout, stderr, err = runTagSubprocessEnv(t, ctx, resolvedOutDir, sandbox.env(),
			"generate", "foo", "widget")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		content, readErr := os.ReadFile(filepath.Join(resolvedOutDir, "widget.go"))
		require.NoError(t, readErr)
		assert.Contains(t, string(content), emptyGenDirBundleMarker)
		assert.NotContains(t, string(content), emptyGenDirGeneratorMarker)
	})

	t.Run("populated generator still beats bundle", func(t *testing.T) {
		// Positive control: passes on unfixed code too.
		sandbox := newNoLibrarySandbox(t)
		templateSrc := emptyGenDirFixtureTemplate(t, true, true)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", templateSrc, "generated", "--output", outDir, "--no-input", "--no-library")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		resolvedOutDir := resolveOutDir(t, outDir)
		stdout, stderr, err = runTagSubprocessEnv(t, ctx, resolvedOutDir, sandbox.env(),
			"generate", "foo", "widget")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		content, readErr := os.ReadFile(filepath.Join(resolvedOutDir, "widget.go"))
		require.NoError(t, readErr)
		assert.Contains(t, string(content), emptyGenDirGeneratorMarker)
		assert.NotContains(t, string(content), emptyGenDirBundleMarker)
	})

	t.Run("empty dir, no bundle: loud failure", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := emptyGenDirFixtureTemplate(t, false, false)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", templateSrc, "generated", "--output", outDir, "--no-input", "--no-library")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		resolvedOutDir := resolveOutDir(t, outDir)
		_, _, err = runTagSubprocessEnv(t, ctx, resolvedOutDir, sandbox.env(),
			"generate", "foo", "widget")
		assert.Error(t, err, "an empty generator dir with no bundle must fail loudly")

		_, statErr := os.Stat(filepath.Join(resolvedOutDir, "widget.go"))
		assert.True(t, os.IsNotExist(statErr), "no file must be written on a loud failure")
	})
}

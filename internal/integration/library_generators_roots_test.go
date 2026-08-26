package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/remote"
)

const (
	libGenRootsSharedContent = "// shared-header-libgens-fixture\n"
	libGenRootsMarker        = "libgens-fixture-marker-71cd"
)

const libGenRootsExpectedOutput = libGenRootsSharedContent + "\npackage generated\n\nconst Marker = \"" + libGenRootsMarker + "\"\n"

func libGenRootsFixtureTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cfg := `{
  "name": "libgen-roots-fixture",
  "version": "1.0.0",
  "vars": {
    "project_name": {"type": "string", "default": "demo"}
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(cfg), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# {{ vars.project_name }}\n"), 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "_generators", "_shared"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "_generators", "_shared", "header.tmpl"),
		[]byte(libGenRootsSharedContent),
		0o644,
	))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "_generators", "mygen"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "_generators", "mygen", "mygen.tmpl"),
		[]byte("---\nto: {{ name }}.go\n---\n{% include \"header.tmpl\" %}\npackage generated\n\nconst Marker = \""+libGenRootsMarker+"\"\n"),
		0o644,
	))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "_generators", "_bundles", "mybundle"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "_generators", "_bundles", "mybundle", "mybundle.json"),
		[]byte(`{"name":"mybundle","generators":[{"name":"mygen"}]}`),
		0o644,
	))

	return dir
}

func resolveOutDir(t *testing.T, outDir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(outDir)
	require.NoError(t, err)
	return resolved
}

func TestIT_LibraryGenerators_GeneratorsDirRoot(t *testing.T) {
	t.Run("local lib add then generate renders bytes", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := libGenRootsFixtureTemplate(t)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"lib", "add", templateSrc, "--as", "libgenroots-local")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err = runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", "libgenroots-local", "generated", "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		resolvedOutDir := resolveOutDir(t, outDir)
		genStdout, genStderr, genErr := runTagSubprocessEnv(t, ctx, resolvedOutDir, sandbox.env(),
			"generate", "mygen", "widget")
		require.NoError(t, genErr, "stdout=%s stderr=%s", genStdout, genStderr)

		content, readErr := os.ReadFile(filepath.Join(resolvedOutDir, "widget.go"))
		require.NoError(t, readErr)
		assert.Equal(t, libGenRootsExpectedOutput, string(content))
	})

	t.Run("remote scaffold then generate renders bytes", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := libGenRootsFixtureTemplate(t)
		const ref = "https://example.invalid/libgenroots-remote-fixture.zip@v1"
		seedRemoteCache(t, sandbox.cacheDir, ref, templateSrc)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", ref, "generated", "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		libName := remote.LibraryName(ref)
		assert.Equal(t, libName, tagConfigTemplate(t, outDir).Name,
			"fixture invariant: a free slot must record the library name")

		resolvedOutDir := resolveOutDir(t, outDir)
		genStdout, genStderr, genErr := runTagSubprocessEnv(t, ctx, resolvedOutDir, sandbox.env(),
			"generate", "mygen", "widget")
		require.NoError(t, genErr, "stdout=%s stderr=%s", genStdout, genStderr)

		content, readErr := os.ReadFile(filepath.Join(resolvedOutDir, "widget.go"))
		require.NoError(t, readErr)
		assert.Equal(t, libGenRootsExpectedOutput, string(content))
	})

	t.Run("--add-to-lib then generate renders bytes", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := libGenRootsFixtureTemplate(t)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", templateSrc, "generated", "--output", outDir, "--no-input", "--add-to-lib")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		assert.NotEmpty(t, tagConfigTemplate(t, outDir).Name,
			"fixture invariant: --add-to-lib must record a library name")

		resolvedOutDir := resolveOutDir(t, outDir)
		genStdout, genStderr, genErr := runTagSubprocessEnv(t, ctx, resolvedOutDir, sandbox.env(),
			"generate", "mygen", "widget")
		require.NoError(t, genErr, "stdout=%s stderr=%s", genStdout, genStderr)

		content, readErr := os.ReadFile(filepath.Join(resolvedOutDir, "widget.go"))
		require.NoError(t, readErr)
		assert.Equal(t, libGenRootsExpectedOutput, string(content))
	})

	t.Run("bundle from generators dir renders bytes", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := libGenRootsFixtureTemplate(t)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"lib", "add", templateSrc, "--as", "libgenroots-bundle")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err = runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", "libgenroots-bundle", "generated", "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		resolvedOutDir := resolveOutDir(t, outDir)
		genStdout, genStderr, genErr := runTagSubprocessEnv(t, ctx, resolvedOutDir, sandbox.env(),
			"generate", "mybundle", "widget")
		require.NoError(t, genErr, "stdout=%s stderr=%s", genStdout, genStderr)

		content, readErr := os.ReadFile(filepath.Join(resolvedOutDir, "widget.go"))
		require.NoError(t, readErr)
		assert.Equal(t, libGenRootsExpectedOutput, string(content))
	})
}

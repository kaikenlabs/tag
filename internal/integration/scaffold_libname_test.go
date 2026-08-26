package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/remote"
)

// libNameFreeRef and libNameTakenRef are two DIFFERENT hosts so their
// DeriveName collides (both basenames are "service-template") while their
// CacheKeys stay distinct, letting both cache entries coexist. Verified by
// TestIT_Scaffold_LibrarySlot/free's own DeriveName-collision assertion
// below before either subtest relies on it.
const (
	libNameFreeRef  = "https://orga.invalid/service-template.zip@v1"
	libNameTakenRef = "https://orgb.invalid/service-template.zip@v1"
)

// libNameGeneratorContent mirrors noLibraryGeneratorContent's role for this
// file: known bytes copied verbatim, so a scaffold or library-add can be
// asserted on content, not just presence.
const libNameGeneratorContent = "package nolibgen\n\nconst Marker = \"libname-fixture-marker-2a7e\"\n"

// libNameFixtureTemplate mirrors noLibraryFixtureTemplate (a rendered file
// plus a _generators/ subdir with known bytes) under an independent name so
// this file does not depend on scaffold_nolibrary_test.go's fixture staying
// byte-identical.
func libNameFixtureTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := `{
  "name": "libname-fixture",
  "version": "1.0.0",
  "vars": {
    "project_name": {"type": "string", "default": "demo"}
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(cfg), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# {{ vars.project_name }}\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "_generators", "nolibgen"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "_generators", "nolibgen", "generator.go"),
		[]byte(libNameGeneratorContent),
		0o644,
	))
	return dir
}

// TestIT_Scaffold_LibrarySlot drives the #429 fix end to end: a remote
// scaffold whose derived library name is FREE records that name and skips
// the project generator copy (today's behaviour, unchanged); one whose
// derived name is already TAKEN by a different source records no name and
// falls back to copying generators into the project instead of silently
// losing them.
func TestIT_Scaffold_LibrarySlot(t *testing.T) {
	t.Run("free", func(t *testing.T) {
		require.Equal(t, remote.DeriveName(libNameFreeRef), remote.DeriveName(libNameTakenRef),
			"fixture invariant: the two refs must collide under DeriveName")

		sandbox := newNoLibrarySandbox(t)
		templateSrc := libNameFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, libNameFreeRef, templateSrc)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", libNameFreeRef, "generated", "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		libName := remote.LibraryName(libNameFreeRef)
		assert.Equal(t, libName, tagConfigTemplate(t, outDir).Name)

		_, statErr := os.Stat(filepath.Join(outDir, ".tag", "nolibgen"))
		assert.True(t, os.IsNotExist(statErr), "a free slot must still skip the project generator copy")

		lib := library.NewLocal(sandbox.libraryDataDir())
		entry, getErr := lib.Get(libName)
		require.NoError(t, getErr, "a free slot must add the template to the library")
		assert.Equal(t, libNameFreeRef, entry.Source)

		templateDir, pathErr := lib.TemplatePath(libName)
		require.NoError(t, pathErr)
		content, readErr := os.ReadFile(filepath.Join(templateDir, "_generators", "nolibgen", "generator.go"))
		require.NoError(t, readErr)
		assert.Equal(t, libNameGeneratorContent, string(content))
	})

	t.Run("taken_by_other", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := libNameFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, libNameTakenRef, templateSrc)

		libName := remote.LibraryName(libNameTakenRef)
		otherSource := "https://orgc.invalid/service-template.zip@v1"
		require.NotEqual(t, libNameTakenRef, otherSource)

		otherTemplateDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(otherTemplateDir, "MARKER.txt"), []byte("other-org"), 0o644))

		lib := library.NewLocal(sandbox.libraryDataDir())
		_, addErr := lib.Add(context.Background(), library.AddOptions{
			Ref:         otherSource,
			Name:        libName,
			ResolvedDir: otherTemplateDir,
			Force:       true,
		})
		require.NoError(t, addErr)
		libBefore := snapshotTree(t, sandbox.libraryDataDir())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", libNameTakenRef, "generated", "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		assert.Empty(t, tagConfigTemplate(t, outDir).Name,
			"a taken slot must not record a library name to resolve generators from")

		content, readErr := os.ReadFile(filepath.Join(outDir, ".tag", "nolibgen", "generator.go"))
		require.NoError(t, readErr, "a taken slot must fall back to copying the project's own generators")
		assert.Equal(t, libNameGeneratorContent, string(content))

		// A weak oracle on its own: this also holds on the pre-#429 tree,
		// which early-returned out of addToLibrary just as often. It is the
		// byte + empty-name assertions above, plus the "free" subtest as a
		// positive control, that make this pair non-vacuous.
		assert.Equal(t, libBefore, snapshotTree(t, sandbox.libraryDataDir()))

		out := string(stdout)
		assert.Contains(t, out, libName)
		assert.Contains(t, out, otherSource)
		assert.Contains(t, out, libNameTakenRef)
	})
}

// collidingGeneratorMarker builds a minimal, VALID generator template (with
// "to:" frontmatter, unlike noLibraryGeneratorContent/libNameGeneratorContent
// which are plain copied files) so `tag generate` can actually run it.
// marker is embedded in the rendered output so the two fixtures below are
// distinguishable by content.
func collidingGeneratorMarker(marker string) string {
	return "---\nto: {{ name }}.go\n---\npackage {{ name }}\n\nconst Marker = \"" + marker + "\"\n"
}

const (
	collidingProjectMarker = "colliding-project-8f2a"
	collidingLibraryMarker = "colliding-library-c4d1"
)

// TestIT_Scaffold_CollidingLibraryName_GeneratesFromProject reproduces the
// taken_by_other setup and then proves the READ half of the #429 fix: with
// no library name recorded in .tagconfig.json, `tag generate` must resolve
// the generator from the project's own copy, never from the unrelated
// library entry occupying the same derived name.
func TestIT_Scaffold_CollidingLibraryName_GeneratesFromProject(t *testing.T) {
	sandbox := newNoLibrarySandbox(t)

	templateSrc := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(templateSrc, "tag.template.json"), []byte(
		`{"name":"libname-fixture","version":"1.0.0","vars":{"project_name":{"type":"string","default":"demo"}}}`,
	), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templateSrc, "README.md"), []byte("# {{ vars.project_name }}\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(templateSrc, "_generators", "nolibgen"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(templateSrc, "_generators", "nolibgen", "gen.txt"),
		[]byte(collidingGeneratorMarker(collidingProjectMarker)),
		0o644,
	))
	seedRemoteCache(t, sandbox.cacheDir, libNameTakenRef, templateSrc)

	libName := remote.LibraryName(libNameTakenRef)
	otherSource := "https://orgc.invalid/service-template.zip@v1"

	libraryTemplateDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(libraryTemplateDir, ".tag", "nolibgen"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(libraryTemplateDir, ".tag", "nolibgen", "gen.txt"),
		[]byte(collidingGeneratorMarker(collidingLibraryMarker)),
		0o644,
	))

	lib := library.NewLocal(sandbox.libraryDataDir())
	_, addErr := lib.Add(context.Background(), library.AddOptions{
		Ref:         otherSource,
		Name:        libName,
		ResolvedDir: libraryTemplateDir,
		Force:       true,
	})
	require.NoError(t, addErr)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	outDir := filepath.Join(sandbox.workDir, "project")
	stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
		"scaffold", libNameTakenRef, "generated", "--output", outDir, "--no-input")
	require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)
	require.Empty(t, tagConfigTemplate(t, outDir).Name, "fixture invariant: this run must land in the taken_by_other case")

	// On macOS resolve the project dir before driving a second subprocess
	// from inside it: t.TempDir() can sit under a symlink there, and a
	// subprocess spawned with that as its cwd sees the unresolved spelling.
	resolvedOutDir, evalErr := filepath.EvalSymlinks(outDir)
	require.NoError(t, evalErr)

	genStdout, genStderr, genErr := runTagSubprocessEnv(t, ctx, resolvedOutDir, sandbox.env(),
		"generate", "nolibgen", "widget")
	require.NoError(t, genErr, "stdout=%s stderr=%s", genStdout, genStderr)

	content, readErr := os.ReadFile(filepath.Join(resolvedOutDir, "widget.go"))
	require.NoError(t, readErr)
	assert.Contains(t, string(content), collidingProjectMarker,
		"generate must resolve the PROJECT's own generator, not the colliding library entry's")
	assert.NotContains(t, string(content), collidingLibraryMarker)
}

// TestIT_LibName_LongNameRoundTrip discharges #430's actual acceptance
// criterion: not just that LibraryName is collision-free in isolation, but
// that the digested name it produces is usable end to end through every
// command a user would run against it.
func TestIT_LibName_LongNameRoundTrip(t *testing.T) {
	sandbox := newNoLibrarySandbox(t)
	templateSrc := libNameFixtureTemplate(t)
	const ref = "https://roundtrip.invalid/roundtrip-template.zip@v1"
	seedRemoteCache(t, sandbox.cacheDir, ref, templateSrc)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	firstOut := filepath.Join(sandbox.workDir, "first")
	stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
		"scaffold", ref, "generated", "--output", firstOut, "--no-input")
	require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

	lsStdout, lsStderr, lsErr := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
		"lib", "ls", "--format", "json")
	require.NoError(t, lsErr, "stdout=%s stderr=%s", lsStdout, lsStderr)

	var doc struct {
		Templates []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"templates"`
	}
	require.NoError(t, json.Unmarshal(lsStdout, &doc))

	var shownName string
	for _, e := range doc.Templates {
		if e.Source == ref {
			shownName = e.Name
			break
		}
	}
	require.NotEmpty(t, shownName, "the entry `tag lib ls` prints for %s", ref)
	assert.Equal(t, remote.LibraryName(ref), shownName)

	secondOut := filepath.Join(sandbox.workDir, "second")
	scaffoldStdout, scaffoldStderr, scaffoldErr := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
		"scaffold", shownName, "generated", "--output", secondOut, "--no-input")
	require.NoError(t, scaffoldErr, "tag scaffold <lib-ls-name> must succeed: stdout=%s stderr=%s", scaffoldStdout, scaffoldStderr)
	_, statErr := os.Stat(filepath.Join(secondOut, "README.md"))
	assert.NoError(t, statErr)

	rmStdout, rmStderr, rmErr := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
		"lib", "rm", shownName)
	require.NoError(t, rmErr, "tag lib rm <lib-ls-name> must succeed: stdout=%s stderr=%s", rmStdout, rmStderr)

	lib := library.NewLocal(sandbox.libraryDataDir())
	_, getErr := lib.Get(shownName)
	assert.ErrorIs(t, getErr, library.ErrTemplateNotFound)
}

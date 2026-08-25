package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/remote"
)

const noLibraryFixtureRef = "https://example.invalid/nolibrary-fixture.zip@v1"

const noLibraryGeneratorContent = "package nolibgen\n\nconst Marker = \"nolibrary-fixture-marker-9f3c\"\n"

// noLibraryFixtureTemplate builds a source template dir with a rendered file
// and a _generators/ subdir carrying known-byte-content, so a scaffold run
// can be asserted both on what it renders and on what it copies verbatim.
func noLibraryFixtureTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg := `{
  "name": "nolibrary-fixture",
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
		[]byte(noLibraryGeneratorContent),
		0o644,
	))
	return dir
}

// seedRemoteCache pre-populates a pinned cache entry for ref so a scaffold
// run resolves it without any network access. httptest.NewServer cannot be
// used here: internal/remote/zip.go rejects any http:// URL outright, and
// ref is a pinned (non-empty version) reference so the resolver consults the
// cache instead of skipping straight to a fetch. Returns the library name
// the scaffold run will derive for this ref.
func seedRemoteCache(t *testing.T, cacheDir, ref, templateDir string) string {
	t.Helper()

	parsed, err := remote.Parse(ref)
	require.NoError(t, err)
	key := parsed.CacheKey()

	cache, err := remote.NewFSCache(cacheDir)
	require.NoError(t, err)

	_, err = cache.Set(key, templateDir, &remote.CacheMeta{
		OriginalRef: ref,
		Version:     parsed.Version,
		FetchedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ExpiresAt:   nil,
	})
	require.NoError(t, err)

	return remote.DeriveName(ref)
}

// snapshotTree records every entry under root (relative path -> sha256 of the
// contents, or "<dir>") so a before/after comparison via assert.Equal catches
// additions, removals, in-place rewrites AND bare directory creation. A root
// that does not exist yet snapshots as empty.
//
// It deliberately does NOT observe permission bits or symlink identity: no
// code path reachable under --no-library mutates either, so recording them
// would be speculative. Widen it here if that stops being true.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := make(map[string]string)
	if _, statErr := os.Stat(root); os.IsNotExist(statErr) {
		return snap
	}
	require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if rel, relErr := filepath.Rel(root, path); relErr == nil && rel != "." {
				snap[rel+"/"] = "<dir>"
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		sum := sha256.Sum256(data)
		snap[rel] = hex.EncodeToString(sum[:])
		return nil
	}))
	return snap
}

// dirEntryNames lists the names of entries directly inside dir, sorted. A
// missing dir reports as empty rather than failing, matching snapshotTree.
func dirEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// tagConfigTemplate reads the template origin block a scaffold run wrote into
// the generated project's .tagconfig.json.
func tagConfigTemplate(t *testing.T, projectRoot string) config.TemplateOrigin {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectRoot, ".tagconfig.json"))
	require.NoError(t, err)

	var cfg config.Config
	require.NoError(t, json.Unmarshal(data, &cfg))
	require.NotNil(t, cfg.Template, "scaffold must always write a template origin block")
	return *cfg.Template
}

// noLibrarySandbox isolates every piece of global state a scaffold run can
// touch: HOME (replay.Save resolves os.UserHomeDir() directly), the library
// data dir (XDG_DATA_HOME), the replay dir, and the remote cache dir.
type noLibrarySandbox struct {
	workDir   string
	xdgData   string
	replayDir string
	cacheDir  string
	homeDir   string
}

func newNoLibrarySandbox(t *testing.T) noLibrarySandbox {
	t.Helper()
	root := t.TempDir()
	s := noLibrarySandbox{
		workDir:   filepath.Join(root, "work"),
		xdgData:   filepath.Join(root, "xdg-data"),
		replayDir: filepath.Join(root, "replay"),
		cacheDir:  filepath.Join(root, "cache"),
		homeDir:   filepath.Join(root, "home"),
	}
	require.NoError(t, os.MkdirAll(s.workDir, 0o755))
	require.NoError(t, os.MkdirAll(s.homeDir, 0o755))
	return s
}

func (s noLibrarySandbox) env() []string {
	return []string{
		"HOME=" + s.homeDir,
		"XDG_DATA_HOME=" + s.xdgData,
		"TAG_REPLAY_DIR=" + s.replayDir,
		"TAG_CACHE_DIR=" + s.cacheDir,
	}
}

func (s noLibrarySandbox) libraryDataDir() string {
	return filepath.Join(s.xdgData, "tag")
}

// TestIT_Scaffold_NoLibrary_LeavesGlobalStateAlone exercises B1-B5 from
// ticket #391: a remote scaffold with --no-library must not touch the shared
// template library or the replay store, regardless of flag position, while
// still copying generators into the project so it stays self-contained.
func TestIT_Scaffold_NoLibrary_LeavesGlobalStateAlone(t *testing.T) {
	t.Run("B1 characterization guard: no flag adds to library and skips generator copy", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		name := seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		lib := library.NewLocal(sandbox.libraryDataDir())
		_, getErr := lib.Get(name)
		assert.NoError(t, getErr, "without the flag, the template must still be added to the library")

		_, statErr := os.Stat(filepath.Join(outDir, ".tag", "nolibgen"))
		assert.True(t, os.IsNotExist(statErr), "without the flag, generators must NOT be copied once the library add succeeds")
	})

	t.Run("B2 --no-library before positionals leaves library and replay untouched", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		libBefore := snapshotTree(t, sandbox.libraryDataDir())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Deliberately WITHOUT --no-save: replay writes are gated by --no-save,
		// not by --no-library, so this asserts only what --no-library alone
		// promises. B4 covers the two flags together.
		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", "--no-library", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		assert.Equal(t, libBefore, snapshotTree(t, sandbox.libraryDataDir()))
	})

	t.Run("B3 --no-library after positionals leaves library and replay untouched", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		libBefore := snapshotTree(t, sandbox.libraryDataDir())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input", "--no-library")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		assert.Equal(t, libBefore, snapshotTree(t, sandbox.libraryDataDir()))
	})

	t.Run("B4 --no-save --no-library: only the project dir and .tag/lock.json are new under workDir", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		libBefore := snapshotTree(t, sandbox.libraryDataDir())
		replayBefore := snapshotTree(t, sandbox.replayDir)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input", "--no-save", "--no-library")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		// This listing passes with or without the --no-library fix -- it is a
		// characterization guard over the pre-existing lockfile write below,
		// not regression coverage for the flag. The library/replay snapshots
		// at the end of this subtest are what detect a regression.
		assert.Equal(t, []string{".tag", "project"}, dirEntryNames(t, sandbox.workDir),
			"the only new top-level entries under workDir must be the project dir and .tag/")

		// verifyTemplateLock's .tag/lock.json write is a KNOWN, PRE-EXISTING,
		// OUT-OF-SCOPE side effect (scaffold.go's verifyTemplateLock ->
		// lockfile.VerifyAndMaybeUpdate) that fires on every remote scaffold's
		// first use, independent of --no-library. If this assertion ever
		// fails because MORE than lock.json appears here, that write moved
		// and needs its own decision — do not silence this by adding
		// --ignore-lock.
		assert.Equal(t, []string{"lock.json"}, dirEntryNames(t, filepath.Join(sandbox.workDir, ".tag")))

		assert.Equal(t, libBefore, snapshotTree(t, sandbox.libraryDataDir()))
		assert.Equal(t, replayBefore, snapshotTree(t, sandbox.replayDir))
	})

	// B6 pins the READ half of the flag's contract. Suppressing the library
	// write is not enough on its own: .tagconfig.json's template.name drives
	// library-FIRST generator resolution (config.HasTemplateOrigin ->
	// resolveGeneratorPaths), so a recorded name would let an unrelated
	// template that happens to share the derived basename win over the
	// generators copied into this project -- reinstating exactly the silent
	// cross-org collision this flag exists to prevent.
	t.Run("B6 --no-library records no library name in .tagconfig.json", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input", "--no-save", "--no-library")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		assert.Empty(t, tagConfigTemplate(t, outDir).Name,
			"--no-library must not record a library name to resolve generators from")

		// Positive control: without the flag the name IS recorded, so the
		// assertion above cannot pass merely because the field moved or the
		// fixture stopped producing a template origin at all.
		control := newNoLibrarySandbox(t)
		seedRemoteCache(t, control.cacheDir, noLibraryFixtureRef, noLibraryFixtureTemplate(t))
		controlOut := filepath.Join(control.workDir, "project")
		stdout, stderr, err = runTagSubprocessEnv(t, ctx, control.workDir, control.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", controlOut, "--no-input", "--no-save")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		assert.NotEmpty(t, tagConfigTemplate(t, controlOut).Name)
	})

	t.Run("B5 --no-library copies generators into the project", func(t *testing.T) {
		sandbox := newNoLibrarySandbox(t)
		templateSrc := noLibraryFixtureTemplate(t)
		seedRemoteCache(t, sandbox.cacheDir, noLibraryFixtureRef, templateSrc)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		outDir := filepath.Join(sandbox.workDir, "project")
		stdout, stderr, err := runTagSubprocessEnv(t, ctx, sandbox.workDir, sandbox.env(),
			"scaffold", noLibraryFixtureRef, "generated", "--output", outDir, "--no-input", "--no-save", "--no-library")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		content, readErr := os.ReadFile(filepath.Join(outDir, ".tag", "nolibgen", "generator.go"))
		require.NoError(t, readErr)
		assert.Equal(t, noLibraryGeneratorContent, string(content))
	})
}

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/remote"
)

func minimalLocalTemplate(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	cfg := `{
  "name": "env-isolation-template",
  "version": "1.0.0",
  "vars": {
    "project_name": {"type": "string", "default": "demo"}
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(cfg), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"),
		[]byte("# {{ vars.project_name }}\n"), 0o644))
	return dir
}

// TestIT_Scaffold_LocalTemplateSucceedsWithBrokenHome is the launch-blocker
// regression from the epic: a container/sandbox HOME that cannot be written
// to must not stop a local-template scaffold, and TAG_REPLAY_DIR must be
// able to route around it independently of TAG_CACHE_DIR.
func TestIT_Scaffold_LocalTemplateSucceedsWithBrokenHome(t *testing.T) {
	templateDir := minimalLocalTemplate(t)

	t.Run("broken HOME alone, replay warns but scaffold succeeds", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		workDir := t.TempDir()
		outDir := filepath.Join(workDir, "out")

		stdout, stderr, err := runTagSubprocessEnv(t, ctx, workDir,
			[]string{"HOME=/nonexistent", "TAG_REPLAY_DIR", "TAG_CACHE_DIR"},
			"scaffold", templateDir, "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		content, readErr := os.ReadFile(filepath.Join(outDir, "README.md"))
		require.NoError(t, readErr)
		assert.Equal(t, "# demo\n", string(content))

		assert.NoDirExists(t, "/nonexistent")

		combined := string(stdout) + string(stderr)
		assert.Contains(t, combined, "Warning: failed to save replay data")
	})

	t.Run("broken HOME with TAG_REPLAY_DIR set, no warning and replay file written", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		workDir := t.TempDir()
		outDir := filepath.Join(workDir, "out")
		replayDir := filepath.Join(workDir, "replay")

		stdout, stderr, err := runTagSubprocessEnv(t, ctx, workDir,
			[]string{"HOME=/nonexistent", "TAG_REPLAY_DIR=" + replayDir, "TAG_CACHE_DIR"},
			"scaffold", templateDir, "--output", outDir, "--no-input")
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)

		combined := string(stdout) + string(stderr)
		assert.NotContains(t, combined, "Warning: failed to save replay data")

		entries, readErr := os.ReadDir(replayDir)
		require.NoError(t, readErr)
		found := false
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".json") {
				found = true
			}
		}
		assert.True(t, found, "expected a replay .json file under TAG_REPLAY_DIR")

		assert.NoDirExists(t, "/nonexistent")
	})
}

// TestIT_CacheLs_HonoursCacheDirEnv exercises `tag cache ls` through the real
// binary so the assertion covers main.go's actual wiring, not just the
// remote package in isolation.
func TestIT_CacheLs_HonoursCacheDirEnv(t *testing.T) {
	t.Run("seeded cache dir lists its entries", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		workDir := t.TempDir()
		isolatedHome := t.TempDir()
		cacheDir := filepath.Join(workDir, "cache")
		entryDir := filepath.Join(cacheDir, "my-template-key")
		require.NoError(t, os.MkdirAll(entryDir, 0o750))

		meta := remote.CacheMeta{
			Version:   "v1.0.0",
			FetchedAt: time.Now(),
		}
		metaData, err := json.Marshal(meta)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(entryDir, "_meta.json"), metaData, 0o644))

		stdout, stderr, runErr := runTagSubprocessEnv(t, ctx, workDir,
			[]string{"TAG_CACHE_DIR=" + cacheDir, "HOME=" + isolatedHome},
			"cache", "ls")
		require.NoError(t, runErr, "stdout=%s stderr=%s", stdout, stderr)
		assert.Contains(t, string(stdout), "my-template-key")
	})

	// The absent-dir subtest must isolate HOME. Its only feature-specific
	// assertion is the empty listing, and a pre-#389 binary against an empty
	// ~/.tag/cache prints exactly the same bytes — so without an isolated HOME
	// this passes on a reverted tree on any machine with a cold cache, which
	// is the default on CI.
	t.Run("absent cache dir prints empty and creates nothing", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		workDir := t.TempDir()
		isolatedHome := t.TempDir()
		cacheDir := filepath.Join(workDir, "does-not-exist", "cache")

		seededHomeCache := filepath.Join(isolatedHome, ".tag", "cache", "home-cache-entry")
		require.NoError(t, os.MkdirAll(seededHomeCache, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(seededHomeCache, "_meta.json"),
			[]byte(`{"original_ref":"gh:x/y","fetched_at":"2026-01-01T00:00:00Z"}`), 0o644))

		stdout, stderr, runErr := runTagSubprocessEnv(t, ctx, workDir,
			[]string{"TAG_CACHE_DIR=" + cacheDir, "HOME=" + isolatedHome},
			"cache", "ls")
		require.NoError(t, runErr, "stdout=%s stderr=%s", stdout, stderr)
		assert.Equal(t, "No cached templates.\n", string(stdout))

		assert.NoDirExists(t, cacheDir)
		assert.DirExists(t, seededHomeCache)
	})

	// Proves runTagSubprocessEnv's bare-"KEY" force-unset actually removes an
	// inherited value: the parent process exports TAG_CACHE_DIR pointing at a
	// seeded cache, and the child must not see it.
	t.Run("force-unset drops an inherited TAG_CACHE_DIR", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		workDir := t.TempDir()
		isolatedHome := t.TempDir()
		inheritedCache := filepath.Join(workDir, "inherited")
		entryDir := filepath.Join(inheritedCache, "inherited-key")
		require.NoError(t, os.MkdirAll(entryDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(entryDir, "_meta.json"),
			[]byte(`{"original_ref":"gh:x/y","fetched_at":"2026-01-01T00:00:00Z"}`), 0o644))

		t.Setenv("TAG_CACHE_DIR", inheritedCache)

		seen, stderr, runErr := runTagSubprocessEnv(t, ctx, workDir,
			[]string{"TAG_CACHE_DIR=" + inheritedCache, "HOME=" + isolatedHome},
			"cache", "ls")
		require.NoError(t, runErr, "stderr=%s", stderr)
		require.Contains(t, string(seen), "inherited-key", "control: the child sees it when passed")

		hidden, stderr, runErr := runTagSubprocessEnv(t, ctx, workDir,
			[]string{"TAG_CACHE_DIR", "HOME=" + isolatedHome},
			"cache", "ls")
		require.NoError(t, runErr, "stderr=%s", stderr)
		assert.Equal(t, "No cached templates.\n", string(hidden))
	})

	t.Run("relative TAG_CACHE_DIR fails loudly at the CLI boundary", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		workDir := t.TempDir()

		stdout, stderr, runErr := runTagSubprocessEnv(t, ctx, workDir,
			[]string{"TAG_CACHE_DIR=relative/cache", "HOME=" + t.TempDir()},
			"cache", "ls")
		require.Error(t, runErr, "stdout=%s", stdout)
		assert.Contains(t, string(stdout)+string(stderr), "TAG_CACHE_DIR")
		assert.NoDirExists(t, filepath.Join(workDir, "relative"))
	})
}

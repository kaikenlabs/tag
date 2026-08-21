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
			[]string{"TAG_CACHE_DIR=" + cacheDir},
			"cache", "ls")
		require.NoError(t, runErr, "stdout=%s stderr=%s", stdout, stderr)
		assert.Contains(t, string(stdout), "my-template-key")
	})

	t.Run("absent cache dir prints empty and creates nothing", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		workDir := t.TempDir()
		cacheDir := filepath.Join(workDir, "does-not-exist", "cache")

		stdout, stderr, runErr := runTagSubprocessEnv(t, ctx, workDir,
			[]string{"TAG_CACHE_DIR=" + cacheDir},
			"cache", "ls")
		require.NoError(t, runErr, "stdout=%s stderr=%s", stdout, stderr)
		assert.Equal(t, "No cached templates.\n", string(stdout))

		assert.NoDirExists(t, cacheDir)
	})
}

package commands

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/remote"
)

// populateCache creates a cache directory with a fake cached entry.
func populateCache(t *testing.T, cacheDir, key string) {
	t.Helper()

	entryDir := filepath.Join(cacheDir, key)
	require.NoError(t, os.MkdirAll(entryDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(entryDir, "README.md"), []byte("cached template"), 0o644))

	meta := remote.CacheMeta{
		Version:   "v1.0.0",
		FetchedAt: time.Now(),
	}
	metaData, err := json.Marshal(meta)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(entryDir, ".meta.json"), metaData, 0o644))
}

func TestUT_CacheListAction_EmptyCache(t *testing.T) {
	// Uses t.Setenv — do NOT use t.Parallel()
	homeDir := t.TempDir()
	cacheDir := filepath.Join(homeDir, ".tag", "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o750))

	t.Setenv("HOME", homeDir)

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := cacheListCommand()
	err := cmd.Action(ctx)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "No cached templates")
}

func TestUT_CacheListAction_WithEntries(t *testing.T) {
	// Uses t.Setenv — do NOT use t.Parallel()
	homeDir := t.TempDir()
	cacheDir := filepath.Join(homeDir, ".tag", "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o750))

	populateCache(t, cacheDir, "gh-user-template")

	t.Setenv("HOME", homeDir)

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := cacheListCommand()
	err := cmd.Action(ctx)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "KEY")
	assert.Contains(t, out, "gh-user-template")
}

func TestUT_CacheClearAction_All(t *testing.T) {
	// Uses t.Setenv — do NOT use t.Parallel()
	homeDir := t.TempDir()
	cacheDir := filepath.Join(homeDir, ".tag", "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o750))

	populateCache(t, cacheDir, "gh-cached-entry")

	t.Setenv("HOME", homeDir)

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	allFlag := &cli.BoolFlag{Name: "all"}
	require.NoError(t, allFlag.Apply(set))
	require.NoError(t, set.Set("all", "true"))
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := cacheClearCommand()
	err := cmd.Action(ctx)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Removed")

	// Verify cache is empty
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "cache should be empty after clear --all")
}

func TestUT_CacheClearAction_ExpiredOnly(t *testing.T) {
	// Uses t.Setenv — do NOT use t.Parallel()
	homeDir := t.TempDir()
	cacheDir := filepath.Join(homeDir, ".tag", "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o750))

	entryDir := filepath.Join(cacheDir, "expired-entry")
	require.NoError(t, os.MkdirAll(entryDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(entryDir, "file.txt"), []byte("old"), 0o644))

	expired := time.Now().Add(-24 * time.Hour)
	meta := remote.CacheMeta{
		FetchedAt: time.Now().Add(-48 * time.Hour),
		ExpiresAt: &expired,
	}
	metaData, err := json.Marshal(meta)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(entryDir, ".meta.json"), metaData, 0o644))

	t.Setenv("HOME", homeDir)

	var buf bytes.Buffer
	cliApp := &cli.App{Writer: &buf}
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	allFlag := &cli.BoolFlag{Name: "all"}
	require.NoError(t, allFlag.Apply(set))
	ctx := cli.NewContext(cliApp, set, nil)

	cmd := cacheClearCommand()
	err = cmd.Action(ctx)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Removed")
}

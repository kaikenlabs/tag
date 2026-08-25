package fileutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ResolveSymlinkedRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	t.Run("non-symlink root returned verbatim", func(t *testing.T) {
		got, err := ResolveSymlinkedRoot("./tmpl")
		require.NoError(t, err)
		assert.Equal(t, "./tmpl", got, "an uncleaned non-symlink root must pass through unchanged")
	})

	t.Run("symlinked root resolves to the real directory", func(t *testing.T) {
		realDir := t.TempDir()
		link := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(realDir, link))
		require.NotEqual(t, realDir, link, "fixture must actually be a symlink")

		resolvedReal, evalErr := filepath.EvalSymlinks(realDir)
		require.NoError(t, evalErr)

		got, err := ResolveSymlinkedRoot(link)
		require.NoError(t, err)
		assert.Equal(t, resolvedReal, got)
	})

	t.Run("dangling symlink errors", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "does-not-exist"), link))

		_, err := ResolveSymlinkedRoot(link)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve template root")
		assert.Contains(t, err.Error(), link)
	})

	t.Run("symlink to a file resolves without judging type", func(t *testing.T) {
		realFile := filepath.Join(t.TempDir(), "file.txt")
		require.NoError(t, os.WriteFile(realFile, []byte("x"), 0o644))
		link := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(realFile, link))
		require.NotEqual(t, realFile, link, "fixture must actually be a symlink")

		resolvedReal, evalErr := filepath.EvalSymlinks(realFile)
		require.NoError(t, evalErr)

		got, err := ResolveSymlinkedRoot(link)
		require.NoError(t, err)
		assert.Equal(t, resolvedReal, got)
	})

	t.Run("intermediate symlink component is not resolved", func(t *testing.T) {
		base := t.TempDir()
		realSub := filepath.Join(base, "real")
		require.NoError(t, os.MkdirAll(realSub, 0o755))
		link := filepath.Join(base, "link")
		require.NoError(t, os.Symlink(realSub, link))
		require.NotEqual(t, realSub, link, "fixture must actually be a symlink")

		arg := filepath.Join(link, "child")
		got, err := ResolveSymlinkedRoot(arg)
		require.NoError(t, err)
		assert.Equal(t, arg, got, "only a symlinked FINAL component should be resolved")
	})
}

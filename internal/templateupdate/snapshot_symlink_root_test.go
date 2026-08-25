package templateupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUT_ReadProjectFiles_SymlinkedRoot_MatchesDirectRoot pins #424 site 2.
// Measured pre-fix: the direct project dir yielded 1 file and the symlinked
// one yielded 0 with a nil error, which makes the 3-way merge behind
// `tag update --dir` / `tag diff --dir` see every file as deleted.
func TestUT_ReadProjectFiles_SymlinkedRoot_MatchesDirectRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	realDir := filepath.Join(base, "project")
	require.NoError(t, os.MkdirAll(filepath.Join(realDir, "nested"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(realDir, ".tag"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "README.md"), []byte("ticket-424\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "nested", "main.go"), []byte("package main\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, ".tagconfig.json"), []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, ".tag", "history.json"), []byte("{}"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(realDir, "README.md"), filepath.Join(realDir, "loose")))

	link := filepath.Join(base, "linked")
	require.NoError(t, os.Symlink(realDir, link))
	resolved, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolved, "fixture must actually be a symlink")

	// Positive control: the literal expected payload of the direct run. Two
	// empty maps compare equal, so a bare direct-vs-link assertion would pass
	// on the broken tree.
	direct, err := ReadProjectFiles(realDir)
	require.NoError(t, err)
	require.Equal(t, []string{"README.md", "nested/main.go"}, sortedKeys(direct), "positive control")
	require.Equal(t, []byte("ticket-424\n"), direct["README.md"].Content, "positive control")

	viaLink, err := ReadProjectFiles(link)
	require.NoError(t, err)

	// The exact key list is what proves both halves at once: the tree is seen,
	// and .tag/, .tagconfig.json and the inner symlink are still skipped.
	require.Equal(t, []string{"README.md", "nested/main.go"}, sortedKeys(viaLink))
	assert.Equal(t, []byte("ticket-424\n"), viaLink["README.md"].Content)
	assert.Equal(t, direct, viaLink)
}

// TestUT_ReadProjectFiles_SymlinkedRoot_AgreesWithIgnoreMatcher covers the
// asymmetry the fix introduces at this site: Differ and Updater hand the
// UNRESOLVED projectDir to NewIgnoreMatcher while ReadProjectFiles now walks
// the resolved one. #419's lesson is that a declaration reader and a tree
// walker disagreeing about the same root is what turns "produces nothing" into
// "produces a confident wrong answer", so the agreement is pinned rather than
// assumed.
func TestUT_ReadProjectFiles_SymlinkedRoot_AgreesWithIgnoreMatcher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	realDir := filepath.Join(base, "project")
	require.NoError(t, os.MkdirAll(realDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "keep.txt"), []byte("keep\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "skip.log"), []byte("skip\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, ".tagignore"), []byte("*.log\n"), 0o600))

	link := filepath.Join(base, "linked")
	require.NoError(t, os.Symlink(realDir, link))

	files, err := ReadProjectFiles(link)
	require.NoError(t, err)
	require.Equal(t, []string{"keep.txt", "skip.log"}, sortedKeys(files), "positive control")

	matcher, err := NewIgnoreMatcher(IgnoreMatcherOptions{ProjectRoot: link})
	require.NoError(t, err)

	assert.True(t, matcher.ShouldSkip("skip.log", false),
		"the matcher must read .tagignore through the unresolved root the callers still pass it")
	assert.False(t, matcher.ShouldSkip("keep.txt", false))
}

func sortedKeys(m map[string]*RenderedFile) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

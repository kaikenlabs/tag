package library

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/kaikenlabs/tag/internal/remote"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUT_Add_SymlinkedLocalRoot_CopiesPayload pins #424 site 4. Measured
// pre-fix against a build: `tag lib add ./linktpl --as vialink` exited 0,
// printed the success banner and the library path, and stored an EMPTY
// template directory — fileutil.CopyDir walks the source with WalkDir, which
// does not descend into a symlinked root.
//
// The resolve deliberately lives in Add and not in CopyDir: FSCache.Set feeds
// CopyDir fetched remote content, where a repository can commit its subpath as
// a symlink pointing outside the tree, so following an arbitrary root there
// would copy the target into the cache.
func TestUT_Add_SymlinkedLocalRoot_CopiesPayload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	realDir := filepath.Join(base, "tmpl")
	require.NoError(t, os.MkdirAll(filepath.Join(realDir, "nested"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "tag.template.json"),
		[]byte(`{"name":"t","description":"d"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(realDir, "nested", "file.txt"),
		[]byte("ticket-424\n"), 0o600))

	link := filepath.Join(base, "linked")
	require.NoError(t, os.Symlink(realDir, link))
	resolved, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolved, "fixture must actually be a symlink")

	lib, err := New(filepath.Join(base, "data"))
	require.NoError(t, err)
	ctx := context.Background()

	// Positive control: an empty stored tree compares equal to another empty
	// stored tree, so the direct add's literal payload is asserted first.
	direct, err := lib.Add(ctx, AddOptions{Ref: realDir, Name: "direct"})
	require.NoError(t, err)
	require.Equal(t, []string{"nested/file.txt", "tag.template.json"},
		storedTree(t, direct.TemplateDir), "positive control")
	require.Equal(t, "ticket-424\n",
		readStored(t, direct.TemplateDir, "nested/file.txt"), "positive control")

	viaLink, err := lib.Add(ctx, AddOptions{Ref: link, Name: "vialink"})
	require.NoError(t, err)

	assert.Equal(t, storedTree(t, direct.TemplateDir), storedTree(t, viaLink.TemplateDir))
	assert.Equal(t, "ticket-424\n", readStored(t, viaLink.TemplateDir, "nested/file.txt"))
	assert.Equal(t, link, viaLink.Source, "the recorded source must stay the ref the user typed")
}

func storedTree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	require.NoError(t, filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	}))
	sort.Strings(out)
	return out
}

func readStored(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	require.NoError(t, err)
	return string(data)
}

// fakeResolver returns a fixed path, standing in for a remote fetch.
type fakeResolver struct{ path string }

func (f fakeResolver) Resolve(context.Context, string, remote.ResolveOptions) (*remote.FetchResult, error) {
	return &remote.FetchResult{Path: f.path}, nil
}

// TestUT_Add_RemoteSymlinkedRootIsRefused pins the local/remote split. A local
// ref's root is resolved because the user named it on their own filesystem; a
// fetched one is not, because a repository can commit its subpath as a symlink
// pointing anywhere and following it would copy the target into the library.
// Resolve returns the raw fetched path whenever cache publication fails, so a
// symlinked fetched root is reachable — it is refused rather than followed.
func TestUT_Add_RemoteSymlinkedRootIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret\n"), 0o600))

	fetched := filepath.Join(base, "fetched")
	require.NoError(t, os.Symlink(outside, fetched))

	dataDir := filepath.Join(base, "data")
	lib := &Library{store: newStore(dataDir), dataDir: dataDir, resolver: fakeResolver{path: fetched}}

	_, err = lib.Add(context.Background(), AddOptions{Ref: "gh:owner/repo/link", Name: "remote"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetched template root is a symlink")
	assert.NoFileExists(t, filepath.Join(dataDir, templatesDir, "remote", "secret.txt"),
		"the link target must never be copied into the library")

	// Positive control: the same fetched path, not a symlink, still installs.
	realFetched := filepath.Join(base, "fetched-real")
	require.NoError(t, os.MkdirAll(realFetched, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(realFetched, "tag.template.json"),
		[]byte(`{"name":"t","description":"d"}`), 0o600))

	lib.resolver = fakeResolver{path: realFetched}
	ok, err := lib.Add(context.Background(), AddOptions{Ref: "gh:owner/repo", Name: "remote-ok"})
	require.NoError(t, err)
	assert.Equal(t, []string{"tag.template.json"}, storedTree(t, ok.TemplateDir))
}

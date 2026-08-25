package convert

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSymlinkRootFixture builds a minimal Cookiecutter template under root:
// one templated directory holding one file, plus a loose symlink that the
// walk must keep skipping (with its warning) after the root-resolution fix.
func writeSymlinkRootFixture(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "{{cookiecutter.project_slug}}"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cookiecutter.json"),
		[]byte(`{"project_slug": "demo"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "{{cookiecutter.project_slug}}", "README.md"),
		[]byte("ticket-424\n"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(root, "cookiecutter.json"), filepath.Join(root, "loose")))
}

// listTree returns every regular file under dir as sorted slash-separated
// relative paths. Asserting the whole listing (rather than NotContains on a
// single name) is what makes these tests fail on the broken tree: before the
// fix the linked run produced an empty tree, and any "must not contain X"
// assertion is vacuously true against nothing.
func listTree(t *testing.T, dir string) []string {
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

// symlinkedFixtureRoots builds the fixture once and returns the realDir root and
// a sibling symlink pointing at it. The symlink is created explicitly: relying
// on macOS t.TempDir() resolving /var -> /private/var makes the test a silent
// no-op on Linux, the only OS this repo's CI runs.
func symlinkedFixtureRoots(t *testing.T) (base, realDir, link string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	realDir = filepath.Join(base, "cc")
	writeSymlinkRootFixture(t, realDir)

	link = filepath.Join(base, "linked")
	require.NoError(t, os.Symlink(realDir, link))
	resolved, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolved, "fixture must actually be a symlink")

	return base, realDir, link
}

// TestUT_Convert_SymlinkedRoot_MatchesDirectRoot pins #424 site 1. Measured on
// a pre-fix build: `tag convert cookiecutter ./linked -o out` printed
// "Files processed: 0" at exit 0 and wrote only tag.template.json, while the
// direct root wrote the full tree.
func TestUT_Convert_SymlinkedRoot_MatchesDirectRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	base, realDir, link := symlinkedFixtureRoots(t)

	c, err := NewConverter()
	require.NoError(t, err)
	ctx := context.Background()

	outDirect := filepath.Join(base, "out-direct")
	direct, err := c.Convert(ctx, Options{Source: realDir, Destination: outDirect})
	require.NoError(t, err)

	// Positive control: two empty results compare equal, so the direct run's
	// literal payload has to be asserted before any differential comparison.
	require.Equal(t, 1, direct.FilesProcessed, "positive control")
	require.Equal(t, []PathConversion{{
		From: "{{cookiecutter.project_slug}}/README.md",
		To:   "{{vars.project_slug}}/README.md",
	}}, direct.Files, "positive control")
	require.Equal(t, []string{
		"tag.template.json",
		"{{vars.project_slug}}/README.md",
	}, listTree(t, outDirect), "positive control")
	require.Equal(t, "ticket-424\n", readFileString(t, filepath.Join(
		outDirect, "{{vars.project_slug}}", "README.md")), "positive control")

	outLink := filepath.Join(base, "out-link")
	viaLink, err := c.Convert(ctx, Options{Source: link, Destination: outLink})
	require.NoError(t, err)

	assert.Equal(t, direct.FilesProcessed, viaLink.FilesProcessed)
	assert.Equal(t, direct.DirsRenamed, viaLink.DirsRenamed)
	assert.Equal(t, direct.FilesRenamed, viaLink.FilesRenamed)
	assert.Equal(t, direct.Files, viaLink.Files)
	assert.Equal(t, listTree(t, outDirect), listTree(t, outLink))
	assert.Equal(t, "ticket-424\n", readFileString(t, filepath.Join(
		outLink, "{{vars.project_slug}}", "README.md")))

	// A symlink INSIDE the tree keeps its existing handling: skipped, warned.
	assert.Contains(t, viaLink.Warnings, "skipped symlink: loose (symlinks not copied for security)")
	assert.Equal(t, direct.Warnings, viaLink.Warnings)
}

// TestUT_Convert_SymlinkedRoot_PreservesDefaultDestination is why the resolve
// call sits in processTemplateFiles and not in resolveSource: Convert derives
// the default destination from filepath.Base(templateDir), so resolving
// earlier would silently turn `./linked` into `cc-tag`. This passes on both
// sides of the fix — it discriminates against the rejected placement, not
// against a revert.
func TestUT_Convert_SymlinkedRoot_PreservesDefaultDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	_, _, link := symlinkedFixtureRoots(t)

	c, err := NewConverter()
	require.NoError(t, err)

	res, err := c.Convert(context.Background(), Options{Source: link, DryRun: true})
	require.NoError(t, err)

	assert.Equal(t, "linked-tag", res.Destination,
		"the default destination must follow the name the user typed, not the link target")
	assert.Equal(t, 1, res.FilesProcessed, "the dry-run walk must see the tree too")
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

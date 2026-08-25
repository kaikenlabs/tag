package vars

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const renameSymlinkConfig = `{
  "name": "demo",
  "vars": {
    "old_name": { "type": "string", "prompt": "Old" }
  }
}`

// TestUT_PlanRename_SymlinkedRoot_MatchesDirectRoot pins #419 for
// `tag template rename-var`: filepath.WalkDir does not descend into a
// symlinked root, so PlanRename's walk found nothing through a symlink and
// reported "No references found ... — nothing to rename." at exit 0. The
// direct-root plan is asserted as a non-empty positive control first, then
// compared to the plan built through the symlink (using FileCount/
// ReplacementCount rather than the whole *RenamePlan — its Root field holds
// an absolute path that legitimately differs by platform).
func TestUT_PlanRename_SymlinkedRoot_MatchesDirectRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	realDir := t.TempDir()
	writeTree(t, realDir, map[string]string{
		"tag.template.json": renameSymlinkConfig,
		"README.md":         "# {{ vars.old_name }}\nHello {{ vars.old_name }}",
	})

	directPlan, err := PlanRename(realDir, "old_name", "new_name")
	require.NoError(t, err)
	require.Positive(t, directPlan.FileCount(), "positive control: direct root must find files to rename")
	require.Positive(t, directPlan.ReplacementCount(), "positive control")

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, link))
	resolvedLink, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolvedLink, "fixture must actually be a symlink")

	linkedPlan, err := PlanRename(link, "old_name", "new_name")
	require.NoError(t, err)

	assert.Equal(t, directPlan.FileCount(), linkedPlan.FileCount())
	assert.Equal(t, directPlan.ReplacementCount(), linkedPlan.ReplacementCount())

	var directPaths, linkedPaths []string
	for _, f := range directPlan.Files {
		directPaths = append(directPaths, f.Path)
	}
	for _, f := range linkedPlan.Files {
		linkedPaths = append(linkedPaths, f.Path)
	}
	assert.Equal(t, directPaths, linkedPaths)
}

// TestUT_PlanRename_SymlinkedRoot_ApplyRewritesRealTreeBytes proves the
// rewrite lands on the REAL files (not silently no-oping), by checking exact
// byte content before and after plus a full before/after tree listing.
func TestUT_PlanRename_SymlinkedRoot_ApplyRewritesRealTreeBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	realDir := t.TempDir()
	writeTree(t, realDir, map[string]string{
		"tag.template.json": renameSymlinkConfig,
		"README.md":         "# {{ vars.old_name }}\nHello {{ vars.old_name }}",
	})

	beforeTree := snapshotTree(t, realDir)
	require.Contains(t, beforeTree["README.md"], "vars.old_name")

	linkParent := t.TempDir()
	link := filepath.Join(linkParent, "link")
	require.NoError(t, os.Symlink(realDir, link))

	plan, err := PlanRename(link, "old_name", "new_name")
	require.NoError(t, err)
	require.NoError(t, plan.Apply())

	afterTree := snapshotTree(t, realDir)
	assert.Equal(t, len(beforeTree), len(afterTree), "apply must not add or remove files in the real tree")
	assert.Contains(t, afterTree["README.md"], "vars.new_name")
	assert.NotContains(t, afterTree["README.md"], "vars.old_name")
	assert.Equal(t, "# {{ vars.new_name }}\nHello {{ vars.new_name }}", afterTree["README.md"])

	afterConfig := afterTree["tag.template.json"]
	assert.Contains(t, afterConfig, `"new_name"`)
	assert.NotContains(t, afterConfig, `"old_name"`)

	// Over-correction guard: the link itself must still be a symlink
	// pointing at the same target, and nothing new must appear beside it.
	linkInfo, lstatErr := os.Lstat(link)
	require.NoError(t, lstatErr)
	assert.NotZero(t, linkInfo.Mode()&os.ModeSymlink, "the rename must not have replaced the symlink itself")

	target, readErr := os.Readlink(link)
	require.NoError(t, readErr)
	assert.Equal(t, realDir, target)

	entries, dirErr := os.ReadDir(linkParent)
	require.NoError(t, dirErr)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	assert.Equal(t, []string{"link"}, names, "nothing else must be written beside the symlink")
}

// TestUT_PlanRename_SymlinkedRoot_StillSkipsInteriorSymlink is a three-way
// discriminator: it fails on the pre-#419 tree (nothing is walked, so
// real.txt is never found either), and it fails on an over-corrected walk
// that follows interior symlinks (linked.txt would then be renamed too). It
// passes only when the root is resolved and interior symlinks keep today's
// skip behaviour.
func TestUT_PlanRename_SymlinkedRoot_StillSkipsInteriorSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	realDir := t.TempDir()
	writeTree(t, realDir, map[string]string{
		"tag.template.json": renameSymlinkConfig,
		"real.txt":          "{{ vars.old_name }}",
	})

	externalFile := filepath.Join(t.TempDir(), "external.txt")
	require.NoError(t, os.WriteFile(externalFile, []byte("{{ vars.old_name }}"), 0o644))
	linkedPath := filepath.Join(realDir, "linked.txt")
	require.NoError(t, os.Symlink(externalFile, linkedPath))
	resolvedLinked, evalErr := filepath.EvalSymlinks(linkedPath)
	require.NoError(t, evalErr)
	require.NotEqual(t, linkedPath, resolvedLinked, "fixture must actually be a symlink")

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, link))

	plan, err := PlanRename(link, "old_name", "new_name")
	require.NoError(t, err)

	var paths []string
	for _, f := range plan.Files {
		paths = append(paths, f.Path)
	}
	assert.NotContains(t, paths, "linked.txt", "interior symlinks must still be skipped")
	assert.Contains(t, paths, "real.txt")

	externalContent, readErr := os.ReadFile(externalFile)
	require.NoError(t, readErr)
	assert.Equal(t, "{{ vars.old_name }}", string(externalContent), "the external file outside the template must never be touched")
}

// TestUT_PlanRename_SymlinkedRoot_StillSkipsInteriorSymlinkToDirectory is
// the directory-target sibling of the interior-symlink-to-file test above:
// a symlink INSIDE the (now-resolved) template that points at a DIRECTORY
// outside it must not be descended into, and applying the rename must leave
// that external directory's bytes completely unchanged. real.txt must also
// show up in the plan and be rewritten, or the "nothing under linkeddir/"
// assertion below would pass vacuously whether anything at all was walked.
func TestUT_PlanRename_SymlinkedRoot_StillSkipsInteriorSymlinkToDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	realDir := t.TempDir()
	writeTree(t, realDir, map[string]string{
		"tag.template.json": renameSymlinkConfig,
		"real.txt":          "{{ vars.old_name }}",
	})

	externalDir := t.TempDir()
	externalFile := filepath.Join(externalDir, "leak.txt")
	require.NoError(t, os.WriteFile(externalFile, []byte("{{ vars.old_name }}"), 0o644))
	linkedDirPath := filepath.Join(realDir, "linkeddir")
	require.NoError(t, os.Symlink(externalDir, linkedDirPath))
	resolvedLinkedDir, evalErr := filepath.EvalSymlinks(linkedDirPath)
	require.NoError(t, evalErr)
	require.NotEqual(t, linkedDirPath, resolvedLinkedDir, "fixture must actually be a symlink")

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, link))

	plan, err := PlanRename(link, "old_name", "new_name")
	require.NoError(t, err)
	require.NoError(t, plan.Apply())

	var paths []string
	for _, f := range plan.Files {
		paths = append(paths, f.Path)
	}
	assert.NotContains(t, paths, filepath.ToSlash(filepath.Join("linkeddir", "leak.txt")),
		"the symlinked directory must not be descended into")
	require.Contains(t, paths, "real.txt", "real.txt must be found and renamed, proving the template was actually walked")

	realContent, readRealErr := os.ReadFile(filepath.Join(realDir, "real.txt"))
	require.NoError(t, readRealErr)
	assert.Equal(t, "{{ vars.new_name }}", string(realContent))

	externalContent, readErr := os.ReadFile(externalFile)
	require.NoError(t, readErr)
	assert.Equal(t, "{{ vars.old_name }}", string(externalContent), "the external directory's content must remain untouched")
}

// TestUT_PlanRename_DanglingSymlinkRoot_StillReportsPreExistingMessage is a
// placement guard: ResolveSymlinkedRoot must run AFTER the existing
// os.Stat + IsDir validation, so a dangling root keeps producing today's
// "path does not exist" message.
func TestUT_PlanRename_DanglingSymlinkRoot_StillReportsPreExistingMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	dangling := filepath.Join(t.TempDir(), "dangling")
	require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "does-not-exist"), dangling))

	_, err := PlanRename(dangling, "old_name", "new_name")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path does not exist:")
}

// TestUT_PlanRename_SymlinkToFile_StillReportsPreExistingMessage is the
// second placement-guard row: a symlink whose target is a FILE keeps the
// existing "path is not a directory" message.
func TestUT_PlanRename_SymlinkToFile_StillReportsPreExistingMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	realFile := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(realFile, []byte("x"), 0o644))
	fileLink := filepath.Join(t.TempDir(), "filelink")
	require.NoError(t, os.Symlink(realFile, fileLink))

	_, err := PlanRename(fileLink, "old_name", "new_name")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path is not a directory:")
}

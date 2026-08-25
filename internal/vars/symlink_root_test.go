package vars

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUT_Analyze_SymlinkedRoot_MatchesDirectRoot pins #419 for
// `tag template variables`: filepath.WalkDir does not descend into a
// symlinked root, so scanFiles never sees any file and every declared
// variable is reported unused while nothing is ever flagged undeclared. The
// direct-root assertions run FIRST as a positive control — matching the
// measured pre-fix numbers exactly (2 declared, 1 undeclared, 1 unused
// direct vs. 2 declared, 0 undeclared, 2 unused through the symlink) makes
// this a real oracle, not a vacuous "results match" check.
func TestUT_Analyze_SymlinkedRoot_MatchesDirectRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	config := `{"vars": {"project_name": "myapp", "unused_var": {"type": "string", "prompt": "x"}}}`
	realDir := setupTemplate(t, config, map[string]string{
		"README.md": "# {{ vars.project_name }}\n{{ vars.undefined_var }}",
	})

	directReport, err := Analyze(realDir)
	require.NoError(t, err)

	require.Equal(t, 2, directReport.Root.Summary.Declared, "positive control")
	require.Equal(t, 1, directReport.Root.Summary.Undeclared, "positive control")
	require.Equal(t, 1, directReport.Root.Summary.Unused, "positive control")
	require.Equal(t, []string{"unused_var"}, directReport.Root.Unused, "positive control")
	require.Len(t, directReport.Root.Undeclared, 1, "positive control")
	require.Equal(t, "undefined_var", directReport.Root.Undeclared[0].Name, "positive control")

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, link))
	resolvedLink, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolvedLink, "fixture must actually be a symlink")

	linkedReport, err := Analyze(link)
	require.NoError(t, err)

	assert.Equal(t, directReport.Root.Summary, linkedReport.Root.Summary)
	assert.Equal(t, directReport.Root.Unused, linkedReport.Root.Unused)
	assert.Equal(t, directReport.Root.Undeclared, linkedReport.Root.Undeclared)
	assert.Equal(t, directReport.Root.Declared, linkedReport.Root.Declared)
}

// TestUT_Analyze_SymlinkedRoot_StillSkipsInteriorSymlink is a three-way
// discriminator: it fails on the pre-#419 tree (nothing is walked, so
// sibling_var never shows as referenced either), and it fails on an
// over-corrected walk that starts following interior symlinks (linked_var
// would then show as referenced instead of unused). It passes only when the
// root is resolved and interior symlinks keep today's skip behaviour.
func TestUT_Analyze_SymlinkedRoot_StillSkipsInteriorSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	config := `{"vars": {"sibling_var": "x", "linked_var": "y"}}`
	realDir := setupTemplate(t, config, map[string]string{
		"real.txt": "{{ vars.sibling_var }}",
	})

	externalFile := filepath.Join(t.TempDir(), "external.txt")
	require.NoError(t, os.WriteFile(externalFile, []byte("{{ vars.linked_var }}"), 0o644))
	linkedPath := filepath.Join(realDir, "linked.txt")
	require.NoError(t, os.Symlink(externalFile, linkedPath))
	resolvedLinked, evalErr := filepath.EvalSymlinks(linkedPath)
	require.NoError(t, evalErr)
	require.NotEqual(t, linkedPath, resolvedLinked, "fixture must actually be a symlink")

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, link))

	report, err := Analyze(link)
	require.NoError(t, err)

	assert.Equal(t, []string{"linked_var"}, report.Root.Unused,
		"sibling_var is referenced via the real file; linked_var's interior symlink must still be skipped")

	var siblingRefs int
	for _, dv := range report.Root.Declared {
		if dv.Name == "sibling_var" {
			siblingRefs = dv.ReferenceCount
		}
	}
	assert.Greater(t, siblingRefs, 0, "sibling_var must be counted as referenced")
}

// TestUT_Analyze_SymlinkedRoot_StillSkipsInteriorSymlinkToDirectory is the
// directory-target sibling of the interior-symlink-to-file test above: a
// symlink INSIDE the (now-resolved) template that points at a DIRECTORY
// outside it must not be descended into. leak.txt references an undeclared
// sentinel variable — if the directory were ever discovered, that variable
// would show up in Root.Undeclared. sibling_var must also show up as
// referenced (not merely "no undeclared vars"), or this would pass
// vacuously whether anything at all was walked.
func TestUT_Analyze_SymlinkedRoot_StillSkipsInteriorSymlinkToDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	config := `{"vars": {"sibling_var": "x"}}`
	realDir := setupTemplate(t, config, map[string]string{
		"real.txt": "{{ vars.sibling_var }}",
	})

	externalDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(externalDir, "leak.txt"), []byte("{{ vars.undiscovered_sentinel }}"), 0o644))
	linkedDirPath := filepath.Join(realDir, "linkeddir")
	require.NoError(t, os.Symlink(externalDir, linkedDirPath))
	resolvedLinkedDir, evalErr := filepath.EvalSymlinks(linkedDirPath)
	require.NoError(t, evalErr)
	require.NotEqual(t, linkedDirPath, resolvedLinkedDir, "fixture must actually be a symlink")

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, link))

	report, err := Analyze(link)
	require.NoError(t, err)

	assert.Empty(t, report.Root.Undeclared, "the symlinked directory's content must never be discovered")
	assert.Empty(t, report.Root.Unused, "sibling_var must show up as referenced, proving real.txt was actually walked")

	var siblingRefs int
	for _, dv := range report.Root.Declared {
		if dv.Name == "sibling_var" {
			siblingRefs = dv.ReferenceCount
		}
	}
	assert.Greater(t, siblingRefs, 0, "sibling_var must be counted as referenced")
}

// TestUT_Analyze_DanglingSymlinkRoot_StillReportsPreExistingMessage is a
// placement guard: ResolveSymlinkedRoot must run AFTER the existing
// os.Stat + IsDir validation, so a dangling root keeps producing today's
// "path does not exist" message.
func TestUT_Analyze_DanglingSymlinkRoot_StillReportsPreExistingMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	dangling := filepath.Join(t.TempDir(), "dangling")
	require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "does-not-exist"), dangling))

	_, err := Analyze(dangling)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path does not exist:")
}

// TestUT_Analyze_SymlinkToFile_StillReportsPreExistingMessage pins the shipped
// "path is not a directory" message for a symlink whose target is a FILE.
// Unlike its dangling-symlink sibling this is a NO-CHANGE GUARD, not a
// placement guard: os.Stat follows the link either way and the message
// prints the root as the user typed it, so moving ResolveSymlinkedRoot
// ahead of the Stat produces byte-identical output here. Verified by
// moving the call: this test passes on both placements, the dangling one
// fails. Do not "tighten" it to assert placement — it cannot.
func TestUT_Analyze_SymlinkToFile_StillReportsPreExistingMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	realFile := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(realFile, []byte("x"), 0o644))
	fileLink := filepath.Join(t.TempDir(), "filelink")
	require.NoError(t, os.Symlink(realFile, fileLink))

	_, err := Analyze(fileLink)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path is not a directory:")
}

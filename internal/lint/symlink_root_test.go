package lint

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUT_NewLinter_SymlinkedRoot_MatchesDirectRoot pins #419:
// filepath.WalkDir does not descend into a symlinked root — it yields the
// root itself as a symlink entry, which lintTemplateFiles' own walk then
// never sees past, so `tag template lint ./link` reported "Template is
// valid. No issues found." for a template with a real undefined-variable
// error. The direct-root assertions run FIRST as a positive control: two
// empty results would otherwise compare equal and pass on the broken tree.
func TestUT_NewLinter_SymlinkedRoot_MatchesDirectRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	realDir := t.TempDir()
	createTemplate(t, realDir, validConfig, map[string]string{
		"README.md": "# {{ vars.project_name }}\n{{ vars.undefined_var }}",
	})

	directLinter, err := NewLinter(realDir)
	require.NoError(t, err)
	directResult, err := directLinter.Run()
	require.NoError(t, err)

	require.True(t, directResult.HasErrors(), "positive control: the direct root must report the undefined-variable error")
	var directIssue *Issue
	for i := range directResult.Issues {
		if directResult.Issues[i].Rule == "undefined-variable" && directResult.Issues[i].File == "README.md" {
			directIssue = &directResult.Issues[i]
			break
		}
	}
	require.NotNil(t, directIssue, "expected undefined-variable issue for README.md on the direct root")
	assert.Equal(t, 2, directIssue.Line)
	assert.Contains(t, directIssue.Message, "undefined_var")

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, link))
	resolvedLink, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolvedLink, "fixture must actually be a symlink")

	linkedLinter, err := NewLinter(link)
	require.NoError(t, err)
	linkedResult, err := linkedLinter.Run()
	require.NoError(t, err)

	require.True(t, linkedResult.HasErrors(), "the symlinked root must report the same error as the direct root")
	require.Len(t, linkedResult.Issues, len(directResult.Issues))

	var linkedIssue *Issue
	for i := range linkedResult.Issues {
		if linkedResult.Issues[i].Rule == "undefined-variable" && linkedResult.Issues[i].File == "README.md" {
			linkedIssue = &linkedResult.Issues[i]
			break
		}
	}
	require.NotNil(t, linkedIssue, "expected undefined-variable issue for README.md through the symlinked root")
	assert.Equal(t, directIssue.Line, linkedIssue.Line)
	assert.Equal(t, directIssue.Message, linkedIssue.Message)
}

// TestUT_NewLinter_SymlinkedRoot_StillSkipsInteriorSymlink is a three-way
// discriminator over an interior symlink (a symlink INSIDE the resolved
// template, distinct from the root itself being a symlink): it fails on the
// pre-#419 tree because nothing is walked at all (the real sibling file is
// never found either), and it fails on an over-corrected walk that starts
// following interior symlinks (the excluded variable would then show up as
// referenced). It passes only when the root is resolved AND interior
// symlinks keep today's skip behaviour.
//
// Lint only reports issues for undefined VARIABLE REFERENCES (not for
// unused-but-declared ones), so both real.txt and linked.txt deliberately
// reference UNDECLARED sentinel names: real.txt's must be discovered (a
// real undefined-variable issue for it), linked.txt's must not (no issue
// mentions it) — an assertion that "no errors occur" would otherwise pass
// vacuously whether anything was walked at all.
func TestUT_NewLinter_SymlinkedRoot_StillSkipsInteriorSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	realDir := t.TempDir()
	createTemplate(t, realDir, validConfig, map[string]string{
		"real.txt": "{{ vars.real_sentinel }}",
	})

	externalFile := filepath.Join(t.TempDir(), "external.txt")
	require.NoError(t, os.WriteFile(externalFile, []byte("{{ vars.linked_sentinel }}"), 0o644))
	linkedPath := filepath.Join(realDir, "linked.txt")
	require.NoError(t, os.Symlink(externalFile, linkedPath))
	resolvedLinked, evalErr := filepath.EvalSymlinks(linkedPath)
	require.NoError(t, evalErr)
	require.NotEqual(t, linkedPath, resolvedLinked, "fixture must actually be a symlink")

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(realDir, link))

	linter, err := NewLinter(link)
	require.NoError(t, err)
	result, err := linter.Run()
	require.NoError(t, err)

	require.True(t, result.HasErrors(), "real.txt's undeclared reference must be discovered")
	var realIssue *Issue
	for i := range result.Issues {
		if result.Issues[i].File == "real.txt" {
			realIssue = &result.Issues[i]
		}
		assert.NotContains(t, result.Issues[i].Message, "linked_sentinel", "the interior symlink's content must never be discovered")
		assert.NotEqual(t, "linked.txt", result.Issues[i].File)
	}
	require.NotNil(t, realIssue, "expected an undefined-variable issue for real.txt")
	assert.Contains(t, realIssue.Message, "real_sentinel")
}

// TestUT_NewLinter_SymlinkedRoot_StillSkipsInteriorSymlinkToDirectory is the
// directory-target sibling of the interior-symlink-to-file test above: a
// symlink INSIDE the (now-resolved) template that points at a DIRECTORY
// outside it must not be descended into. Both real.txt and leak.txt
// deliberately reference UNDECLARED sentinel names: real.txt's must be
// discovered, leak.txt's must not — asserting only "no errors" would pass
// vacuously whether anything was walked at all.
func TestUT_NewLinter_SymlinkedRoot_StillSkipsInteriorSymlinkToDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	realDir := t.TempDir()
	createTemplate(t, realDir, validConfig, map[string]string{
		"real.txt": "{{ vars.real_sentinel }}",
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

	linter, err := NewLinter(link)
	require.NoError(t, err)
	result, err := linter.Run()
	require.NoError(t, err)

	require.True(t, result.HasErrors(), "real.txt's undeclared reference must be discovered")
	var realIssue *Issue
	for i := range result.Issues {
		if result.Issues[i].File == "real.txt" {
			realIssue = &result.Issues[i]
		}
		assert.NotContains(t, result.Issues[i].Message, "undiscovered_sentinel", "the symlinked directory's content must never be discovered")
	}
	require.NotNil(t, realIssue, "expected an undefined-variable issue for real.txt")
	assert.Contains(t, realIssue.Message, "real_sentinel")
}

// TestUT_NewLinter_DanglingSymlinkRoot_StillReportsPreExistingMessage is a
// placement guard: ResolveSymlinkedRoot must run AFTER the existing
// os.Stat + IsDir validation, so a dangling root keeps producing today's
// "path does not exist" message rather than a new "failed to resolve
// template root" one.
func TestUT_NewLinter_DanglingSymlinkRoot_StillReportsPreExistingMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	dangling := filepath.Join(t.TempDir(), "dangling")
	require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "does-not-exist"), dangling))

	_, err := NewLinter(dangling)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path does not exist:")
}

// TestUT_NewLinter_SymlinkToFile_StillReportsPreExistingMessage is the
// second placement-guard row: a symlink whose target is a FILE keeps the
// existing "path is not a directory" message.
func TestUT_NewLinter_SymlinkToFile_StillReportsPreExistingMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	realFile := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(realFile, []byte("x"), 0o644))
	fileLink := filepath.Join(t.TempDir(), "filelink")
	require.NoError(t, os.Symlink(realFile, fileLink))

	_, err := NewLinter(fileLink)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path is not a directory:")
}

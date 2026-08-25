package graph

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUT_Analyze_SymlinkedRoot_MatchesDirectRoot pins #419 for
// `tag template graph`: filepath.WalkDir does not descend into a symlinked
// root, so scanMarkers found nothing through a symlink — 1 marker direct vs.
// 0 markers through the symlink, measured against a `main` build. The
// fixture also carries a second, orphaned inject generator so a
// missing_target warning fires on BOTH roots (it depends only on
// report.Generators, populated via os.ReadDir which already follows a
// symlinked root); that warning is asserted as part of the literal expected
// payload on the direct run and then required to be byte-identical on the
// linked run, but report.Markers — the field the bug actually corrupts — is
// what the regression assertion hinges on, not the warning.
func TestUT_Analyze_SymlinkedRoot_MatchesDirectRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"route": {
				"route.go": "---\nto: internal/router.go\ninject: true\nafter: // TAG:routes\n---\nrouter.Handle()\n",
			},
			"orphan": {
				"orphan.go": "---\nto: internal/missing.go\ninject: true\nafter: // TAG:missing\n---\norphan.Handle()\n",
			},
		},
		map[string]string{
			"internal/router.go": "package internal\n\n// TAG:routes\n",
		},
	)

	directReport, err := Analyze(root)
	require.NoError(t, err)

	require.Len(t, directReport.Markers, 1, "positive control: direct root must find the marker")
	assert.Equal(t, "internal/router.go", directReport.Markers[0].File)
	assert.Equal(t, 3, directReport.Markers[0].Line)

	require.Contains(t, directReport.Warnings, Warning{
		Code:      "missing_target",
		Generator: "orphan",
		Message:   `generator "orphan" injects into "internal/missing.go", but no generator creates it (may exist in scaffold)`,
	}, "positive control: the orphaned inject target must produce a missing_target warning")

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(root, link))
	resolvedLink, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr)
	require.NotEqual(t, link, resolvedLink, "fixture must actually be a symlink")

	linkedReport, err := Analyze(link)
	require.NoError(t, err)

	assert.Equal(t, directReport.Markers, linkedReport.Markers)
	assert.Equal(t, directReport.Warnings, linkedReport.Warnings, "missing_target is independent of the walk bug and must be byte-identical")
	assert.Equal(t, len(directReport.Generators), len(linkedReport.Generators))
}

// TestUT_Analyze_SymlinkedRoot_InteriorSymlinkAsymmetry runs entirely
// through the symlinked ROOT and is a three-way discriminator over BOTH
// kinds of interior symlink at once:
//   - a symlink to a FILE: graph follows it today (scanFileForMarkers does
//     a plain os.ReadFile, which follows symlinks) — router_linked.go's
//     marker must still be found.
//   - a symlink to a DIRECTORY: filepath.WalkDir never descends into it
//     regardless of the #419 fix — deep.go's marker must NOT appear.
//
// It fails on the pre-#419 tree (0 markers — nothing is walked, so even
// router.go's own marker is missing), and it fails on an over-corrected walk
// that starts skipping interior file-symlinks too (only 1 marker). It
// passes only when the root is resolved and today's asymmetric interior
// handling is preserved exactly.
func TestUT_Analyze_SymlinkedRoot_InteriorSymlinkAsymmetry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests unreliable on Windows")
	}

	root := setupGraphTemplate(t,
		map[string]map[string]string{
			"route": {
				"route.go": "---\nto: internal/router.go\ninject: true\nafter: // TAG:routes\n---\nrouter.Handle()\n",
			},
		},
		map[string]string{
			"internal/router.go": "package internal\n\n// TAG:routes\n",
		},
	)

	externalFile := filepath.Join(t.TempDir(), "linked_router.go")
	require.NoError(t, os.WriteFile(externalFile, []byte("package internal\n\n// TAG:routes\n"), 0o644))
	linkedFilePath := filepath.Join(root, "internal", "router_linked.go")
	require.NoError(t, os.Symlink(externalFile, linkedFilePath))
	resolvedLinkedFile, evalErr := filepath.EvalSymlinks(linkedFilePath)
	require.NoError(t, evalErr)
	require.NotEqual(t, linkedFilePath, resolvedLinkedFile, "fixture must actually be a symlink")

	externalDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(externalDir, "deep.go"), []byte("package internal\n\n// TAG:routes\n"), 0o644))
	linkedDirPath := filepath.Join(root, "internal", "routerdir_linked")
	require.NoError(t, os.Symlink(externalDir, linkedDirPath))
	resolvedLinkedDir, evalErr := filepath.EvalSymlinks(linkedDirPath)
	require.NoError(t, evalErr)
	require.NotEqual(t, linkedDirPath, resolvedLinkedDir, "fixture must actually be a symlink")

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(root, link))

	report, err := Analyze(link)
	require.NoError(t, err)

	var files []string
	for _, m := range report.Markers {
		files = append(files, m.File)
	}
	sort.Strings(files)
	assert.Equal(t, []string{"internal/router.go", "internal/router_linked.go"}, files,
		"the symlinked file must still be followed; the symlinked directory must not be descended into")
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

// TestUT_Analyze_SymlinkToFile_StillReportsPreExistingMessage is the second
// placement-guard row: a symlink whose target is a FILE keeps the existing
// "path is not a directory" message.
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

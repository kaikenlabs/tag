package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
)

// newTestLibraryDataDir points newLocalLibrary at a fresh, empty library and
// isolates HOME so replay.Save (which resolves os.UserHomeDir() directly)
// never touches the developer's real ~/.tag/replay.
func newTestLibraryDataDir(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	dataDir := t.TempDir()
	orig := newLocalLibrary
	newLocalLibrary = func() (*library.Library, error) {
		return library.NewLocal(dataDir), nil
	}
	t.Cleanup(func() { newLocalLibrary = orig })
	return dataDir
}

// TestCT_Scaffold_NoLibrary_BeatsAddToLib pins that --no-library suppresses
// the library write even when --add-to-lib is also set, matching the
// documented precedent that --ignore-lock beats --update-lock.
func TestCT_Scaffold_NoLibrary_BeatsAddToLib(t *testing.T) {
	dataDir := newTestLibraryDataDir(t)

	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, types.GeneratorsDir), 0o755))

	name := filepath.Base(templateDir)

	outputDir := t.TempDir()
	ctx := newCoverageCLIContext(t, nil, map[string]string{
		"no-input":         "true",
		"output":           filepath.Join(outputDir, "with-lib"),
		flags.AddToLibFlag: "true",
	}, nil)
	require.NoError(t, scaffoldFromRef(ctx, []string{templateDir, "with-lib"}, false, testVersion))

	lib := library.NewLocal(dataDir)
	_, err := lib.Get(name)
	assert.NoError(t, err, "expected --add-to-lib alone to add a library entry")

	require.NoError(t, lib.Remove(name))

	outputDir2 := t.TempDir()
	ctx2 := newCoverageCLIContext(t, nil, map[string]string{
		"no-input":          "true",
		"output":            filepath.Join(outputDir2, "no-lib"),
		flags.AddToLibFlag:  "true",
		flags.NoLibraryFlag: "true",
	}, nil)
	require.NoError(t, scaffoldFromRef(ctx2, []string{templateDir, "no-lib"}, false, testVersion))

	_, err = lib.Get(name)
	assert.Error(t, err, "expected --no-library to suppress the library entry even with --add-to-lib")
}

// TestCT_Scaffold_NoLibrary_NeverConsultsThePrompter pins that --no-library
// short-circuits resolveAddToLib entirely, so the interactive "Add template
// to library?" prompt never fires even when the template has generators and
// the run is otherwise interactive.
func TestCT_Scaffold_NoLibrary_NeverConsultsThePrompter(t *testing.T) {
	newTestLibraryDataDir(t)
	withNoPrompting(t)
	withTTY(t, true)

	templateDir := templateDirWithGenerators(t)
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	outputDir := t.TempDir()
	ctx := newCoverageCLIContext(t, nil, map[string]string{
		"output":            filepath.Join(outputDir, "interactive-project"),
		flags.NoLibraryFlag: "true",
	}, nil)

	require.NoError(t, scaffoldFromRef(ctx, []string{templateDir, "interactive-project"}, false, testVersion))
}

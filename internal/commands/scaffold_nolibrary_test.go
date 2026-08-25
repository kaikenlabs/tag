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

// TestUT_Scaffold_NoLibrary_BeatsAddToLib pins that --no-library suppresses
// the library write even when --add-to-lib is also set, and that the project
// still receives its own generators in that case. The precedence (rather than
// a usage error) mirrors --ignore-lock beating --update-lock.
//
// Not parallel: isolateLibrary substitutes package-level state.
func TestUT_Scaffold_NoLibrary_BeatsAddToLib(t *testing.T) {
	dataDir := isolateLibrary(t)

	templateDir := t.TempDir()
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")
	generatorFile := filepath.Join(templateDir, types.GeneratorsDir, "gen", "gen.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(generatorFile), 0o755))
	require.NoError(t, os.WriteFile(generatorFile, []byte("package gen\n"), 0o644))

	name := filepath.Base(templateDir)
	lib := library.NewLocal(dataDir)

	addToLibOut := filepath.Join(t.TempDir(), "with-lib")
	ctx := newCoverageCLIContext(t, nil, map[string]string{
		"no-input":         "true",
		"output":           addToLibOut,
		flags.AddToLibFlag: "true",
	}, nil)
	require.NoError(t, scaffoldFromRef(ctx, []string{templateDir, "with-lib"}, false, testVersion))

	_, err := lib.Get(name)
	require.NoError(t, err, "expected --add-to-lib alone to add a library entry")
	require.NoError(t, lib.Remove(name))

	noLibOut := filepath.Join(t.TempDir(), "no-lib")
	ctx2 := newCoverageCLIContext(t, nil, map[string]string{
		"no-input":          "true",
		"output":            noLibOut,
		flags.AddToLibFlag:  "true",
		flags.NoLibraryFlag: "true",
	}, nil)
	require.NoError(t, scaffoldFromRef(ctx2, []string{templateDir, "no-lib"}, false, testVersion))

	// Assert the specific not-found error rather than any error: a corrupt
	// registry or a write under another name would satisfy a bare assert.Error
	// while global state had in fact changed.
	_, err = lib.Get(name)
	require.ErrorIs(t, err, library.ErrTemplateNotFound)

	entries, listErr := lib.List()
	require.NoError(t, listErr)
	assert.Empty(t, entries, "--no-library must not add an entry under any name")

	// Suppressing the library write is only half the contract; the project has
	// to end up with the generators it would otherwise resolve from the library.
	// createMinimalTemplate builds a {{ vars.project_name }} wrapper, so the
	// project root sits one level below --output.
	projectRoot := filepath.Join(noLibOut, "no-lib")
	copied, readErr := os.ReadFile(filepath.Join(projectRoot, types.TemplatesDir, "gen", "gen.go"))
	require.NoError(t, readErr)
	assert.Equal(t, "package gen\n", string(copied))
}

// TestUT_Scaffold_NoLibrary_NeverConsultsThePrompter pins that --no-library
// short-circuits resolveAddToLib entirely, so the interactive "Add template to
// library?" prompt never fires even though the template has generators and the
// run is otherwise interactive.
//
// Not parallel: substitutes newPrompter and isTTY package state.
func TestUT_Scaffold_NoLibrary_NeverConsultsThePrompter(t *testing.T) {
	isolateLibrary(t)
	withNoPrompting(t)
	withTTY(t, true)

	templateDir := templateDirWithGenerators(t)
	createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

	ctx := newCoverageCLIContext(t, nil, map[string]string{
		"output":            filepath.Join(t.TempDir(), "interactive-project"),
		flags.NoLibraryFlag: "true",
	}, nil)

	require.NoError(t, scaffoldFromRef(ctx, []string{templateDir, "interactive-project"}, false, testVersion))
}

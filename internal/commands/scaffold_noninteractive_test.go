package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/pkg/app"
)

// `--format json` implies non-interactive, and every test in this file forces
// isTTY TRUE before exercising a prompt site.
//
// That is the whole point. GetPrompter already falls back to NoopPrompter
// whenever !IsTTY(), and `go test` never has a TTY, so under the default probe
// every one of these call sites takes the non-interactive branch whether or not
// JSON mode is wired up — the assertions would pass on a tree with the jsonMode
// term deleted. Pinning isTTY true removes that alibi: the only remaining term
// that can produce non-interactive behaviour is jsonMode itself.
//
// These tests cover the call SITES. TestUT_ScaffoldNonInteractive_TruthTable
// covers the nonInteractive predicate in isolation; passing that one does not
// establish that anything actually consults it.

// failingPrompter fails the test on any call. It is the actual assertion in the
// call-site tests below: a return-value check cannot distinguish "skipped the
// prompt" from "prompted and the prompt errored", and both call sites collapse
// those two into the same answer.
type failingPrompter struct{ t *testing.T }

func (p failingPrompter) Input(label, _ string, _ bool) (string, error) {
	p.t.Fatalf("prompted for input (%q) in JSON mode", label)
	return "", nil
}

func (p failingPrompter) Select(label string, _ []string, _ int) (string, error) {
	p.t.Fatalf("prompted for selection (%q) in JSON mode", label)
	return "", nil
}

func (p failingPrompter) Confirm(label string, _ bool) (bool, error) {
	p.t.Fatalf("prompted for confirmation (%q) in JSON mode", label)
	return false, nil
}

func (p failingPrompter) Number(label string, _ float64) (float64, error) {
	p.t.Fatalf("prompted for a number (%q) in JSON mode", label)
	return 0, nil
}

// withNoPrompting installs a prompter that fails the test if anything consults
// it. Not parallel-safe: newPrompter is package state.
func withNoPrompting(t *testing.T) {
	t.Helper()
	orig := newPrompter
	newPrompter = func() scaffold.Prompter { return failingPrompter{t: t} }
	t.Cleanup(func() { newPrompter = orig })
}

// withTTY forces the interactivity probe for one test and restores it after.
// Not parallel-safe: isTTY is package state.
func withTTY(t *testing.T, v bool) {
	t.Helper()
	orig := isTTY
	isTTY = func() bool { return v }
	t.Cleanup(func() { isTTY = orig })
}

// templateDirWithGenerators builds a template directory carrying a generators
// subdir. Without one, resolveAddToLib returns false before it ever reaches the
// interactivity check, and a test asserting "did not prompt" would pass
// vacuously.
func templateDirWithGenerators(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, types.TemplatesDir, "handler"), 0o755))
	return dir
}

func TestUT_BuildScaffoldOpts_JSONForcesNoInput(t *testing.T) {
	// The AC is "--format json forces the noop prompter and never blocks on
	// stdin". Asserting that through prompter behaviour is untestable for the
	// reason described above, so assert it where it is decided: on the Options
	// struct handed to the scaffold core.
	for _, tc := range []struct {
		name     string
		noInput  bool
		jsonMode bool
		want     bool
	}{
		{name: "text mode without --no-input stays interactive", noInput: false, jsonMode: false, want: false},
		{name: "text mode with --no-input", noInput: true, jsonMode: false, want: true},
		{name: "json mode forces NoInput", noInput: false, jsonMode: true, want: true},
		{name: "json mode and --no-input agree", noInput: true, jsonMode: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]string{}
			if tc.noInput {
				values["no-input"] = "true"
			}
			ctx := newScaffoldCLIContextExtra(t, values)

			opts := buildScaffoldOpts(ctx, t.TempDir(), "proj", nil, tc.jsonMode)

			assert.Equal(t, tc.want, opts.NoInput)
		})
	}
}

func TestUT_ResolveAddToLib_JSONModeDoesNotPromptOnATTY(t *testing.T) {
	withTTY(t, true)
	withNoPrompting(t)
	templateDir := templateDirWithGenerators(t)
	ctx := newScaffoldCLIContextExtra(t, nil)

	assert.False(t, resolveAddToLib(ctx, templateDir, true),
		"JSON mode must skip the 'add to library?' prompt even with a TTY attached")
}

func TestUT_ResolveTemplateName_JSONModeIsUsageErrorOnATTY(t *testing.T) {
	withTTY(t, true)
	ctx := newScaffoldCLIContextExtra(t, nil)

	// lib is nil deliberately: reaching pickTemplate would dereference it, so
	// a regression here fails loudly rather than silently opening a picker.
	name, err := resolveTemplateName(ctx, nil, nil, true)

	require.Error(t, err)
	assert.Empty(t, name)

	var cmdErr *app.CommandError
	require.ErrorAs(t, err, &cmdErr)
	assert.Equal(t, app.ExitUsage, cmdErr.Code)
}

func TestUT_HandleCookiecutterDetection_JSONModeErrorsWithoutPromptingOnATTY(t *testing.T) {
	withTTY(t, true)
	withNoPrompting(t)
	ctx := newScaffoldCLIContextExtra(t, nil)

	err := handleCookiecutterDetection(ctx, nil, "gh:user/tmpl", t.TempDir(), scaffold.Options{}, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Cookiecutter")
	assert.Contains(t, err.Error(), "tag convert cookiecutter",
		"the error must point at the non-interactive escape hatch")
}

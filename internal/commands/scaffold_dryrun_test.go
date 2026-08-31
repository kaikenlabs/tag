package commands

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/lockfile"
	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/types/flags"
)

// cookiecutterFixtureForDryRun builds a minimal, convertible Cookiecutter
// template so handleCookiecutterDetection's real-conversion branch has
// something genuine to convert.
func cookiecutterFixtureForDryRun(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ccJSON := `{"project_name": "my-project", "version": "1.0.0"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, types.CookiecutterConfigFile), []byte(ccJSON), 0o644))
	wrapperDir := filepath.Join(dir, "{{cookiecutter.project_name}}")
	require.NoError(t, os.MkdirAll(wrapperDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(wrapperDir, "README.md"),
		[]byte("# {{cookiecutter.project_name}}"),
		0o644,
	))
	return dir
}

// TestUT_VerifyTemplateLock_DryRunWritesNothingInCwd proves the HELPER
// honours opts.DryRun. It does NOT prove scaffoldFromRef's call expression
// forwards opts.DryRun into that helper — that mapping is
// TestUT_ScaffoldFromRef_DryRun_DoesNotWriteLockfile's job, below.
func TestUT_VerifyTemplateLock_DryRunWritesNothingInCwd(t *testing.T) {
	dryRunDir := t.TempDir()
	t.Chdir(dryRunDir)

	err := verifyTemplateLock("gh:test/tmpl", t.TempDir(), lockfile.VerifyOptions{DryRun: true})
	require.NoError(t, err)
	assert.NoDirExists(t, filepath.Join(dryRunDir, ".tag"))

	controlDir := t.TempDir()
	t.Chdir(controlDir)

	err = verifyTemplateLock("gh:test/tmpl", t.TempDir(), lockfile.VerifyOptions{})
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(controlDir, ".tag", "lock.json"))
}

// TestUT_HandleCookiecutterDetection_DryRunRefusesWithoutPrompting drives the
// dry-run refusal through a TTY, non-JSON, non-no-input context so the ONLY
// thing that can be making it refuse is opts.DryRun. withNoPrompting installs
// a prompter that fails the test on any call — that is the actual assertion:
// a returned error alone cannot distinguish "refused before prompting" from
// "prompted and the prompt errored".
func TestUT_HandleCookiecutterDetection_DryRunRefusesWithoutPrompting(t *testing.T) {
	withTTY(t, true)
	withNoPrompting(t)

	cwd := t.TempDir()
	t.Chdir(cwd)
	before, err := os.ReadDir(cwd)
	require.NoError(t, err)

	ctx := newScaffoldCLIContextExtra(t, nil)
	templateDir := cookiecutterFixtureForDryRun(t)

	err = handleCookiecutterDetection(ctx, nil, "gh:user/tmpl", templateDir, scaffold.Options{DryRun: true}, false, testVersion)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag convert cookiecutter gh:user/tmpl")

	after, readErr := os.ReadDir(cwd)
	require.NoError(t, readErr)
	assert.Equal(t, before, after, "a dry-run refusal must not touch the current directory")
}

// TestUT_HandleCookiecutterDetection_RealRunConvertsAndWrites is the
// positive control one field apart from the dry-run refusal above: it drives
// the exact same fixture through the real (non-dry-run) path with a scripted
// prompter and asserts the conversion actually happened. It deliberately does
// NOT assert on the function's returned error — runScaffold's retry against
// the freshly converted, minimal fixture may legitimately fail for unrelated
// reasons, and coupling this control to require.NoError would make it brittle
// for a reason that has nothing to do with #442.
func TestUT_HandleCookiecutterDetection_RealRunConvertsAndWrites(t *testing.T) {
	withTTY(t, true)

	templateDir := cookiecutterFixtureForDryRun(t)
	destination := filepath.Join(t.TempDir(), "converted")

	ctx := newScaffoldCLIContextExtra(t, nil)
	prompter := &mockPrompterForScaffold{confirmResult: true, inputResult: destination}
	origPrompter := newPrompter
	newPrompter = func() scaffold.Prompter { return prompter }
	t.Cleanup(func() { newPrompter = origPrompter })

	_ = handleCookiecutterDetection(ctx, nil, "gh:user/tmpl", templateDir, scaffold.Options{DryRun: false}, false, testVersion)

	assert.FileExists(t, filepath.Join(destination, types.TemplateConfigFile),
		"a real (non-dry-run) run must actually convert and write the template")
}

// TestUT_HandleCookiecutterDetection_RefusalPrecedence pins that the
// --dry-run refusal message wins over the non-interactive one when both
// conditions hold, per the ordering mandated in handleCookiecutterDetection.
// The two messages both mention "tag convert cookiecutter", so the
// discriminator has to be a phrase unique to each branch.
func TestUT_HandleCookiecutterDetection_RefusalPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name           string
		dryRun         bool
		noInput        bool
		jsonMode       bool
		wantPhrase     string
		unwantedPhrase string
	}{
		{
			name:           "dry-run and non-interactive together: dry-run message wins",
			dryRun:         true,
			noInput:        true,
			jsonMode:       false,
			wantPhrase:     "Cannot convert during --dry-run",
			unwantedPhrase: "Cannot convert in non-interactive mode",
		},
		{
			name:           "dry-run and json mode together: dry-run message wins",
			dryRun:         true,
			noInput:        false,
			jsonMode:       true,
			wantPhrase:     "Cannot convert during --dry-run",
			unwantedPhrase: "Cannot convert in non-interactive mode",
		},
		{
			// NO-CHANGE GUARD: this row passes on both sides of the #442
			// fix. It is the control proving the new dry-run branch does
			// not shadow the pre-existing non-interactive one.
			name:           "non-interactive alone: non-interactive message",
			dryRun:         false,
			noInput:        true,
			jsonMode:       false,
			wantPhrase:     "Cannot convert in non-interactive mode",
			unwantedPhrase: "Cannot convert during --dry-run",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withTTY(t, true)
			withNoPrompting(t)

			values := map[string]string{}
			if tc.noInput {
				values["no-input"] = "true"
			}
			ctx := newScaffoldCLIContextExtra(t, values)
			templateDir := cookiecutterFixtureForDryRun(t)

			err := handleCookiecutterDetection(ctx, nil, "gh:user/tmpl", templateDir,
				scaffold.Options{DryRun: tc.dryRun}, tc.jsonMode, testVersion)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantPhrase)
			assert.NotContains(t, err.Error(), tc.unwantedPhrase)
		})
	}
}

// TestUT_ScaffoldFromRef_DryRun_DoesNotWriteLockfile proves opts.DryRun
// actually reaches verifyTemplateLock through scaffoldFromRef's call
// expression, not merely that verifyTemplateLock honours the option in
// isolation (the #429 lesson: a pure helper proves the verdict, nothing
// proves the mapping). Before this test the only guard on that call
// expression was the subprocess-driven
// TestIT_Scaffold_DryRun_LeavesWorkDirUntouched.
//
// The ref must be REMOTE — scaffoldFromRef only reaches the lockfile at all
// under `if isRemote` — so the cache is seeded with a pinned .invalid ref,
// the same pattern the integration suite uses: httptest cannot serve a
// remote template (internal/remote/zip.go rejects http:// outright), and
// only a pinned ref consults the cache instead of fetching. --no-library
// keeps the run to the lockfile; the library half is #432's.
func TestUT_ScaffoldFromRef_DryRun_DoesNotWriteLockfile(t *testing.T) {
	const ref = "https://example.invalid/dryrun-lock-fixture.zip@v1"

	for _, tc := range []struct {
		name      string
		dryRun    bool
		wantWrite bool
	}{
		{name: "dry run writes no lockfile", dryRun: true, wantWrite: false},
		{name: "real run writes the lockfile", dryRun: false, wantWrite: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// t.Chdir panics under t.Parallel, and verifyTemplateLock
			// anchors the lockfile on os.Getwd.
			templateDir := t.TempDir()
			createMinimalTemplate(t, templateDir, "{{ vars.project_name }}")

			cacheDir := t.TempDir()
			parsed, parseErr := remote.Parse(ref)
			require.NoError(t, parseErr)
			cache, cacheErr := remote.NewFSCache(cacheDir)
			require.NoError(t, cacheErr)
			_, setErr := cache.Set(parsed.CacheKey(), templateDir, &remote.CacheMeta{
				OriginalRef: ref,
				Version:     parsed.Version,
				FetchedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			})
			require.NoError(t, setErr)
			t.Setenv("TAG_CACHE_DIR", cacheDir)

			workDir := t.TempDir()
			t.Chdir(workDir)

			outputPath := filepath.Join(t.TempDir(), "lockfile-mapping-proj")
			flagValues := map[string]string{
				"no-input":          "true",
				"output":            outputPath,
				flags.NoLibraryFlag: "true",
			}
			if tc.dryRun {
				flagValues[flags.DryRunFlag] = "true"
			}
			ctx := newCoverageCLIContext(t, nil, flagValues, io.Discard)

			require.NoError(t, scaffoldFromRef(ctx, []string{ref, "lockfile-mapping-proj"}, false, testVersion))

			if tc.wantWrite {
				lf, loadErr := lockfile.Load(workDir)
				require.NoError(t, loadErr)
				entry, ok := lf.Templates[ref]
				require.True(t, ok, "a real remote scaffold must pin the ref")
				assert.NotEmpty(t, entry.SHA256)
				return
			}
			assert.NoDirExists(t, filepath.Join(workDir, ".tag"),
				"a dry run must not create .tag/ in the working directory")
		})
	}
}

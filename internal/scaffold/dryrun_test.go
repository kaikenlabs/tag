package scaffold

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/hooks"
)

// These tests pin the documented contract of --dry-run
// (docs/commands/scaffold.md:46,185): "preview which files a scaffold would
// create without writing anything to disk or creating the output directory".
//
// Before #383 the flag suppressed only the per-file template writes. Every
// other step of executeScaffold still ran, so a "preview" created the output
// directory, wrote .tagconfig.json and .tag/history.json, executed hooks,
// deleted pre-existing directories under --force, and crashed outright on a
// project-wrapper template.
//
// Every dry-run assertion below is paired with a DryRun:false positive
// control. Without one, a tree where scaffolding is broken outright — nothing
// written, no hooks reached — satisfies the whole file.

// recordingHookRunner records invocations instead of executing anything.
//
// Asserting on a real shell hook's sentinel file would be weaker:
// RunPostScaffoldHooks discards the runner's error, so a missing sentinel is
// equally consistent with "the hook ran and failed".
type recordingHookRunner struct {
	phases []hooks.HookPhase
}

func (r *recordingHookRunner) Run(phase hooks.HookPhase, commands []string, _ string, _ []string) ([]hooks.HookResult, error) {
	if len(commands) > 0 {
		r.phases = append(r.phases, phase)
	}
	return nil, nil
}

// dryRunTemplate builds a minimal flat template (no project wrapper).
func dryRunTemplate(t *testing.T, hookCfg map[string]any) string {
	t.Helper()
	dir := t.TempDir()

	config := map[string]any{
		"name":    "dry-run-template",
		"version": "1.0.0",
		"vars": map[string]any{
			"project_name": map[string]any{"type": "string", "default": "demo"},
		},
	}
	if hookCfg != nil {
		config["hooks"] = hookCfg
	}

	data, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), data, 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "main.go"),
		[]byte("package main // {{ vars.project_name }}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"),
		[]byte("# {{ vars.project_name }}\n"), 0o644))

	return dir
}

func dryRunOpts(templateDir, outputDir string) Options {
	return Options{
		TemplateDir: templateDir,
		OutputDir:   outputDir,
		NoInput:     true,
		NoSave:      true,
		DryRun:      true,
	}
}

// runScaffold runs a scaffold with user-facing output captured, and returns the
// captured text plus the recording hook runner so callers can assert on both.
func runScaffold(t *testing.T, opts Options) (string, *recordingHookRunner, error) {
	t.Helper()
	var buf strings.Builder
	s, err := NewScaffold(opts, WithOutput(&buf), WithIsTTY(false))
	require.NoError(t, err)

	rec := &recordingHookRunner{}
	s.hookRunner = rec

	_, runErr := s.Run(opts)
	return buf.String(), rec, runErr
}

// listTree returns every path under root, relative and sorted, so a test can
// compare a directory against itself before and after a run. Comparing whole
// listings (rather than naming known artefacts) is what stops a newly added
// side-effect file from slipping through.
func listTree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel != "." {
			out = append(out, rel)
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(out)
	return out
}

func TestUT_Scaffold_DryRun_WritesNothing(t *testing.T) {
	templateDir := dryRunTemplate(t, nil)
	parent := t.TempDir()
	outputDir := filepath.Join(parent, "output")

	_, _, err := runScaffold(t, dryRunOpts(templateDir, outputDir))
	require.NoError(t, err)

	// One assertion covers MkdirAll, .tagconfig.json, .tag/history.json,
	// copied generators and every template write at once: if the output dir
	// was never created, none of them happened.
	assert.NoDirExists(t, outputDir,
		"--dry-run must not create the output directory (docs/commands/scaffold.md:46)")
	assert.Empty(t, listTree(t, parent), "--dry-run must write nothing to disk")
}

func TestUT_Scaffold_RealRun_WritesFilesAndHistory(t *testing.T) {
	// Positive control for TestUT_Scaffold_DryRun_WritesNothing. Without it,
	// a tree where scaffolding writes nothing at all would pass that test.
	templateDir := dryRunTemplate(t, nil)
	outputDir := filepath.Join(t.TempDir(), "output")

	opts := dryRunOpts(templateDir, outputDir)
	opts.DryRun = false

	_, _, err := runScaffold(t, opts)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(outputDir, "README.md"))
	assert.FileExists(t, filepath.Join(outputDir, "src", "main.go"))
	assert.FileExists(t, filepath.Join(outputDir, ".tagconfig.json"))
}

func TestUT_Scaffold_DryRun_RunsNoHooks(t *testing.T) {
	hookCfg := map[string]any{
		"pre_scaffold":  []string{"true"},
		"post_scaffold": []string{"true"},
	}

	for _, tc := range []struct {
		name       string
		dryRun     bool
		wantPhases int
	}{
		// The dry-run case uses AcceptHooks: the strongest form of the
		// assertion is that approved hooks still do not run.
		{name: "dry-run runs no hooks", dryRun: true, wantPhases: 0},
		{name: "real run runs both phases", dryRun: false, wantPhases: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			templateDir := dryRunTemplate(t, hookCfg)
			opts := dryRunOpts(templateDir, filepath.Join(t.TempDir(), "output"))
			opts.DryRun = tc.dryRun
			opts.AcceptHooks = true

			_, rec, err := runScaffold(t, opts)
			require.NoError(t, err)

			assert.Len(t, rec.phases, tc.wantPhases)
		})
	}
}

func TestUT_Scaffold_DryRun_MalformedHookTemplateDoesNotAbort(t *testing.T) {
	// renderHooksConfig runs before the file walk. Unless it too is skipped in
	// dry-run, an unparseable hook template aborts a preview that would never
	// have executed the hook anyway.
	templateDir := dryRunTemplate(t, map[string]any{
		"pre_scaffold": []string{"echo {{ vars.x | no_such_filter }}"},
	})

	opts := dryRunOpts(templateDir, filepath.Join(t.TempDir(), "output"))
	opts.AcceptHooks = true

	_, _, err := runScaffold(t, opts)
	require.NoError(t, err, "a malformed hook template must not abort a dry-run preview")
}

func TestUT_Scaffold_DryRun_WrapperTemplateWithExplicitOutput(t *testing.T) {
	// D4: a project-wrapper template ({{ vars.project_name }}/) plus an
	// explicit output dir made GenerateTagConfig write into a projectRoot
	// subdirectory that dry-run never created, so the run exited 1.
	// createTestTemplate is exactly that shape.
	templateDir := createTestTemplate(t)
	outputDir := filepath.Join(t.TempDir(), "output")

	opts := dryRunOpts(templateDir, outputDir)
	opts.Meta = map[string]string{"project_name": "awesome_project", "author": "John Doe"}

	// The previewed file list is deliberately not asserted here: the writer
	// still prints to a hardcoded os.Stdout, so it never reaches s.output.
	// #356 threads the writer through and exposes the list as
	// ScaffoldResult.Files, which is where that assertion belongs.
	_, _, err := runScaffold(t, opts)
	require.NoError(t, err, "dry-run must not crash on a project-wrapper template with --output")
	assert.NoDirExists(t, outputDir)
}

// seedOutputDir creates a pre-existing output directory holding a canary file
// and a nested file, and returns its path plus its full listing.
//
// The nested file matters: os.RemoveAll takes the whole tree, so a check
// confined to the top level can be satisfied by a partial recreate.
func seedOutputDir(t *testing.T) (string, []string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "proj")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "canary.txt"), []byte("USER DATA"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nested", "deep.txt"), []byte("NESTED DATA"), 0o644))
	return dir, listTree(t, dir)
}

func assertOutputDirIntact(t *testing.T, dir string, before []string) {
	t.Helper()
	// Content, never DirExists: executeScaffold calls MkdirAll immediately
	// after prepareOutputDir's RemoveAll, so "the directory exists" passes on
	// the broken tree. The surviving bytes are the whole test.
	canary, err := os.ReadFile(filepath.Join(dir, "canary.txt"))
	require.NoError(t, err, "the pre-existing canary file must survive")
	assert.Equal(t, "USER DATA", string(canary))

	nested, err := os.ReadFile(filepath.Join(dir, "nested", "deep.txt"))
	require.NoError(t, err, "the pre-existing nested file must survive")
	assert.Equal(t, "NESTED DATA", string(nested))

	assert.Equal(t, before, listTree(t, dir),
		"a dry run must neither remove nor add anything in the output directory")
}

func TestUT_Scaffold_DryRunForce_DoesNotDeleteExistingOutput(t *testing.T) {
	// D2: prepareOutputDir calls os.RemoveAll unconditionally under --force.
	templateDir := dryRunTemplate(t, nil)
	outputDir, before := seedOutputDir(t)

	opts := dryRunOpts(templateDir, outputDir)
	opts.Force = true

	_, _, err := runScaffold(t, opts)
	require.NoError(t, err, "a dry run must not fail")

	assertOutputDirIntact(t, outputDir, before)
}

func TestUT_Scaffold_DryRunFailure_DoesNotRollbackExistingOutput(t *testing.T) {
	// D5: the rollback `defer if !success { os.RemoveAll(outputDir) }` deletes
	// a pre-existing directory TAG never created. Gating only prepareOutputDir
	// (D2) leaves this live, which is why it needs its own fixture.
	//
	// The failure is an invalid template BODY, not the D4 wrapper crash: once
	// D4 is fixed that run succeeds, the defer no-ops, and a test built on it
	// would silently stop exercising the rollback.
	templateDir := dryRunTemplate(t, nil)
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "broken.md"),
		[]byte("{% for %}unclosed\n"), 0o644))

	outputDir, before := seedOutputDir(t)

	opts := dryRunOpts(templateDir, outputDir)
	opts.Force = true

	_, _, err := runScaffold(t, opts)
	require.Error(t, err, "the malformed template must make this run fail")

	assertOutputDirIntact(t, outputDir, before)
}

func TestUT_Scaffold_RealRunForce_StillReplacesExistingOutput(t *testing.T) {
	// Positive control for the two tests above: gating everything on !DryRun
	// and breaking --force outright must not look the same.
	templateDir := dryRunTemplate(t, nil)
	outputDir, _ := seedOutputDir(t)

	opts := dryRunOpts(templateDir, outputDir)
	opts.DryRun = false
	opts.Force = true

	_, _, err := runScaffold(t, opts)
	require.NoError(t, err)

	assert.NoFileExists(t, filepath.Join(outputDir, "canary.txt"),
		"a real --force run must still replace the existing output directory")
	assert.FileExists(t, filepath.Join(outputDir, "README.md"))
}

func TestUT_Scaffold_DryRun_StillReportsExistingOutputWithoutForce(t *testing.T) {
	// The fix gates prepareOutputDir's RemoveAll but must keep its existence
	// check: a preview whose real run would refuse to start has to say so.
	// Without this, "gate prepareOutputDir wholesale" reads as an equally
	// valid implementation of #383.
	templateDir := dryRunTemplate(t, nil)
	outputDir, before := seedOutputDir(t)

	_, _, err := runScaffold(t, dryRunOpts(templateDir, outputDir))

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOutputExists)
	assertOutputDirIntact(t, outputDir, before)
}

func TestUT_Scaffold_RunOptsDriveTheWriterDryRunFlag(t *testing.T) {
	// Run takes its own Options, independent of the ones NewScaffold was built
	// with, so DryRun has two readers: the writer's flag (set at construction)
	// and executeScaffold's gating (read from Run). If they disagree the run
	// still reports success — building dry and running real yields a project
	// containing .tagconfig.json and no files at all.
	templateDir := dryRunTemplate(t, nil)
	outputDir := filepath.Join(t.TempDir(), "output")

	construct := dryRunOpts(templateDir, outputDir)
	construct.DryRun = true

	run := construct
	run.DryRun = false

	var buf strings.Builder
	s, err := NewScaffold(construct, WithOutput(&buf), WithIsTTY(false))
	require.NoError(t, err)

	_, err = s.Run(run)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(outputDir, "README.md"),
		"Run's DryRun must drive the writer, not the value NewScaffold was built with")
	assert.FileExists(t, filepath.Join(outputDir, "src", "main.go"))
}

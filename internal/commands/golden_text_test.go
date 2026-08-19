package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/remote"
)

// updateGoldenText rewrites the golden fixtures instead of asserting against
// them. Run with: go test ./internal/commands -run TestUT_TextGolden -update-golden
//
// Provenance matters here, because a golden regenerated from the code it is
// meant to police proves only that the code equals itself:
//
//   - The six #346 fixtures were captured in the commit that introduced this
//     file, before any --format work landed on the branch.
//   - The four #345 fixtures (lint-text, variables-text, graph-text, graph-dot)
//     were captured by building `main` in a git worktree and running its binary
//     against the same fixture template, then asserting the refactored code
//     reproduces those bytes.
//   - The ten #347/#348/#349 fixtures were captured in a baseline commit before
//     those stories touched the source, and then independently re-verified
//     against a `main` worktree build (binary for version/doctor, a probe test
//     calling generateList and versionCheckAction for the rest). Every one
//     matched byte-for-byte.
//
// A later commit that changes a fixture is visible as a testdata diff in
// review — that is the whole point of the "text output stays byte-identical"
// acceptance criterion, and regenerating a fixture to make a failing test pass
// defeats it. If output must legitimately change, recapture from the previous
// release rather than from the working tree, and say so in the commit message.
var updateGoldenText = flag.Bool("update-golden", false, "rewrite golden text fixtures")

// cliRun is the captured result of one command invocation.
type cliRun struct {
	// Writer is what the command wrote to c.App.Writer.
	Writer string
	// Stdout is what the command wrote to the real os.Stdout, bypassing the
	// injected writer. Commands that have been converted to c.App.Writer leave
	// this empty; that emptiness is the machine-readable form of "stdout is
	// clean", which JSON consumers depend on.
	Stdout string
	Err    error
}

// All returns everything the user would have seen on stdout, regardless of
// which sink the command used. Golden fixtures assert on this so that moving a
// command from os.Stdout to c.App.Writer is provably output-preserving.
func (r cliRun) All() string { return r.Writer + r.Stdout }

// runCLI executes argv through a real cli.App so that urfave/cli's own argument
// parser runs. Tests that build a flag.FlagSet by hand cannot see flag-vs-positional
// ordering at all, because they hand the action a context in which the flag is
// already set.
func runCLI(t *testing.T, cmd *cli.Command, argv ...string) cliRun {
	t.Helper()

	var buf bytes.Buffer
	err := newTestApp(cmd, &buf).Run(append([]string{"tag"}, argv...))
	return cliRun{Writer: buf.String(), Err: err}
}

// runCLICapturingStdout additionally redirects the real os.Stdout, so a command
// that bypasses c.App.Writer is still observed.
//
// It replaces a process-global, so a test using it must NOT call t.Parallel:
// a sibling parallel test writing to os.Stdout during the capture window hits a
// closed pipe. That race is invisible locally and shows up under
// `go test -coverprofile`, whose end-of-run report is itself such a write.
// Prefer plain runCLI unless the test genuinely needs to see os.Stdout.
func runCLICapturingStdout(t *testing.T, cmd *cli.Command, argv ...string) cliRun {
	t.Helper()

	var buf bytes.Buffer
	app := newTestApp(cmd, &buf)

	var err error
	stdout := captureStdout(t, func() {
		err = app.Run(append([]string{"tag"}, argv...))
	})

	return cliRun{Writer: buf.String(), Stdout: stdout, Err: err}
}

func newTestApp(cmd *cli.Command, out io.Writer) *cli.App {
	return &cli.App{
		Writer:    out,
		ErrWriter: io.Discard,
		Commands:  []*cli.Command{cmd},
		// Default handler calls os.Exit for any cli.ExitCoder (check returns one).
		ExitErrHandler: func(*cli.Context, error) {},
	}
}

// goldenDir is resolved absolutely: some golden tests t.Chdir into a temp
// directory, which would otherwise send a relative testdata path there.
func goldenDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve golden fixture directory")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "golden")
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join(goldenDir(), name+".txt")
	if *updateGoldenText {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
		return
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden fixture %s — regenerate with -update-golden", path)
	require.Equal(t, string(want), got, "text output drifted from the golden fixture")
}

// --- fixtures -------------------------------------------------------------

// goldenTime is a fixed timestamp so fixtures containing dates stay stable.
var goldenTime = time.Date(2024, 3, 15, 9, 30, 0, 0, time.UTC)

// seedHome points HOME and XDG_DATA_HOME at a temp dir so the cache and the
// user-global dialect tier are both empty and deterministic.
func seedHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	return home
}

// seedCacheEntry writes a cache entry with a fixed timestamp. Optional opts
// mutate the CacheMeta before it is written — e.g. to set a ResolvedURL/
// OriginalRef containing a query-string credential for the redaction tests,
// which the fixed defaults below cannot express.
func seedCacheEntry(t *testing.T, home, key, version string, expires *time.Time, opts ...func(*remote.CacheMeta)) {
	t.Helper()

	entryDir := filepath.Join(home, ".tag", "cache", key)
	require.NoError(t, os.MkdirAll(entryDir, 0o750))

	meta := remote.CacheMeta{
		OriginalRef: "gh:acme/" + key,
		ResolvedURL: "https://github.com/acme/" + key,
		Version:     version,
		FetchedAt:   goldenTime,
		ExpiresAt:   expires,
	}
	for _, opt := range opts {
		opt(&meta)
	}
	data, err := json.Marshal(meta)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(entryDir, "_meta.json"), data, 0o600))
}

// seedLibrary substitutes newLocalLibrary with a library holding the given entries.
// Mutates a package-level var: callers must not use t.Parallel.
func seedLibrary(t *testing.T, entries ...*library.Entry) {
	t.Helper()

	dataDir := t.TempDir()
	reg := library.Registry{Version: 1, Entries: map[string]*library.Entry{}}
	for _, e := range entries {
		reg.Entries[e.Name] = e
		require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "templates", e.Name), 0o750))
	}

	data, err := json.Marshal(reg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "library.json"), data, 0o600))

	original := newLocalLibrary
	newLocalLibrary = func() (*library.Library, error) { return library.NewLocal(dataDir), nil }
	t.Cleanup(func() { newLocalLibrary = original })
}

// seedSearchServer points lib search at an httptest server returning repos.
// Mutates a package-level var: callers must not use t.Parallel.
func seedSearchServer(t *testing.T, repos []map[string]any) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": len(repos),
			"items":       repos,
		})
	}))
	t.Cleanup(srv.Close)

	original := searchBaseURL
	searchBaseURL = srv.URL
	t.Cleanup(func() { searchBaseURL = original })
}

func goldenRepo(name, desc string, stars int) map[string]any {
	return map[string]any{
		"name":             name,
		"full_name":        "acme/" + name,
		"description":      desc,
		"html_url":         "https://github.com/acme/" + name,
		"stargazers_count": stars,
		"updated_at":       goldenTime.Format(time.RFC3339),
		"language":         "Go",
	}
}

// stubResolver returns a fixed commit SHA for check.
type stubResolver struct{ sha string }

func (s stubResolver) ResolveLatestCommit(context.Context, *remote.Reference) (string, error) {
	return s.sha, nil
}

// seedProject writes a .tagconfig.json and substitutes the commit resolver.
// Mutates a package-level var: callers must not use t.Parallel.
func seedProject(t *testing.T, currentSHA, latestSHA string) string {
	t.Helper()

	dir := t.TempDir()
	cfg := map[string]any{
		"schema_version": 1,
		"template": map[string]any{
			"type":   "remote",
			"source": "gh:acme/go-api",
			"ref":    "main",
			"commit": currentSHA,
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tagconfig.json"), data, 0o600))

	original := newGitResolver
	newGitResolver = func() remote.LatestCommitResolver { return stubResolver{sha: latestSHA} }
	t.Cleanup(func() { newGitResolver = original })

	return dir
}

// seedTemplate writes a minimal template directory. Used for the four commands
// #345 retrofits onto the shared --format helpers, whose text paths must also
// stay byte-identical.
func seedTemplate(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(validTemplateConfig), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go.tmpl"), []byte("package {{ vars.project_name }}\n"), 0o600))
	return dir
}

// seedInfoTemplate writes a template carrying every section `template info`
// prints: metadata, variables of each kind, and both hook phases. It
// deliberately ships no README/HOWTO — glamour's rendering depends on
// terminal detection, so a golden containing it would pin this machine rather
// than this code. The doc path is covered separately by
// TestUT_TemplateInfo_TextRendersDocsThroughGlamour.
func seedInfoTemplate(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	cfg := `{
  "name": "go-api",
  "version": "1.2.0",
  "description": "Go HTTP API scaffold",
  "vars": {
    "project_name": "my-app",
    "use_docker": {"type": "boolean", "default": true},
    "license": {"type": "choice", "options": ["MIT", "Apache-2.0", "GPL-3.0", "BSD-3-Clause"]},
    "maintainer": {"type": "string", "prompt": "Maintainer", "required": true}
  },
  "hooks": {
    "pre_scaffold": ["echo pre"],
    "post_scaffold": ["go mod tidy", "git init"]
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(cfg), 0o600))
	return dir
}

// --- the golden suite -----------------------------------------------------

// TestUT_TextGolden pins the exact bytes of every text output path this wave
// touches. Substring assertions cannot detect tabwriter column-width or
// trailing-whitespace drift, which is precisely what regresses when a printer
// is extracted or its writer is swapped.
func TestUT_TextGolden(t *testing.T) {
	// Uses t.Setenv and mutates package-level vars — do NOT use t.Parallel.

	t.Run("cache-ls-empty", func(t *testing.T) {
		seedHome(t)
		run := runCLICapturingStdout(t, CacheCommand(), "cache", "ls")
		require.NoError(t, run.Err)
		assertGolden(t, "cache-ls-empty", run.All())
	})

	t.Run("cache-ls-entries", func(t *testing.T) {
		home := seedHome(t)
		expired := goldenTime.Add(-time.Hour)
		future := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
		seedCacheEntry(t, home, "go-api", "v1.2.0", &future)
		seedCacheEntry(t, home, "py-svc", "", &expired)
		seedCacheEntry(t, home, "pinned", "v0.1.0", nil)
		run := runCLICapturingStdout(t, CacheCommand(), "cache", "ls")
		require.NoError(t, run.Err)
		assertGolden(t, "cache-ls-entries", run.All())
	})

	t.Run("lib-ls-empty", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t)
		run := runCLICapturingStdout(t, LibCommand(), "lib", "ls")
		require.NoError(t, run.Err)
		assertGolden(t, "lib-ls-empty", run.All())
	})

	t.Run("lib-ls-entries", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t,
			&library.Entry{
				Name: "go-api", Source: "gh:acme/go-api", Version: "v1.2.0",
				Description: "Production Go HTTP service with observability wired in",
				AddedAt:     goldenTime, UpdatedAt: goldenTime,
			},
			&library.Entry{
				Name: "py-svc", Source: "gh:acme/py-svc",
				AddedAt: goldenTime, UpdatedAt: goldenTime,
			},
		)
		run := runCLICapturingStdout(t, LibCommand(), "lib", "ls")
		require.NoError(t, run.Err)
		assertGolden(t, "lib-ls-entries", run.All())
	})

	t.Run("lib-search-empty", func(t *testing.T) {
		seedSearchServer(t, nil)
		run := runCLICapturingStdout(t, LibCommand(), "lib", "search", "nothing")
		require.NoError(t, run.Err)
		assertGolden(t, "lib-search-empty", run.All())
	})

	t.Run("lib-search-results", func(t *testing.T) {
		seedSearchServer(t, []map[string]any{
			goldenRepo("go-api", "Production Go HTTP service template", 142),
			goldenRepo("py-svc", "", 7),
		})
		run := runCLICapturingStdout(t, LibCommand(), "lib", "search", "go")
		require.NoError(t, run.Err)
		assertGolden(t, "lib-search-results", run.All())
	})

	t.Run("dialect-list", func(t *testing.T) {
		seedHome(t)
		run := runCLICapturingStdout(t, DialectCommand(), "dialect", "list")
		require.NoError(t, run.Err)
		assertGolden(t, "dialect-list", run.All())
	})

	t.Run("dialect-show", func(t *testing.T) {
		seedHome(t)
		run := runCLICapturingStdout(t, DialectCommand(), "dialect", "show", "go")
		require.NoError(t, run.Err)
		assertGolden(t, "dialect-show-go", run.All())
	})

	t.Run("check-up-to-date", func(t *testing.T) {
		dir := seedProject(t, "abc1234567890", "abc1234567890")
		run := runCLICapturingStdout(t, CheckCommand(), "check", "--dir", dir)
		require.NoError(t, run.Err)
		assertGolden(t, "check-up-to-date", run.All())
	})

	// The four commands #345 retrofits onto the shared helpers. Their print
	// paths were not meant to change; these fixtures are what proves it.
	t.Run("lint-text", func(t *testing.T) {
		run := runCLICapturingStdout(t, templateLintCommand(), "lint", seedTemplate(t))
		assertGolden(t, "lint-text", run.All())
	})

	t.Run("variables-text", func(t *testing.T) {
		run := runCLICapturingStdout(t, templateVariablesCommand(), "variables", seedTemplate(t))
		require.NoError(t, run.Err)
		assertGolden(t, "variables-text", run.All())
	})

	t.Run("graph-text", func(t *testing.T) {
		run := runCLICapturingStdout(t, templateGraphCommand(), "graph", seedTemplate(t))
		require.NoError(t, run.Err)
		assertGolden(t, "graph-text", run.All())
	})

	t.Run("graph-dot", func(t *testing.T) {
		run := runCLICapturingStdout(t, templateGraphCommand(), "graph", seedTemplate(t), "--format", formatDOT)
		require.NoError(t, run.Err)
		assertGolden(t, "graph-dot", run.All())
	})

	t.Run("check-updates-available", func(t *testing.T) {
		dir := seedProject(t, "abc1234567890", "def0987654321")
		run := runCLICapturingStdout(t, CheckCommand(), "check", "--dir", dir)
		require.Error(t, run.Err)
		assertGolden(t, "check-updates-available", run.All())
	})
	t.Run("template-info-text", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t)
		run := runCLICapturingStdout(t, templateInfoCommand(), "info", seedInfoTemplate(t))
		require.NoError(t, run.Err)
		assertGolden(t, "template-info-text", run.All())
	})
}

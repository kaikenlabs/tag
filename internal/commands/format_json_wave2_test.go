package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/pkg/app"
)

// TestUT_FormatJSONWave2 covers the `--format json` behaviour added by #348
// (doctor), #349 (version) and #347 (template list / generate list).
//
// Uses seedHome/seedLibrary/seedGeneratorsWithBundleMembers/t.Setenv/t.Chdir,
// which mutate package-level vars — no subtest here may call t.Parallel().

// seedGeneratorsWithBundleMembers builds a .tag tree like seedGenerators (in
// golden_wave2_test.go), but gives the bundle a real member generator so the
// "generators" (bundle -> members)
// JSON cross-reference fields have real, non-null values to assert on.
func seedGeneratorsWithBundleMembers(t *testing.T) *config.Config {
	t.Helper()

	root := t.TempDir()
	tagDir := filepath.Join(root, ".tag")

	genDir := filepath.Join(tagDir, "mygen")
	require.NoError(t, os.MkdirAll(genDir, 0o750))
	writeJSONFixture(t, filepath.Join(genDir, types.TemplateConfigFile), map[string]any{
		"description": "A test generator",
	})
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "gen.tmpl"), []byte("content"), 0o644))

	gatedDir := filepath.Join(tagDir, "gatedgen")
	require.NoError(t, os.MkdirAll(gatedDir, 0o750))
	writeJSONFixture(t, filepath.Join(gatedDir, types.TemplateConfigFile), map[string]any{
		"description": "Needs a flag that is not set",
		"requires":    []string{"use_db"},
	})
	require.NoError(t, os.WriteFile(filepath.Join(gatedDir, "gen.tmpl"), []byte("content"), 0o644))

	bundleDir := filepath.Join(tagDir, "_bundles", "mybundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0o750))
	writeJSONFixture(t, filepath.Join(bundleDir, "mybundle"+types.BundleExtension), engine.Bundle{
		Description: "A test bundle",
		Generators:  []engine.GeneratorRef{{Name: "mygen"}},
	})

	return &config.Config{
		Env: config.Env{Path: tagDir, SharedPath: "_shared", BundlePath: "_bundles"},
	}
}

// seedBrokenLibraryEntry registers a library entry whose template directory
// was never created on disk, so doctorCheckLibraries' TemplatePath lookup
// fails and the check reports a fail rather than a pass/warn — used to
// exercise doctor's "fail" branch deterministically (unlike relying on git
// being absent from the host, which is not portable).
//
// doctorCheckLibraries opens the library via xdg.DataHome() + library.New
// directly — unlike the rest of the package it does NOT go through the
// overridable newLocalLibrary var — so the registry must be written where
// xdg.DataHome() actually resolves: seedHome must be called first so
// XDG_DATA_HOME is set, and the registry goes to $XDG_DATA_HOME/tag/library.json.
func seedBrokenLibraryEntry(t *testing.T, home, name string) {
	t.Helper()

	dataDir := filepath.Join(home, "data", "tag")
	reg := library.Registry{Version: 1, Entries: map[string]*library.Entry{
		name: {Name: name, Source: "gh:acme/" + name, AddedAt: goldenTime, UpdatedAt: goldenTime},
	}}
	// Deliberately no templates/<name> directory — TemplatePath() must fail.

	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	data, err := json.Marshal(reg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "library.json"), data, 0o600))
}

// --- #348 doctor ------------------------------------------------------

func TestUT_FormatJSON_Doctor(t *testing.T) {
	t.Run("emits parseable JSON with real values, status never an int, worst status surfaced", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t)
		t.Setenv("GITHUB_TOKEN", "")
		t.Chdir(t.TempDir())

		var buf bytes.Buffer
		// "dev" short-circuits the update check, so no network call is made.
		err := doctorAction(context.Background(), &buf, "dev", formatJSON)
		require.Error(t, err, "an empty project warns, so doctor must exit non-zero")

		var cmdErr *app.CommandError
		require.ErrorAs(t, err, &cmdErr)
		assert.Equal(t, doctorExitWarnings, cmdErr.ExitCode())

		// The JSON must be complete despite the non-zero exit. If a future
		// change returned the exit error BEFORE writing, this Unmarshal would
		// fail on an empty/truncated buffer.
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed),
			"doctor must write complete JSON before returning its exit-code-carrying error")

		sections, ok := parsed["sections"].([]any)
		require.True(t, ok, "top-level 'sections' key must be present")

		worst := "pass"
		sawWarn := false
		foundGitCheck := false
		for _, s := range sections {
			sec, ok := s.(map[string]any)
			require.True(t, ok)
			checks, ok := sec["checks"].([]any)
			require.True(t, ok)
			for _, c := range checks {
				check, ok := c.(map[string]any)
				require.True(t, ok)

				// doctorStatus must decode as a JSON string. If
				// MarshalJSON were removed, the underlying int would
				// encode as a JSON number and this assertion would fail.
				statusStr, isString := check["status"].(string)
				require.True(t, isString,
					"doctorStatus must serialise as a string, never a number: got %#v (label %v)",
					check["status"], check["label"])

				switch statusStr {
				case doctorStatusWarn:
					sawWarn = true
					if worst != doctorStatusFail {
						worst = doctorStatusWarn
					}
				case doctorStatusFail:
					worst = doctorStatusFail
				}

				if sec["name"] == "ENVIRONMENT" && check["label"] == "Git installed" {
					foundGitCheck = true
				}
			}
		}

		assert.True(t, sawWarn, "an empty project with no GITHUB_TOKEN must produce at least one warn check")
		assert.True(t, foundGitCheck, "ENVIRONMENT section must contain the real 'Git installed' check")
		assert.Equal(t, worst, parsed["status"],
			"top-level status must equal the worst individual check status — a broken worst-status computation would report pass/warn here instead")
	})

	t.Run("a failing check reports status fail and exit code 2", func(t *testing.T) {
		home := seedHome(t)
		t.Setenv("GITHUB_TOKEN", "ok") // suppress the GITHUB_TOKEN warn so fail is unambiguously the worst
		t.Chdir(t.TempDir())
		seedBrokenLibraryEntry(t, home, "broken-template")

		var buf bytes.Buffer
		err := doctorAction(context.Background(), &buf, "dev", formatJSON)
		require.Error(t, err)

		var cmdErr *app.CommandError
		require.ErrorAs(t, err, &cmdErr)
		assert.Equal(t, doctorExitFailures, cmdErr.ExitCode())

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
		assert.Equal(t, doctorStatusFail, parsed["status"])

		// A regression that hardcoded a "fail" status elsewhere (or dropped
		// the LIBRARIES section) must not pass here: the failure must be
		// traceable to the seeded broken library entry specifically.
		sections, ok := parsed["sections"].([]any)
		require.True(t, ok, "top-level 'sections' key must be present")

		var foundBrokenLibraryFail bool
		for _, s := range sections {
			sec, ok := s.(map[string]any)
			require.True(t, ok)
			if sec["name"] != "LIBRARIES" {
				continue
			}
			checks, ok := sec["checks"].([]any)
			require.True(t, ok)
			for _, c := range checks {
				check, ok := c.(map[string]any)
				require.True(t, ok)
				label, _ := check["label"].(string)
				if check["status"] == doctorStatusFail && strings.Contains(label, "broken-template") {
					foundBrokenLibraryFail = true
				}
			}
		}
		assert.True(t, foundBrokenLibraryFail,
			"LIBRARIES section must report a fail check for the seeded broken-template entry")
	})

	t.Run("rejects unsupported format", func(t *testing.T) {
		run := runCLI(t, DoctorCommand("dev"), "doctor", "--format", "xml")
		require.Error(t, run.Err)

		var cmdErr *app.CommandError
		require.ErrorAs(t, run.Err, &cmdErr)
		assert.Equal(t, app.ExitUsage, cmdErr.Code)
	})
}

// --- #349 version -------------------------------------------------------

// versionTestApp builds a minimal cli.App wired the same way VersionCommand
// wires its flags, so versionAction (which needs an injectable repoURL) can
// be exercised through a real cli.Context.
func versionTestApp(action func(c *cli.Context) error) *cli.App {
	return &cli.App{
		Commands: []*cli.Command{{
			Name:   "version",
			Flags:  versionFlags(),
			Action: action,
		}},
		ExitErrHandler: func(*cli.Context, error) {},
	}
}

func TestUT_FormatJSON_Version(t *testing.T) {
	t.Run("without --check: no network call, latest/update_available omitted", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Error("a version call without --check must never perform a network call")
		}))
		defer srv.Close()

		var buf bytes.Buffer
		cliApp := versionTestApp(func(c *cli.Context) error {
			return versionAction(c, &buf, "1.4.0", srv.URL)
		})
		require.NoError(t, cliApp.Run([]string{"tag", "version", "--format", "json"}))

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
		assert.Equal(t, "1.4.0", parsed["version"])
		assert.Equal(t, false, parsed["dev_build"])
		assert.NotContains(t, parsed, "latest")
		assert.NotContains(t, parsed, "update_available")
	})

	t.Run("dev build: dev_build true, update_available false, no network call even under --check", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Error("a dev build must never perform a network call")
		}))
		defer srv.Close()

		var buf bytes.Buffer
		cliApp := versionTestApp(func(c *cli.Context) error {
			return versionAction(c, &buf, "dev", srv.URL)
		})
		require.NoError(t, cliApp.Run([]string{"tag", "version", "--check", "--format", "json"}))

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
		assert.Equal(t, "dev", parsed["version"])
		assert.Equal(t, true, parsed["dev_build"])
		assert.Equal(t, false, parsed["update_available"],
			"a dev build must report update_available:false, never a spurious true")
	})

	t.Run("--check success reports real latest and update_available values", func(t *testing.T) {
		srv := seedReleaseServer(t, "v1.5.0")

		var buf bytes.Buffer
		cliApp := versionTestApp(func(c *cli.Context) error {
			return versionAction(c, &buf, "1.4.0", srv)
		})
		require.NoError(t, cliApp.Run([]string{"tag", "version", "--check", "--format", "json"}))

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
		assert.Equal(t, "1.4.0", parsed["version"])
		assert.Equal(t, false, parsed["dev_build"])
		assert.Equal(t, "1.5.0", parsed["latest"])
		assert.Equal(t, true, parsed["update_available"])
	})

	t.Run("--check network failure aborts, no JSON written", func(t *testing.T) {
		// Closed before use: an httptest server URL that is guaranteed
		// connection-refused, rather than dialing a fixed loopback port,
		// which depends on nothing else happening to listen there.
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Error("a closed server must never receive a request")
		}))
		srv.Close()

		var buf bytes.Buffer
		cliApp := versionTestApp(func(c *cli.Context) error {
			return versionAction(c, &buf, "1.4.0", srv.URL)
		})
		err := cliApp.Run([]string{"tag", "version", "--check", "--format", "json"})
		require.Error(t, err)

		var cmdErr *app.CommandError
		require.ErrorAs(t, err, &cmdErr)
		assert.NotEqual(t, app.ExitOK, cmdErr.ExitCode())
		assert.Empty(t, buf.String(), "a --check network failure must not emit partial/any JSON")
	})

	t.Run("rejects unsupported format", func(t *testing.T) {
		run := runCLI(t, VersionCommand("1.4.0"), "version", "--format", "xml")
		require.Error(t, run.Err)

		var cmdErr *app.CommandError
		require.ErrorAs(t, run.Err, &cmdErr)
		assert.Equal(t, app.ExitUsage, cmdErr.Code)
	})
}

// --- #347 template list / generate list ----------------------------------

func TestUT_FormatJSON_GeneratorLists(t *testing.T) {
	t.Run("template list and generate list emit the identical shape", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t)
		cfg := seedGeneratorsWithBundleMembers(t)

		tmplRun := runCLI(t, templateListCommand(cfg), "list", "--format", "json")
		require.NoError(t, tmplRun.Err)

		genRun := runCLI(t, generateListCommand(cfg), "list", "--format", "json")
		require.NoError(t, genRun.Err)

		assert.Equal(t, tmplRun.Writer, genRun.Writer,
			"template list and generate list share one implementation and must emit byte-identical JSON")
	})

	t.Run("default listing hides the gated generator and lists bundle members", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t)
		cfg := seedGeneratorsWithBundleMembers(t)

		run := runCLI(t, generateListCommand(cfg), "list", "--format", "json")
		require.NoError(t, run.Err)

		var parsed struct {
			Generators []map[string]any `json:"generators"`
			Bundles    []map[string]any `json:"bundles"`
		}
		require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))

		require.Len(t, parsed.Generators, 1, "gatedgen has an unmet requirement and must be hidden without --all")
		gen := parsed.Generators[0]
		assert.Equal(t, "mygen", gen["name"])
		assert.Equal(t, "A test generator", gen["description"])
		assert.NotContains(t, gen, "bundle",
			"a per-generator owning-bundle field cannot be substantiated from bundle->members data; "+
				"membership is derivable from bundles[].generators")
		assert.Equal(t, true, gen["requirements_met"])

		require.Len(t, parsed.Bundles, 1)
		bundle := parsed.Bundles[0]
		assert.Equal(t, "mybundle", bundle["name"])
		assert.Equal(t, "A test bundle", bundle["description"])
		assert.Equal(t, []any{"mygen"}, bundle["generators"])
		assert.Equal(t, true, bundle["requirements_met"])
	})

	t.Run("--all surfaces the gated generator with requirements_met false", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t)
		cfg := seedGeneratorsWithBundleMembers(t)

		run := runCLI(t, generateListCommand(cfg), "list", "--all", "--format", "json")
		require.NoError(t, run.Err)

		var parsed struct {
			Generators []map[string]any `json:"generators"`
		}
		require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))
		require.Len(t, parsed.Generators, 2, "--all must surface the gated generator alongside mygen")

		met := make(map[string]bool, len(parsed.Generators))
		for _, g := range parsed.Generators {
			met[g["name"].(string)] = g["requirements_met"].(bool)
		}
		assert.Equal(t, true, met["mygen"])
		assert.Equal(t, false, met["gatedgen"],
			"gatedgen's unmet 'use_db' requirement must surface as requirements_met:false under --all — "+
				"this is how JSON mirrors the text output's '[requires: x]' suffix")
	})

	t.Run("empty listing serialises generators and bundles as [] not null", func(t *testing.T) {
		cfg := &config.Config{Env: config.Env{Path: t.TempDir()}}

		run := runCLI(t, generateListCommand(cfg), "list", "--format", "json")
		require.NoError(t, run.Err)

		// Asserted on raw bytes: [] and null both unmarshal to a nil Go
		// slice, so decoding first would hide a regression to `var x []T`.
		assert.Contains(t, run.Writer, `"generators": []`)
		assert.Contains(t, run.Writer, `"bundles": []`)
		assert.NotContains(t, run.Writer, "null")
	})

	t.Run("rejects unsupported format", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t)
		cfg := seedGeneratorsWithBundleMembers(t)

		for _, cmd := range []*cli.Command{templateListCommand(cfg), generateListCommand(cfg)} {
			run := runCLI(t, cmd, "list", "--format", "xml")
			require.Error(t, run.Err)

			var cmdErr *app.CommandError
			require.ErrorAs(t, run.Err, &cmdErr)
			assert.Equal(t, app.ExitUsage, cmdErr.Code)
		}
	})
}

// jsonFieldName returns the wire name a struct field serialises under (the
// part of its `json:"..."` tag before the first comma), so a comparison
// ignores options like ",omitempty".
func jsonFieldName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	return name
}

// TestUT_GeneratorMetadataFieldNames_MatchGenerateInfo covers #347's
// acceptance criterion that field names for generator metadata match `tag
// generate info` where the concepts overlap. GeneratorInfo (generate_list.go)
// backs `generate list`/`template list --format json`; generatorInfoJSON
// (generate_info.go) backs `generate info --format json`. Both describe a
// generator's name and description and must use the same JSON key for each,
// so a script parsing both outputs never has to special-case one of them for
// the same concept.
func TestUT_GeneratorMetadataFieldNames_MatchGenerateInfo(t *testing.T) {
	t.Parallel()

	listType := reflect.TypeFor[GeneratorInfo]()
	infoType := reflect.TypeFor[generatorInfoJSON]()

	for _, goFieldName := range []string{"Name", "Description"} {
		listField, ok := listType.FieldByName(goFieldName)
		require.True(t, ok, "GeneratorInfo must have a %s field", goFieldName)
		infoField, ok := infoType.FieldByName(goFieldName)
		require.True(t, ok, "generatorInfoJSON must have a %s field", goFieldName)

		assert.Equal(t, jsonFieldName(infoField), jsonFieldName(listField),
			"GeneratorInfo.%s and generatorInfoJSON.%s must serialise under the same JSON key", goFieldName, goFieldName)
	}
}

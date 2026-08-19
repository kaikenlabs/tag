package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/pkg/app"
)

// TestUT_SixCommands_FormatJSON_Shape asserts that every --format json capable
// command in this wave writes exactly one JSON document to c.App.Writer (never
// to the real os.Stdout) and that the expected envelope/bare-object keys are
// present.
//
// Uses seedHome/seedLibrary/seedSearchServer/seedProject which mutate
// package-level vars and call t.Setenv — do NOT use t.Parallel.
func TestUT_SixCommands_FormatJSON_Shape(t *testing.T) {
	t.Run("check", func(t *testing.T) {
		dir := seedProject(t, "abc1234567890", "abc1234567890")
		run := runCLI(t, CheckCommand(), "check", "--dir", dir, "--format", "json")
		require.NoError(t, run.Err)
		assert.Empty(t, run.Stdout)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))
		assert.Contains(t, parsed, "up_to_date")
		assert.Contains(t, parsed, "current_sha")
		assert.Contains(t, parsed, "latest_sha")
		assert.Contains(t, parsed, "source")
		assert.Equal(t, true, parsed["up_to_date"])
	})

	t.Run("cache ls", func(t *testing.T) {
		home := seedHome(t)
		seedCacheEntry(t, home, "go-api", "v1.2.0", nil)
		run := runCLI(t, CacheCommand(), "cache", "ls", "--format", "json")
		require.NoError(t, run.Err)
		assert.Empty(t, run.Stdout)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))
		require.Contains(t, parsed, "entries")
		entries, ok := parsed["entries"].([]any)
		require.True(t, ok)
		require.Len(t, entries, 1)

		entry, ok := entries[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "go-api", entry["key"])

		meta, ok := entry["meta"].(map[string]any)
		require.True(t, ok, "meta object must be present")
		assert.Equal(t, "v1.2.0", meta["version"])
		assert.Equal(t, goldenTime.Format(time.RFC3339), meta["fetched_at"])
	})

	t.Run("lib ls", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t, &library.Entry{
			Name: "go-api", Source: "gh:acme/go-api", Version: "v1.2.0",
			AddedAt: goldenTime, UpdatedAt: goldenTime,
		})
		run := runCLI(t, LibCommand(), "lib", "ls", "--format", "json")
		require.NoError(t, run.Err)
		assert.Empty(t, run.Stdout)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))
		require.Contains(t, parsed, "templates")
		templates, ok := parsed["templates"].([]any)
		require.True(t, ok)
		require.Len(t, templates, 1)

		tmpl, ok := templates[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "go-api", tmpl["name"])
		assert.Equal(t, "gh:acme/go-api", tmpl["source"])
		assert.Equal(t, "v1.2.0", tmpl["version"])
	})

	t.Run("lib search", func(t *testing.T) {
		seedSearchServer(t, []map[string]any{goldenRepo("go-api", "desc", 5)})
		run := runCLI(t, LibCommand(), "lib", "search", "go", "--format", "json")
		require.NoError(t, run.Err)
		assert.Empty(t, run.Stdout)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))
		require.Contains(t, parsed, "results")
		results, ok := parsed["results"].([]any)
		require.True(t, ok)
		require.Len(t, results, 1)

		result, ok := results[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "go-api", result["name"])
		assert.Equal(t, "acme/go-api", result["full_name"])
		assert.Equal(t, "desc", result["description"])
		assert.Equal(t, float64(5), result["stars"])
	})

	t.Run("dialect list", func(t *testing.T) {
		seedHome(t)
		run := runCLI(t, DialectCommand(), "dialect", "list", "--format", "json")
		require.NoError(t, run.Err)
		assert.Empty(t, run.Stdout)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))
		require.Contains(t, parsed, "dialects")
		dialects, ok := parsed["dialects"].([]any)
		require.True(t, ok)
		assert.NotEmpty(t, dialects)

		var goDialect map[string]any
		for _, d := range dialects {
			entry, ok := d.(map[string]any)
			require.True(t, ok)
			if entry["name"] == "go" {
				goDialect = entry
				break
			}
		}
		require.NotNil(t, goDialect, "expected the built-in \"go\" dialect in the list")
		assert.Equal(t, "Go language type mappings", goDialect["description"])
	})

	t.Run("dialect show", func(t *testing.T) {
		seedHome(t)
		run := runCLI(t, DialectCommand(), "dialect", "show", "go", "--format", "json")
		require.NoError(t, run.Err)
		assert.Empty(t, run.Stdout)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))
		assert.Equal(t, "go", parsed["name"])
		assert.Contains(t, parsed, "types")
	})
}

// TestUT_SixCommands_RejectUnknownFormat asserts that all six commands reject
// an unsupported --format value with a usage error naming it.
func TestUT_SixCommands_RejectUnknownFormat(t *testing.T) {
	dir := seedProject(t, "abc1234567890", "abc1234567890")
	seedLibrary(t)
	seedSearchServer(t, nil)

	tests := []struct {
		name string
		cmd  func() *cli.Command
		argv []string
	}{
		{"check", CheckCommand, []string{"check", "--dir", dir, "--format", "xml"}},
		{"cache ls", CacheCommand, []string{"cache", "ls", "--format", "xml"}},
		{"lib ls", LibCommand, []string{"lib", "ls", "--format", "xml"}},
		{"lib search", LibCommand, []string{"lib", "search", "foo", "--format", "xml"}},
		{"dialect list", DialectCommand, []string{"dialect", "list", "--format", "xml"}},
		{"dialect show", DialectCommand, []string{"dialect", "show", "go", "--format", "xml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := runCLI(t, tt.cmd(), tt.argv...)
			require.Error(t, run.Err)
			assert.Contains(t, run.Err.Error(), `unsupported format "xml"`)

			var cmdErr *app.CommandError
			require.ErrorAs(t, run.Err, &cmdErr)
			assert.Equal(t, app.ExitUsage, cmdErr.Code)
		})
	}
}

// TestUT_EmptyResults_SerializeAsEmptyArray asserts on the raw JSON bytes,
// because both `null` and `[]` unmarshal to a nil Go slice — a struct
// assertion here would be vacuous.
func TestUT_EmptyResults_SerializeAsEmptyArray(t *testing.T) {
	t.Run("cache ls", func(t *testing.T) {
		seedHome(t)
		run := runCLI(t, CacheCommand(), "cache", "ls", "--format", "json")
		require.NoError(t, run.Err)
		assert.Contains(t, run.Writer, "\"entries\": []")
	})

	t.Run("lib ls", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t)
		run := runCLI(t, LibCommand(), "lib", "ls", "--format", "json")
		require.NoError(t, run.Err)
		assert.Contains(t, run.Writer, "\"templates\": []")
	})

	t.Run("lib search", func(t *testing.T) {
		seedSearchServer(t, nil)
		run := runCLI(t, LibCommand(), "lib", "search", "nothing", "--format", "json")
		require.NoError(t, run.Err)
		assert.Contains(t, run.Writer, "\"results\": []")
	})
}

// TestUT_DialectList_JSON_OmitsSource pins the decision that the registry
// does not track dialect provenance, so JSON must not fabricate a "source"
// key the way the text path's hardcoded "built-in" column does.
func TestUT_DialectList_JSON_OmitsSource(t *testing.T) {
	seedHome(t)
	run := runCLI(t, DialectCommand(), "dialect", "list", "--format", "json")
	require.NoError(t, run.Err)
	assert.NotContains(t, run.Writer, "source")
}

// TestUT_DialectShow_JSON_UsesTypesKey pins the field name to "types" (matches
// the Go field and the YAML users author), not "mappings".
func TestUT_DialectShow_JSON_UsesTypesKey(t *testing.T) {
	seedHome(t)
	run := runCLI(t, DialectCommand(), "dialect", "show", "go", "--format", "json")
	require.NoError(t, run.Err)
	assert.Contains(t, run.Writer, "\"types\"")
	assert.NotContains(t, run.Writer, "\"mappings\"")
}

// TestUT_Check_JSON_ThenExitCode asserts the exit-code-after-write ordering
// invariant: JSON is written in both cases, and the exit code fires after.
func TestUT_Check_JSON_ThenExitCode(t *testing.T) {
	t.Run("updates available", func(t *testing.T) {
		dir := seedProject(t, "abc1234567890", "def0987654321")
		run := runCLI(t, CheckCommand(), "check", "--dir", dir, "--format", "json")
		require.Error(t, run.Err)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed), "JSON must still be written before the exit code fires")
		assert.Equal(t, false, parsed["up_to_date"])

		exitErr, ok := run.Err.(cli.ExitCoder)
		require.True(t, ok, "error must carry an exit code")
		assert.Equal(t, 1, exitErr.ExitCode())
	})

	t.Run("up to date", func(t *testing.T) {
		dir := seedProject(t, "abc1234567890", "abc1234567890")
		run := runCLI(t, CheckCommand(), "check", "--dir", dir, "--format", "json")
		require.NoError(t, run.Err)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(run.Writer), &parsed))
		assert.Equal(t, true, parsed["up_to_date"])
	})
}

// TestUT_Check_QuietSuppressesJSON asserts that --quiet wins over --format
// json: nothing is written, but the exit code is unchanged.
func TestUT_Check_QuietSuppressesJSON(t *testing.T) {
	t.Run("up to date", func(t *testing.T) {
		dir := seedProject(t, "abc1234567890", "abc1234567890")
		run := runCLI(t, CheckCommand(), "check", "--dir", dir, "--quiet", "--format", "json")
		require.NoError(t, run.Err)
		assert.Empty(t, run.Writer)
		assert.Empty(t, run.Stdout)
	})

	t.Run("updates available", func(t *testing.T) {
		dir := seedProject(t, "abc1234567890", "def0987654321")
		run := runCLI(t, CheckCommand(), "check", "--dir", dir, "--quiet", "--format", "json")
		require.Error(t, run.Err)
		assert.Empty(t, run.Writer)
		assert.Empty(t, run.Stdout)

		exitErr, ok := run.Err.(cli.ExitCoder)
		require.True(t, ok, "error must carry an exit code")
		assert.Equal(t, 1, exitErr.ExitCode())
	})
}

// TestUT_LibSearch_TrailingFlagNotSwallowed is the regression test for the
// variadic-positional query: a trailing flag must be reparsed out of the
// query rather than folded into it as free text.
func TestUT_LibSearch_TrailingFlagNotSwallowed(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"total_count": 0, "items": []any{}})
	}))
	t.Cleanup(srv.Close)

	original := searchBaseURL
	searchBaseURL = srv.URL
	t.Cleanup(func() { searchBaseURL = original })

	run := runCLI(t, LibCommand(), "lib", "search", "foo", "--limit", "5")
	require.NoError(t, run.Err)
	assert.Equal(t, "topic:tag-template foo", gotQuery.Get("q"), "trailing --limit must not be folded into the query")
	assert.Equal(t, "5", gotQuery.Get("per_page"))

	run = runCLI(t, LibCommand(), "lib", "search", "a", "b", "--sort", "updated")
	require.NoError(t, run.Err)
	assert.Equal(t, "topic:tag-template a b", gotQuery.Get("q"))
	assert.Equal(t, "updated", gotQuery.Get("sort"))
}

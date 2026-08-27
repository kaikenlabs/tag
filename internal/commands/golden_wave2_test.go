package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/types"
)

// seedGenerators builds a .tag tree with one generator and one bundle, and
// returns a config pointing at it.
func seedGenerators(t *testing.T) *config.Config {
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
	})

	return &config.Config{
		Env: config.Env{Path: tagDir, SharedPath: "_shared", BundlePath: "_bundles"},
	}
}

func writeJSONFixture(t *testing.T, path string, v any) {
	t.Helper()

	data, err := json.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

// seedReleaseServer serves a GitHub /releases/latest redirect for the given tag.
func seedReleaseServer(t *testing.T, tag string) string {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://example.test/releases/tag/"+tag)
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// TestUT_TextGoldenWave2 pins the text output of the commands #347, #348 and
// #349 convert. Captured before those stories touched the source; see the
// provenance note on updateGoldenText.
func TestUT_TextGoldenWave2(t *testing.T) {
	// Uses t.Setenv / t.Chdir and mutates package-level vars — no t.Parallel.

	t.Run("generate-list", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t)
		cfg := seedGenerators(t)

		var buf bytes.Buffer
		require.NoError(t, generateList(cfg, false, &buf, formatText))
		assertGolden(t, "generate-list", buf.String())
	})

	t.Run("generate-list-all", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t)
		cfg := seedGenerators(t)

		var buf bytes.Buffer
		require.NoError(t, generateList(cfg, true, &buf, formatText))
		assertGolden(t, "generate-list-all", buf.String())
	})

	t.Run("version", func(t *testing.T) {
		run := runCLICapturingStdout(t, VersionCommand("1.4.0"), "version")
		require.NoError(t, run.Err)
		assertGolden(t, "version", run.All())
	})

	t.Run("version-check-update-available", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, versionCheckAction(context.Background(), &buf, "1.4.0", seedReleaseServer(t, "v1.5.0")))
		assertGolden(t, "version-check-update", buf.String())
	})

	t.Run("version-check-up-to-date", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, versionCheckAction(context.Background(), &buf, "1.4.0", seedReleaseServer(t, "v1.4.0")))
		assertGolden(t, "version-check-current", buf.String())
	})

	t.Run("version-check-dev-build", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, versionCheckAction(context.Background(), &buf, "dev", "unused"))
		assertGolden(t, "version-check-dev", buf.String())
	})

	t.Run("doctor-empty-project", func(t *testing.T) {
		seedHome(t)
		seedLibrary(t)
		t.Setenv("GITHUB_TOKEN", "")
		t.Chdir(t.TempDir())

		// "dev" short-circuits the update check, so no network call is made.
		var buf bytes.Buffer
		err := doctorAction(context.Background(), &buf, "dev", formatText)
		require.Error(t, err, "an empty project warns, so doctor must exit non-zero")
		assertGolden(t, "doctor-empty-project", buf.String())
	})
}

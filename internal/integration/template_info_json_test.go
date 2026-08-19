package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/commands"
	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/library"
)

// runTemplateInfoJSON drives `tag template info <ref> --format json` through a
// real cli.App (not a hand-built action call), so the library/remote
// resolution path in resolveTemplateDir is genuinely exercised end to end.
func runTemplateInfoJSON(t *testing.T, ref string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	app := &cli.App{
		Writer:         &buf,
		ErrWriter:      io.Discard,
		Commands:       []*cli.Command{commands.TemplateCommand(&config.Config{})},
		ExitErrHandler: func(*cli.Context, error) {},
	}

	err := app.Run([]string{"tag", "template", "info", ref, "--format", "json"})
	return buf.String(), err
}

// TestIT_TemplateInfoJSON_ResolvesLibraryName covers ticket #350's DTO end to
// end through resolveTemplateDir's two resolution paths: a name registered in
// the local template library, and a plain local filesystem path.
func TestIT_TemplateInfoJSON_ResolvesLibraryName(t *testing.T) {
	t.Run("library name", func(t *testing.T) {
		// newLocalLibrary is unexported to the commands package, so this test
		// cannot substitute it directly. Instead it points XDG_DATA_HOME at a
		// real on-disk library laid out exactly as the library package writes
		// one, and lets the real resolution path find it.
		dataHome := t.TempDir()
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_DATA_HOME", dataHome)

		appDataDir := filepath.Join(dataHome, "tag")
		templateDir := filepath.Join(appDataDir, "templates", "go-api")
		require.NoError(t, os.MkdirAll(templateDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"), []byte(`{
  "name": "go-api",
  "version": "1.0.0",
  "vars": {
    "project_name": "my-app"
  }
}`), 0o600))

		reg := library.Registry{
			Version: 1,
			Entries: map[string]*library.Entry{
				"go-api": {
					Name:      "go-api",
					Source:    "gh:acme/go-api",
					AddedAt:   time.Now(),
					UpdatedAt: time.Now(),
				},
			},
		}
		regData, err := json.Marshal(reg)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(appDataDir, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(appDataDir, "library.json"), regData, 0o600))

		out, err := runTemplateInfoJSON(t, "go-api")
		require.NoError(t, err)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &parsed), "output: %s", out)
		assert.Equal(t, "go-api", parsed["name"])
		assert.Equal(t, "1.0.0", parsed["version"])
	})

	t.Run("local path", func(t *testing.T) {
		// Point XDG_DATA_HOME at an empty library so library resolution
		// fails over to the local-path branch of resolveTemplateDir.
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_DATA_HOME", t.TempDir())

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "tag.template.json"), []byte(`{
  "name": "local-template",
  "vars": {}
}`), 0o600))

		out, err := runTemplateInfoJSON(t, dir)
		require.NoError(t, err)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &parsed), "output: %s", out)
		assert.Equal(t, "local-template", parsed["name"])
	})
}

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIT_JSONContract_VersionKeysOnRealBinary is the one behaviour in #395 that
// genuinely needs a subprocess: main.Version is set in package main by ldflags,
// so no test inside internal/commands can observe the value a real build
// reports.
//
// It cross-checks rather than asserting a literal. A plain `go build` yields
// "dev", `make build` yields "dev-<sha>", and a release build yields a semver —
// so the invariant that holds across all three is that the two documents agree
// with `tag version`, not that any of them matches a shape.
func TestIT_JSONContract_VersionKeysOnRealBinary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	workDir := t.TempDir()

	templateDir := filepath.Join(workDir, "tmpl")
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, "template"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "tag.template.json"),
		[]byte(`{"name":"probe","keywords":["go"],"categories":["backend"],"vars":{"project_name":"my-app"}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "template", "README.md"),
		[]byte("# {{ vars.project_name }}\n"), 0o600))

	// The version the binary reports about itself, whatever the build stamped.
	stdout, _, err := runTagSubprocess(t, ctx, workDir, "version", "--format", "json")
	require.NoError(t, err)

	var versionDoc struct {
		Version string `json:"version"`
	}
	require.NoError(t, json.Unmarshal(stdout, &versionDoc))
	require.NotEmpty(t, versionDoc.Version, "tag version must report something")

	t.Run("template info", func(t *testing.T) {
		out, _, infoErr := runTagSubprocess(t, ctx, workDir,
			"template", "info", templateDir, "--format", "json")
		require.NoError(t, infoErr)

		root := map[string]json.RawMessage{}
		require.NoError(t, json.Unmarshal(out, &root))

		assert.Equal(t, "1", string(root["schema_version"]))
		assert.JSONEq(t, `"`+versionDoc.Version+`"`, string(root["tag_version"]))

		require.Contains(t, root, "resolved_commit")
		assert.JSONEq(t, `""`, string(root["resolved_commit"]), "a local template has no commit")
		assert.JSONEq(t, `["go"]`, string(root["keywords"]))
		assert.JSONEq(t, `["backend"]`, string(root["categories"]))
	})

	t.Run("scaffold", func(t *testing.T) {
		out, _, scaffoldErr := runTagSubprocess(t, ctx, workDir,
			"scaffold", templateDir, "proj", "--output", filepath.Join(workDir, "out"), "--format", "json")
		require.NoError(t, scaffoldErr)

		root := map[string]json.RawMessage{}
		require.NoError(t, json.Unmarshal(out, &root))

		assert.Equal(t, "1", string(root["schema_version"]))
		assert.JSONEq(t, `"`+versionDoc.Version+`"`, string(root["tag_version"]))
	})
}

package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
)

// rootKeys decodes a JSON document to its raw top-level members. Decoding into
// a struct or a map[string]any would make "emitted as empty" and "not emitted
// at all" the same observation, which is exactly the distinction these tests
// exist to pin.
func rootKeys(t *testing.T, doc string) map[string]json.RawMessage {
	t.Helper()

	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(doc), &root))
	return root
}

// TestUT_TemplateInfoJSON_ResolvedCommitAlwaysPresent covers a local template,
// where the value is empty. Asserting presence on a remote template would prove
// nothing: a non-empty value is present under `omitempty` too.
func TestUT_TemplateInfoJSON_ResolvedCommitAlwaysPresent(t *testing.T) {
	dir := t.TempDir()
	createTemplateConfig(t, dir, map[string]any{
		"name": "local-tmpl",
		"vars": map[string]any{},
	})

	run := runCLI(t, templateInfoCommand(testVersion), "info", dir, "--format", formatJSON)
	require.NoError(t, run.Err)

	root := rootKeys(t, run.Writer)
	require.Contains(t, root, "resolved_commit",
		"a local template must still carry the key, so a consumer can tell empty from absent")
	assert.JSONEq(t, `""`, string(root["resolved_commit"]))
}

// TestUT_TemplateInfo_ResolvedCommit_FromCacheMeta drives the branch the ticket
// calls out as fragile: a cache hit repopulates CommitSHA from the entry's
// metadata rather than from a fetch. A pinned ref is required — a floating ref
// always refetches and would need the network.
func TestUT_TemplateInfo_ResolvedCommit_FromCacheMeta(t *testing.T) {
	const sha = "abc123def456789012345678901234567890abcd"

	home := seedHome(t)
	ref, err := remote.Parse("gh:acme/go-api@v1.2.0")
	require.NoError(t, err)

	key := ref.CacheKey()
	seedCacheEntry(t, home, key, "v1.2.0", nil, func(m *remote.CacheMeta) {
		m.CommitSHA = sha
	})
	entryDir := filepath.Join(home, ".tag", "cache", key)
	require.NoError(t, os.WriteFile(filepath.Join(entryDir, "tag.template.json"),
		[]byte(`{"name":"go-api","vars":{}}`), 0o600))

	run := runCLI(t, templateInfoCommand(testVersion), "info", "gh:acme/go-api@v1.2.0", "--format", formatJSON)
	require.NoError(t, run.Err)

	root := rootKeys(t, run.Writer)
	assert.JSONEq(t, `"`+sha+`"`, string(root["resolved_commit"]))
}

// TestUT_BuildTemplateInfoJSON_KeywordsAndCategories pins both the populated
// and the absent case. The absent case is the one that regresses:
// slices.Clone(nil) is nil, which marshals to null rather than [].
func TestUT_BuildTemplateInfoJSON_KeywordsAndCategories(t *testing.T) {
	t.Parallel()

	t.Run("populated", func(t *testing.T) {
		t.Parallel()

		dto := buildTemplateInfoJSON(&scaffold.TemplateConfig{
			Keywords:   []string{"go", "http"},
			Categories: []string{"backend"},
		}, false, false, "", testVersion)

		assert.Equal(t, []string{"go", "http"}, dto.Keywords)
		assert.Equal(t, []string{"backend"}, dto.Categories)
	})

	t.Run("absent", func(t *testing.T) {
		t.Parallel()

		dto := buildTemplateInfoJSON(&scaffold.TemplateConfig{}, false, false, "", testVersion)

		require.NotNil(t, dto.Keywords, "must be an empty slice, not nil — nil marshals to null")
		require.NotNil(t, dto.Categories, "must be an empty slice, not nil — nil marshals to null")
		assert.Empty(t, dto.Keywords)
		assert.Empty(t, dto.Categories)
	})

	t.Run("cloned not aliased", func(t *testing.T) {
		t.Parallel()

		config := &scaffold.TemplateConfig{Keywords: []string{"go"}}
		dto := buildTemplateInfoJSON(config, false, false, "", testVersion)
		dto.Keywords[0] = "mutated"

		assert.Equal(t, []string{"go"}, config.Keywords,
			"the DTO is documented as a pure value; mutating it must not reach the parsed config")
	})
}

// TestUT_BuildTemplateInfoJSON_DependsOn pins the DTO wiring only. What counts
// as a reference is specified by TestUT_DeclaredDeps_Table in internal/vars;
// re-testing the walker here would create a second spec that can drift.
func TestUT_BuildTemplateInfoJSON_DependsOn(t *testing.T) {
	t.Parallel()

	dto := buildTemplateInfoJSON(&scaffold.TemplateConfig{
		Vars: map[string]scaffold.VariableDef{
			"module": {Type: scaffold.VarTypeString, Default: `{{ "example.com/" ~ vars.project_name }}`},
			"plain":  {Type: scaffold.VarTypeString, Default: "literal"},
			"project_name": {
				Type:    scaffold.VarTypeString,
				Default: "svc",
			},
		},
	}, false, false, "", testVersion)

	byName := map[string][]string{}
	for _, v := range dto.Variables {
		require.NotNil(t, v.DependsOn, "variable %q has a nil depends_on", v.Name)
		byName[v.Name] = v.DependsOn
	}

	assert.Equal(t, []string{"project_name"}, byName["module"])
	assert.Equal(t, []string{}, byName["plain"])
	assert.Equal(t, []string{}, byName["project_name"])
}

// TestUT_TemplateInfoJSON_DependsOnSerialisesAsArray closes the gap that the
// existing EmptyCollectionsAreArrays test cannot: its fixture declares no
// variables, so no per-variable slice exists there for a null to hide in.
func TestUT_TemplateInfoJSON_DependsOnSerialisesAsArray(t *testing.T) {
	dir := t.TempDir()
	createTemplateConfig(t, dir, map[string]any{
		"name": "has-vars",
		"vars": map[string]any{"project_name": "my-app"},
	})

	run := runCLI(t, templateInfoCommand(testVersion), "info", dir, "--format", formatJSON)
	require.NoError(t, run.Err)

	assert.Contains(t, run.Writer, `"depends_on": []`)
	assert.NotContains(t, run.Writer, "null")
}

// TestUT_TemplateInfoJSON_RootKeySet pins the exact top-level key set of the
// `template info` document. Nothing asserted this before, so a new or dropped
// root key was previously invisible; #395 adds two and this is what makes any
// future third one a deliberate, reviewable decision.
func TestUT_TemplateInfoJSON_RootKeySet(t *testing.T) {
	dir := t.TempDir()
	createTemplateConfig(t, dir, map[string]any{
		"name": "keyset",
		"vars": map[string]any{},
	})

	run := runCLI(t, templateInfoCommand(testVersion), "info", dir, "--format", formatJSON)
	require.NoError(t, run.Err)

	root := rootKeys(t, run.Writer)

	got := make([]string, 0, len(root))
	for k := range root {
		got = append(got, k)
	}
	assert.ElementsMatch(t, []string{
		"schema_version", "tag_version", "name", "description", "version",
		"keywords", "categories", "variables", "hooks", "has_readme",
		"has_howto", "resolved_commit",
	}, got)

	// Compared as raw bytes: decoding into map[string]any yields float64 for
	// both 1 and 1.0 and cannot show that this is a bare integer literal.
	assert.Equal(t, "1", string(root["schema_version"]))
	assert.JSONEq(t, `"`+testVersion+`"`, string(root["tag_version"]),
		"tag_version must be the string threaded into the command, never a derived shape")
}

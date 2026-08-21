package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Manifest_LoadNonExistent_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	m, err := Load(dir)
	require.NoError(t, err)
	assert.Empty(t, m.Generations)
}

func TestUT_Manifest_SaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	hb := "sha256:aabbcc"
	original := Manifest{
		Generations: []Generation{
			{
				ID:        "gen_1_abc",
				Timestamp: time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC),
				Template:  "model",
				Command:   "generate",
				Files: []FileEntry{
					{Path: "handler.go", Action: ActionCreate, HashBefore: nil, HashAfter: "sha256:deadbeef"},
					{Path: "router.go", Action: ActionInject, HashBefore: &hb, HashAfter: "sha256:cafebabe"},
				},
			},
		},
	}

	require.NoError(t, Save(dir, original))
	loaded, err := Load(dir)
	require.NoError(t, err)

	require.Len(t, loaded.Generations, 1)
	g := loaded.Generations[0]
	assert.Equal(t, "gen_1_abc", g.ID)
	assert.Equal(t, "model", g.Template)
	assert.Len(t, g.Files, 2)
	assert.Nil(t, g.Files[0].HashBefore)
	assert.NotNil(t, g.Files[1].HashBefore)
	assert.Equal(t, "sha256:aabbcc", *g.Files[1].HashBefore)
}

func TestUT_Manifest_AtomicWrite_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()

	first := Manifest{Generations: []Generation{{ID: "gen_1_aaa", Command: "generate"}}}
	require.NoError(t, Save(dir, first))

	second := Manifest{Generations: []Generation{{ID: "gen_2_bbb", Command: "generate"}}}
	require.NoError(t, Save(dir, second))

	loaded, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, loaded.Generations, 1)
	assert.Equal(t, "gen_2_bbb", loaded.Generations[0].ID)
}

func TestUT_SaveManifest_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()

	const numWriters = 4
	const rounds = 5

	for round := range rounds {
		var wg sync.WaitGroup
		errs := make([]error, numWriters)
		candidates := make([]Manifest, numWriters)
		for w := range numWriters {
			m := Manifest{Generations: []Generation{{ID: fmt.Sprintf("gen_%d_%d", round, w), Command: "generate"}}}
			candidates[w] = m
			wg.Go(func() {
				errs[w] = Save(dir, m)
			})
		}
		wg.Wait()

		for _, err := range errs {
			require.NoError(t, err)
		}

		data, err := os.ReadFile(manifestPath(dir))
		require.NoError(t, err)

		var parsed Manifest
		require.NoError(t, json.Unmarshal(data, &parsed), "round %d: final manifest file must be valid JSON", round)

		matched := false
		for _, c := range candidates {
			if reflect.DeepEqual(parsed, c) {
				matched = true
				break
			}
		}
		assert.True(t, matched, "round %d: surviving manifest content must equal exactly one writer's payload, not a mix", round)
	}

	info, err := os.Stat(manifestPath(dir))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "history manifest must remain world-readable, owner-writable")
}

func TestUT_Manifest_CorruptJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	require.NoError(t, os.WriteFile(path, []byte("{not valid json"), 0o644))

	_, err := Load(dir)
	assert.Error(t, err)
}

func TestUT_Manifest_Append_AddsGeneration(t *testing.T) {
	dir := t.TempDir()
	g1 := Generation{ID: "gen_1_aaa", Command: "generate"}
	g2 := Generation{ID: "gen_2_bbb", Command: "generate"}

	require.NoError(t, Append(dir, g1))
	require.NoError(t, Append(dir, g2))

	m, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, m.Generations, 2)
	assert.Equal(t, "gen_1_aaa", m.Generations[0].ID)
	assert.Equal(t, "gen_2_bbb", m.Generations[1].ID)
}

func TestUT_Manifest_Remove_RemovesGeneration(t *testing.T) {
	dir := t.TempDir()
	g1 := Generation{ID: "gen_1_aaa", Command: "generate"}
	g2 := Generation{ID: "gen_2_bbb", Command: "generate"}
	require.NoError(t, Append(dir, g1))
	require.NoError(t, Append(dir, g2))

	require.NoError(t, Remove(dir, "gen_1_aaa"))

	m, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, m.Generations, 1)
	assert.Equal(t, "gen_2_bbb", m.Generations[0].ID)
}

func TestUT_Manifest_Remove_UnknownID_ReturnsErrNotFound(t *testing.T) {
	dir := t.TempDir()
	err := Remove(dir, "gen_unknown")
	assert.ErrorIs(t, err, ErrNotFound)
}

// legacyManifestJSON was produced by running main's history.Save before any
// of #352's source changes landed on this branch — i.e. it is what a real
// TAG binary wrote to disk before FileEntry.Action became a typed alias. It
// also carries a sixth entry with an action value ("future-op") that no
// version of TAG has ever written, standing in for a manifest produced by a
// future TAG release.
const legacyManifestJSON = `{
  "generations": [
    {
      "id": "gen_1700000000_abc123",
      "timestamp": "2025-11-14T22:13:20Z",
      "template": "service",
      "command": "generate",
      "files": [
        {
          "path": "internal/handler.go",
          "action": "create",
          "hash_before": null,
          "hash_after": "sha256:aaa"
        },
        {
          "path": "internal/router.go",
          "action": "inject",
          "hash_before": "sha256:before",
          "hash_after": "sha256:bbb"
        },
        {
          "path": "internal/routes.go",
          "action": "append",
          "hash_before": "sha256:before",
          "hash_after": "sha256:ccc"
        },
        {
          "path": "internal/config.go",
          "action": "overwrite",
          "hash_before": "sha256:before",
          "hash_after": "sha256:ddd"
        },
        {
          "path": "api/openapi.yaml",
          "action": "openapi-merge",
          "hash_before": "sha256:before",
          "hash_after": "sha256:eee"
        },
        {
          "path": "internal/future.go",
          "action": "future-op",
          "hash_before": "sha256:before",
          "hash_after": "sha256:fff"
        }
      ]
    }
  ]
}`

func TestUT_Manifest_Load_LegacyFixture_PreservesActions(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "history.json"), []byte(legacyManifestJSON), 0o644))

	m, err := Load(dir)
	require.NoError(t, err)
	require.Len(t, m.Generations, 1)

	files := m.Generations[0].Files
	require.Len(t, files, 6)
	byPath := make(map[string]FileEntry, len(files))
	for _, f := range files {
		byPath[f.Path] = f
	}

	create := byPath["internal/handler.go"]
	assert.Equal(t, ActionCreate, create.Action)
	assert.Nil(t, create.HashBefore)

	inject := byPath["internal/router.go"]
	assert.Equal(t, ActionInject, inject.Action)
	require.NotNil(t, inject.HashBefore)
	assert.Equal(t, "sha256:before", *inject.HashBefore)

	appendEntry := byPath["internal/routes.go"]
	assert.Equal(t, ActionAppend, appendEntry.Action)
	require.NotNil(t, appendEntry.HashBefore)

	overwrite := byPath["internal/config.go"]
	assert.Equal(t, ActionOverwrite, overwrite.Action)
	require.NotNil(t, overwrite.HashBefore)

	merge := byPath["api/openapi.yaml"]
	assert.Equal(t, ActionOpenAPIMerge, merge.Action)
	require.NotNil(t, merge.HashBefore)

	// The unknown action must load without error and be preserved verbatim —
	// this guards against someone later adding a validating UnmarshalJSON to
	// the named type, which would reject manifests from newer TAG versions.
	future := byPath["internal/future.go"]
	assert.Equal(t, Action("future-op"), future.Action)
	require.NotNil(t, future.HashBefore)
}

// TestUT_Manifest_Save_ActionWireValues asserts on the raw JSON bytes Save
// produces, not on a round-tripped struct — a round trip cannot catch a
// MarshalJSON that silently renames the wire values, since Unmarshal would
// invert the same renaming.
func TestUT_Manifest_Save_ActionWireValues(t *testing.T) {
	dir := t.TempDir()
	hb := "sha256:before"
	m := Manifest{
		Generations: []Generation{
			{
				ID:      "gen_1_abc",
				Command: "generate",
				Files: []FileEntry{
					{Path: "a.go", Action: ActionCreate, HashAfter: "sha256:a"},
					{Path: "b.go", Action: ActionInject, HashBefore: &hb, HashAfter: "sha256:b"},
					{Path: "c.go", Action: ActionAppend, HashBefore: &hb, HashAfter: "sha256:c"},
					{Path: "d.go", Action: ActionOverwrite, HashBefore: &hb, HashAfter: "sha256:d"},
					{Path: "e.yaml", Action: ActionOpenAPIMerge, HashBefore: &hb, HashAfter: "sha256:e"},
				},
			},
		},
	}

	require.NoError(t, Save(dir, m))
	data, err := os.ReadFile(filepath.Join(dir, "history.json"))
	require.NoError(t, err)

	raw := string(data)
	assert.Contains(t, raw, `"action": "create"`)
	assert.Contains(t, raw, `"action": "inject"`)
	assert.Contains(t, raw, `"action": "append"`)
	assert.Contains(t, raw, `"action": "overwrite"`)
	assert.Contains(t, raw, `"action": "openapi-merge"`)
}

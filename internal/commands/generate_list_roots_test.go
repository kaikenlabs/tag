package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

func countByName(infos []GeneratorInfo, name string) int {
	n := 0
	for _, info := range infos {
		if info.Name == name {
			n++
		}
	}
	return n
}

func descriptionsByName(infos []GeneratorInfo, name string) []string {
	var out []string
	for _, info := range infos {
		if info.Name == name {
			out = append(out, info.Description)
		}
	}
	return out
}

func TestUT_GenerateList_ScansBothLibraryRoots(t *testing.T) {
	setupLibEntryRoots(t, "roots-t6", map[string]string{
		filepath.Join(types.TemplatesDir, "dupgen", types.TemplateConfigFile):                       `{"description":"tag-desc"}`,
		filepath.Join(types.GeneratorsDir, "dupgen", types.TemplateConfigFile):                      `{"description":"generators-desc"}`,
		filepath.Join(types.TemplatesDir, "onlyTag", "gen.go"):                                      "package main\n",
		filepath.Join(types.GeneratorsDir, "onlyGenerators", "gen.go"):                              "package main\n",
		filepath.Join(types.TemplatesDir, types.BundlesDir, "dupbundle", "dupbundle.json"):          `{"description":"tag-bundle-desc","generators":[{"name":"g1"}]}`,
		filepath.Join(types.GeneratorsDir, types.BundlesDir, "dupbundle", "dupbundle.json"):         `{"description":"generators-bundle-desc","generators":[{"name":"g1"}]}`,
		filepath.Join(types.TemplatesDir, types.BundlesDir, "onlyTagBundle", "onlyTagBundle.json"):  `{"description":"only tag bundle","generators":[{"name":"g1"}]}`,
		filepath.Join(types.GeneratorsDir, types.BundlesDir, "onlyGenBundle", "onlyGenBundle.json"): `{"description":"only gen bundle","generators":[{"name":"g1"}]}`,
	})

	localDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(localDir, "dupgen"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(localDir, "dupgen", "gen.go"), []byte("package main\n"), 0o644))

	cfg := createTestConfigWithLib(t, localDir, "roots-t6")

	var buf bytes.Buffer
	err := generateList(cfg, true, &buf, formatJSON)
	require.NoError(t, err)

	var doc generatorListJSON
	dec := json.NewDecoder(&buf)
	require.NoError(t, dec.Decode(&doc))

	assert.Equal(t, 2, countByName(doc.Generators, "dupgen"), "one template-scoped, one local-scoped, no cross-source dedup")
	assert.Contains(t, descriptionsByName(doc.Generators, "dupgen"), "tag-desc", ".tag/ must win the within-library collision")
	assert.NotContains(t, descriptionsByName(doc.Generators, "dupgen"), "generators-desc", "_generators/ entry must be dropped on collision")
	assert.Equal(t, 1, countByName(doc.Generators, "onlyTag"))
	assert.Equal(t, 1, countByName(doc.Generators, "onlyGenerators"))

	assert.Equal(t, 1, countByName(doc.Bundles, "dupbundle"))
	assert.Contains(t, descriptionsByName(doc.Bundles, "dupbundle"), "tag-bundle-desc")
	assert.Equal(t, 1, countByName(doc.Bundles, "onlyTagBundle"))
	assert.Equal(t, 1, countByName(doc.Bundles, "onlyGenBundle"))
}

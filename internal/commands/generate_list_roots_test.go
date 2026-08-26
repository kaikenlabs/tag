package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		filepath.Join(types.TemplatesDir, "dupgen", "gen.tmpl"):                                     "content",
		filepath.Join(types.GeneratorsDir, "dupgen", types.TemplateConfigFile):                      `{"description":"generators-desc"}`,
		filepath.Join(types.GeneratorsDir, "dupgen", "gen.tmpl"):                                    "content",
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

// TestUT_GenerateList_OmitsEmptyGeneratorDirs pins #436: an empty generator
// directory must not appear in `tag generate list`. The positive half is
// mandatory — a bare absence assertion would pass for free if the library
// entry never resolved at all.
func TestUT_GenerateList_OmitsEmptyGeneratorDirs(t *testing.T) {
	templateDir := setupLibEntryRoots(t, "roots-t436-g", map[string]string{
		filepath.Join(types.GeneratorsDir, "populated", "gen.tmpl"): "content",
	})
	emptyDir := filepath.Join(templateDir, types.GeneratorsDir, "empty")
	require.NoError(t, os.MkdirAll(emptyDir, 0o750))
	entries, err := os.ReadDir(emptyDir)
	require.NoError(t, err)
	require.Empty(t, entries, "fixture invariant: generator dir must actually be empty")

	cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t436-g")

	var buf bytes.Buffer
	err = generateList(cfg, true, &buf, formatJSON)
	require.NoError(t, err)

	var doc generatorListJSON
	dec := json.NewDecoder(&buf)
	require.NoError(t, dec.Decode(&doc))

	assert.Equal(t, 0, countByName(doc.Generators, "empty"), "an empty generator dir must not be listed")
	assert.Equal(t, 1, countByName(doc.Generators, "populated"), "a populated sibling must still be listed")
}

// TestUT_GenerateList_EmptyTagDirDoesNotHideGeneratorsRootEntry catches an
// implementation that filters empty directories AFTER appendNewByName's
// name-keyed dedup instead of before it: an empty .tag/foo would claim the
// name "foo" on collision, get dropped for being empty, and hide the real
// generator sitting in _generators/foo.
func TestUT_GenerateList_EmptyTagDirDoesNotHideGeneratorsRootEntry(t *testing.T) {
	const marker = "generators-root-desc-t436"
	templateDir := setupLibEntryRoots(t, "roots-t436-h", map[string]string{
		filepath.Join(types.GeneratorsDir, "foo", types.TemplateConfigFile): `{"description":"` + marker + `"}`,
		filepath.Join(types.GeneratorsDir, "foo", "gen.tmpl"):               "content",
	})
	tagFooDir := filepath.Join(templateDir, types.TemplatesDir, "foo")
	require.NoError(t, os.MkdirAll(tagFooDir, 0o750))
	entries, err := os.ReadDir(tagFooDir)
	require.NoError(t, err)
	require.Empty(t, entries, "fixture invariant: .tag/foo must actually be empty")

	cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t436-h")

	var buf bytes.Buffer
	err = generateList(cfg, true, &buf, formatJSON)
	require.NoError(t, err)

	var doc generatorListJSON
	dec := json.NewDecoder(&buf)
	require.NoError(t, dec.Decode(&doc))

	assert.Equal(t, 1, countByName(doc.Generators, "foo"), "foo must still be listed from _generators/")
	assert.Contains(t, descriptionsByName(doc.Generators, "foo"), marker)
}

// TestUT_CompleteGeneratorNames_OmitsEmptyGeneratorDirs is the shell
// completion analog of TestUT_GenerateList_OmitsEmptyGeneratorDirs. A
// listing that disagrees with what `generate` actually runs is worse than
// either bug alone.
func TestUT_CompleteGeneratorNames_OmitsEmptyGeneratorDirs(t *testing.T) {
	templateDir := setupLibEntryRoots(t, "roots-t436-i", map[string]string{
		filepath.Join(types.GeneratorsDir, "populated", "gen.tmpl"): "content",
	})
	emptyDir := filepath.Join(templateDir, types.GeneratorsDir, "empty")
	require.NoError(t, os.MkdirAll(emptyDir, 0o750))
	entries, err := os.ReadDir(emptyDir)
	require.NoError(t, err)
	require.Empty(t, entries, "fixture invariant: generator dir must actually be empty")

	cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t436-i")

	var buf bytes.Buffer
	completeGeneratorNames(cfg, &buf)

	names := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.NotContains(t, names, "empty")
	assert.Contains(t, names, "populated")
}

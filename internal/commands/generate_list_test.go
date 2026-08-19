package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/config"
	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/types"
)

// --- scanDirEntries ---

func TestUT_ScanDirEntries_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	result := scanDirEntries(dir)
	assert.Empty(t, result)
}

func TestUT_ScanDirEntries_SkipsDotAndUnderscorePrefixed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for _, name := range []string{"_private", ".hidden", "visible"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, name), 0o750))
	}

	result := scanDirEntries(dir)
	require.Len(t, result, 1)
	assert.Equal(t, "visible", result[0].Name)
}

// TestUT_ScanDirEntries_SkipsHistory reproduces B2: TAG's own .tag/history/ backup
// directory must not surface as a phantom generator in `tag generate list`.
func TestUT_ScanDirEntries_SkipsHistory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for _, name := range []string{"history", "realgen"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, name), 0o750))
	}

	result := scanDirEntries(dir)
	require.Len(t, result, 1)
	assert.Equal(t, "realgen", result[0].Name)
}

func TestUT_ScanDirEntries_SkipsFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "notadir.go"), []byte("x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "realgen"), 0o750))

	result := scanDirEntries(dir)
	require.Len(t, result, 1)
	assert.Equal(t, "realgen", result[0].Name)
}

func TestUT_ScanDirEntries_ReadsTemplateConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	genDir := filepath.Join(dir, "mygen")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	tc := map[string]any{
		"description": "A test generator",
		"requires":    []string{"use_db"},
	}
	data, err := json.Marshal(tc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(genDir, types.TemplateConfigFile), data, 0o644))

	result := scanDirEntries(dir)
	require.Len(t, result, 1)
	assert.Equal(t, "mygen", result[0].Name)
	assert.Equal(t, "A test generator", result[0].Description)
	assert.Equal(t, []string{"use_db"}, result[0].Requires)
}

func TestUT_ScanDirEntries_FallsBackToFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	genDir := filepath.Join(dir, "fmgen")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	tmpl := "---\nto: output.go\ndesc: Frontmatter description\n---\npackage main\n"
	require.NoError(t, os.WriteFile(filepath.Join(genDir, "fmgen.go"), []byte(tmpl), 0o644))

	result := scanDirEntries(dir)
	require.Len(t, result, 1)
	assert.Equal(t, "Frontmatter description", result[0].Description)
}

func TestUT_ScanDirEntries_NonexistentDir(t *testing.T) {
	t.Parallel()
	result := scanDirEntries("/nonexistent/path/that/does/not/exist")
	assert.Nil(t, result)
}

// --- scanBundles ---

func TestUT_ScanBundles_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	result := scanBundles(dir)
	assert.Empty(t, result)
}

func TestUT_ScanBundles_ReadsManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bundleDir := filepath.Join(dir, "mybundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0o750))

	bundle := engine.Bundle{
		Description: "A test bundle",
		Requires:    []string{"use_api"},
	}
	data, err := json.Marshal(bundle)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "mybundle"+types.BundleExtension), data, 0o644))

	result := scanBundles(dir)
	require.Len(t, result, 1)
	assert.Equal(t, "mybundle", result[0].Name)
	assert.Equal(t, "A test bundle", result[0].Description)
	assert.Equal(t, []string{"use_api"}, result[0].Requires)
}

func TestUT_ScanBundles_NoManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "bare"), 0o750))

	result := scanBundles(dir)
	require.Len(t, result, 1)
	assert.Equal(t, "bare", result[0].Name)
	assert.Empty(t, result[0].Description)
}

// --- readFrontmatterDesc ---

func TestUT_ReadFrontmatterDesc_WithDescription(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmpl := "---\nto: output.go\ndesc: My generator\n---\npackage main\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gen.go"), []byte(tmpl), 0o644))

	assert.Equal(t, "My generator", readFrontmatterDesc(dir))
}

func TestUT_ReadFrontmatterDesc_NoFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.go"), []byte("package main\n"), 0o644))

	assert.Equal(t, "", readFrontmatterDesc(dir))
}

func TestUT_ReadFrontmatterDesc_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assert.Equal(t, "", readFrontmatterDesc(dir))
}

func TestUT_ReadFrontmatterDesc_SkipsSubdirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o750))

	assert.Equal(t, "", readFrontmatterDesc(dir))
}

// --- filterByRequirements ---

func TestUT_FilterByRequirements_AllMet(t *testing.T) {
	t.Parallel()
	items := []GeneratorInfo{
		{Name: "gen1", Requires: []string{"use_db"}},
		{Name: "gen2", Requires: []string{"use_api"}},
	}
	vars := map[string]any{"use_db": true, "use_api": true}

	result := filterByRequirements(items, vars)
	assert.Len(t, result, 2)
}

func TestUT_FilterByRequirements_SomeUnmet(t *testing.T) {
	t.Parallel()
	items := []GeneratorInfo{
		{Name: "gen1", Requires: []string{"use_db"}},
		{Name: "gen2", Requires: []string{"use_api"}},
	}
	vars := map[string]any{"use_db": true}

	result := filterByRequirements(items, vars)
	require.Len(t, result, 1)
	assert.Equal(t, "gen1", result[0].Name)
}

func TestUT_FilterByRequirements_NoRequirements(t *testing.T) {
	t.Parallel()
	items := []GeneratorInfo{
		{Name: "gen1"},
		{Name: "gen2"},
	}
	vars := map[string]any{}

	result := filterByRequirements(items, vars)
	assert.Len(t, result, 2)
}

func TestUT_FilterByRequirements_NilVars(t *testing.T) {
	t.Parallel()
	items := []GeneratorInfo{
		{Name: "no_req"},
		{Name: "has_req", Requires: []string{"use_db"}},
	}

	result := filterByRequirements(items, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "no_req", result[0].Name)
}

// --- printGeneratorLine ---

func TestUT_PrintGeneratorLine_NameAndDescription(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	printGeneratorLine(&buf, GeneratorInfo{Name: "model", Description: "Generate a model"})

	assert.Contains(t, buf.String(), "model")
	assert.Contains(t, buf.String(), "Generate a model")
}

func TestUT_PrintGeneratorLine_NameOnly(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	printGeneratorLine(&buf, GeneratorInfo{Name: "simple"})

	out := buf.String()
	assert.Contains(t, out, "simple")
	// Name-only format: "  %s\n" (no padding)
	assert.Equal(t, "  simple\n", out)
}

func TestUT_PrintGeneratorLine_WithRequires(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	printGeneratorLine(&buf, GeneratorInfo{
		Name:        "dbgen",
		Description: "DB generator",
		Requires:    []string{"use_db", "use_orm"},
	})

	out := buf.String()
	assert.Contains(t, out, "dbgen")
	assert.Contains(t, out, "[requires: use_db, use_orm]")
}

// --- generateList ---

func TestUT_GenerateList_WithTemplateOriginHeader(t *testing.T) {
	// cfg.Template is set, so collectGeneratorLists calls the overridable
	// newLocalLibrary var. seedLibrary stubs it to an isolated empty library;
	// without it this test would hit the developer's real library. Not
	// parallel: seedLibrary mutates a package-level var and races with any
	// sibling test doing the same.
	seedLibrary(t)

	dir := t.TempDir()
	cfg := createTestConfig(t, dir)
	cfg.Template = &config.TemplateOrigin{
		Name:    "mytemplate",
		Source:  "gh:acme/mytemplate",
		Version: "v1.2.0",
	}

	// Create a local generator so the list isn't empty
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "svc"), 0o750))

	var buf bytes.Buffer
	err := generateList(cfg, false, &buf, formatText)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "gh:acme/mytemplate@v1.2.0")
}

func TestUT_GenerateList_TemplateOriginNoVersion(t *testing.T) {
	// cfg.Template is set, so collectGeneratorLists calls the overridable
	// newLocalLibrary var. seedLibrary stubs it to an isolated empty library;
	// without it this test would hit the developer's real library. Not
	// parallel: seedLibrary mutates a package-level var and races with any
	// sibling test doing the same.
	seedLibrary(t)

	dir := t.TempDir()
	cfg := createTestConfig(t, dir)
	cfg.Template = &config.TemplateOrigin{
		Name:   "mytemplate",
		Source: "gh:acme/mytemplate",
	}

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "svc"), 0o750))

	var buf bytes.Buffer
	err := generateList(cfg, false, &buf, formatText)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "gh:acme/mytemplate")
	assert.NotContains(t, out, "@")
}

func TestUT_GenerateList_NoTemplateOriginHint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := createTestConfig(t, dir)

	var buf bytes.Buffer
	err := generateList(cfg, false, &buf, formatText)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "This project was not scaffolded from a library template.")
}

func TestUT_GenerateList_LocalWithBundles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := createTestConfig(t, dir)

	// Create a local generator
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "model"), 0o750))
	// Create a local bundle
	bundleDir := filepath.Join(dir, "_bundles", "full-stack")
	require.NoError(t, os.MkdirAll(bundleDir, 0o750))
	bundle := engine.Bundle{Description: "Full stack scaffold"}
	data, err := json.Marshal(bundle)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "full-stack.json"), data, 0o644))

	var buf bytes.Buffer
	err = generateList(cfg, false, &buf, formatText)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "PROJECT GENERATORS")
	assert.Contains(t, out, "BUNDLES")
	assert.Contains(t, out, "full-stack")
	assert.Contains(t, out, "tag generate <name> <target>")
}

// --- collectGeneratorLists ---

func TestUT_CollectGeneratorLists_LocalOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := createTestConfig(t, dir)

	// Create local generator
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ctrl"), 0o750))

	lists := collectGeneratorLists(cfg)
	assert.Empty(t, lists.templateName)
	require.Len(t, lists.localGens, 1)
	assert.Equal(t, "ctrl", lists.localGens[0].Name)
}

func TestUT_CollectGeneratorLists_WithLibrary(t *testing.T) {
	// Cannot t.Parallel() — setupFakeLibrary mutates package state
	templateName := "mylib"
	templateDir := setupFakeLibrary(t, templateName)

	// Create a generator inside the library template's .tag dir
	libTagDir := filepath.Join(templateDir, types.TemplatesDir)
	genDir := filepath.Join(libTagDir, "libgen")
	require.NoError(t, os.MkdirAll(genDir, 0o750))

	localDir := t.TempDir()
	cfg := createTestConfigWithLib(t, localDir, templateName)

	lists := collectGeneratorLists(cfg)
	assert.Equal(t, templateName, lists.templateName)
	require.Len(t, lists.templateGens, 1)
	assert.Equal(t, "libgen", lists.templateGens[0].Name)
}

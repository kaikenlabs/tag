package templateupdate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_RenderPair_Success(t *testing.T) {
	oldConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"}}}`
	newConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"}}}`

	oldFixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: oldConfig,
		"README.md":              "# {{ vars.project_name }} v1",
		"main.go":                "package main // old",
	})

	newFixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: newConfig,
		"README.md":              "# {{ vars.project_name }} v2",
		"main.go":                "package main // new",
		"extra.go":               "package extra",
	})

	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: oldFixture,
		newSHA: newFixture,
	}}
	renderer := NewHistoricalRenderer(fetcher)

	vars := map[string]any{"project_name": "myapp"}

	base, theirs, err := renderer.RenderPair(context.Background(), "https://github.com/test/repo.git", oldSHA, newSHA, vars)

	require.NoError(t, err)

	// Base (old) should have 2 files.
	assert.Len(t, base, 2)
	assert.Equal(t, "# myapp v1", string(base["README.md"].Content))
	assert.Equal(t, "package main // old", string(base["main.go"].Content))

	// Theirs (new) should have 3 files.
	assert.Len(t, theirs, 3)
	assert.Equal(t, "# myapp v2", string(theirs["README.md"].Content))
	assert.Equal(t, "package main // new", string(theirs["main.go"].Content))
	assert.Equal(t, "package extra", string(theirs["extra.go"].Content))
}

func TestUT_RenderPair_BaseFetchError(t *testing.T) {
	fetcher := &commitFetcher{
		fixtures: map[string]string{},
		err:      assert.AnError,
	}
	renderer := NewHistoricalRenderer(fetcher)

	_, _, err := renderer.RenderPair(context.Background(), "https://github.com/test/repo.git", oldSHA, newSHA, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "render base")
}

func TestUT_RenderPair_TheirsFetchError(t *testing.T) {
	config := `{"name":"test","vars":{}}`
	oldFixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: config,
		"file.txt":               "content",
	})

	// Only old commit exists, new commit will fail.
	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: oldFixture,
	}}
	renderer := NewHistoricalRenderer(fetcher)

	_, _, err := renderer.RenderPair(context.Background(), "https://github.com/test/repo.git", oldSHA, newSHA, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "render theirs")
}

func TestUT_LoadConfigAtCommit_Success(t *testing.T) {
	templateJSON := `{"name":"my-template","vars":{"db_name":{"type":"string","default":"postgres"},"use_cache":{"type":"boolean","default":true}}}`

	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: templateJSON,
		"main.go":                "package main",
	})

	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: fixture,
	}}
	renderer := NewHistoricalRenderer(fetcher)

	cfg, err := renderer.LoadConfigAtCommit(context.Background(), "https://github.com/test/repo.git", oldSHA)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "my-template", cfg.Name)
	assert.Contains(t, cfg.Vars, "db_name")
	assert.Contains(t, cfg.Vars, "use_cache")
}

func TestUT_LoadConfigAtCommit_FetchError(t *testing.T) {
	fetcher := &commitFetcher{
		fixtures: map[string]string{},
		err:      assert.AnError,
	}
	renderer := NewHistoricalRenderer(fetcher)

	cfg, err := renderer.LoadConfigAtCommit(context.Background(), "https://github.com/test/repo.git", oldSHA)

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "fetch template at commit")
}

func TestUT_LoadConfigAtCommit_MissingConfigFile(t *testing.T) {
	fixture := setupFixture(t, map[string]any{
		"main.go": "package main",
	})

	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: fixture,
	}}
	renderer := NewHistoricalRenderer(fetcher)

	cfg, err := renderer.LoadConfigAtCommit(context.Background(), "https://github.com/test/repo.git", oldSHA)

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), types.TemplateConfigFile)
}

func TestUT_LoadConfigAtCommit_InvalidJSON(t *testing.T) {
	fixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: "not valid json{{{",
	})

	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: fixture,
	}}
	renderer := NewHistoricalRenderer(fetcher)

	cfg, err := renderer.LoadConfigAtCommit(context.Background(), "https://github.com/test/repo.git", oldSHA)

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "parse")
}

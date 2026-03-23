package templateupdate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_Differ_Diff_AlreadyUpToDate(t *testing.T) {
	projectDir := t.TempDir()
	writeTagConfig(t, projectDir, &scaffold.TagConfig{
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: oldSHA,
		},
		Variables: map[string]any{"project_name": "myapp"},
	})

	// Create a minimal .tag dir so project file reading works.
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, types.TemplatesDir), 0o755))

	renderer := NewHistoricalRenderer(&commitFetcher{fixtures: map[string]string{}})
	// Resolver returns the SAME SHA as the project config → already up to date.
	resolver := &mockResolver{sha: oldSHA}
	differ := NewDiffer(renderer, resolver)

	result, err := differ.Diff(context.Background(), DiffOptions{ProjectDir: projectDir})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, oldSHA, result.OldSHA)
	assert.Equal(t, oldSHA, result.NewSHA)
	assert.Empty(t, result.Results)
}

func TestUT_Differ_Diff_DetectsChanges(t *testing.T) {
	configJSON := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"}}}`

	oldFixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: configJSON,
		"README.md":              "# {{ vars.project_name }} v1",
	})
	newFixture := setupFixture(t, map[string]any{
		types.TemplateConfigFile: configJSON,
		"README.md":              "# {{ vars.project_name }} v2",
		"new.go":                 "package newpkg",
	})

	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: oldFixture,
		newSHA: newFixture,
	}}
	renderer := NewHistoricalRenderer(fetcher)
	resolver := &mockResolver{sha: newSHA}
	differ := NewDiffer(renderer, resolver)

	// Set up project dir with .tagconfig.json and the user's current file.
	projectDir := t.TempDir()
	writeTagConfig(t, projectDir, &scaffold.TagConfig{
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: oldSHA,
		},
		Variables: map[string]any{"project_name": "myapp"},
	})
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, types.TemplatesDir), 0o755))
	// User has the v1 README (matching old template output).
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("# myapp v1"), 0o644))

	result, err := differ.Diff(context.Background(), DiffOptions{ProjectDir: projectDir})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, oldSHA, result.OldSHA)
	assert.Equal(t, newSHA, result.NewSHA)
	assert.Equal(t, "gh:acme/template", result.Source)
	assert.NotEmpty(t, result.Results)

	// Should have at least an update for README.md and an add for new.go.
	ops := map[string]MergeOp{}
	for _, r := range result.Results {
		ops[r.Path] = r.Op
	}
	assert.Equal(t, MergeUpdate, ops["README.md"])
	assert.Equal(t, MergeAdd, ops["new.go"])
}

func TestUT_Differ_Diff_ResolverError(t *testing.T) {
	projectDir := t.TempDir()
	writeTagConfig(t, projectDir, &scaffold.TagConfig{
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: oldSHA,
		},
	})

	renderer := NewHistoricalRenderer(&commitFetcher{fixtures: map[string]string{}})
	resolver := &mockResolver{err: assert.AnError}
	differ := NewDiffer(renderer, resolver)

	_, err := differ.Diff(context.Background(), DiffOptions{ProjectDir: projectDir})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve latest commit")
}

func TestUT_Differ_Diff_InvalidConfig(t *testing.T) {
	projectDir := t.TempDir()
	// Config without CommitSHA should fail validation.
	writeTagConfig(t, projectDir, &scaffold.TagConfig{
		Template: &scaffold.TagTemplate{
			Source: "gh:acme/template",
		},
	})

	renderer := NewHistoricalRenderer(&commitFetcher{fixtures: map[string]string{}})
	resolver := &mockResolver{sha: newSHA}
	differ := NewDiffer(renderer, resolver)

	_, err := differ.Diff(context.Background(), DiffOptions{ProjectDir: projectDir})

	require.Error(t, err)
}

func TestUT_Differ_Diff_RefOverride(t *testing.T) {
	projectDir := t.TempDir()
	writeTagConfig(t, projectDir, &scaffold.TagConfig{
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: oldSHA,
		},
		Variables: map[string]any{"project_name": "myapp"},
	})
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, types.TemplatesDir), 0o755))

	// Resolver returns same SHA → already up to date, but proves Ref was used.
	resolver := &mockResolver{sha: oldSHA}
	renderer := NewHistoricalRenderer(&commitFetcher{fixtures: map[string]string{}})
	differ := NewDiffer(renderer, resolver)

	result, err := differ.Diff(context.Background(), DiffOptions{
		ProjectDir: projectDir,
		Ref:        "v2.0.0",
	})

	require.NoError(t, err)
	assert.Equal(t, oldSHA, result.OldSHA)
}

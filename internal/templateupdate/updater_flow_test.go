package templateupdate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
	"github.com/kaikenlabs/tag/internal/types"
)

// commitFetcher returns different fixtures depending on the requested commit SHA.
type commitFetcher struct {
	fixtures map[string]string // commitSHA → fixture dir path
	err      error
}

func (f *commitFetcher) FetchAtCommit(_ context.Context, _, commitSHA, destDir string) (string, error) {
	if f.err != nil {
		return "", f.err
	}

	fixtureDir, ok := f.fixtures[commitSHA]
	if !ok {
		return "", os.ErrNotExist
	}

	return destDir, copyDir(fixtureDir, destDir)
}

const (
	oldSHA = "aaa111aaa111aaa111aaa111aaa111aaa111aaa1"
	newSHA = "bbb222bbb222bbb222bbb222bbb222bbb222bbb2"
)

// makeTemplateFixture creates a template directory with tag.template.json and optional extra files.
func makeTemplateFixture(t *testing.T, configJSON string, files map[string]string) string {
	t.Helper()

	fixture := map[string]any{
		types.TemplateConfigFile: configJSON,
	}
	for path, content := range files {
		fixture[path] = content
	}

	return setupFixture(t, fixture)
}

// setupFlowProject creates a project directory with .tagconfig.json pointing to oldSHA,
// and writes the given project files.
func setupFlowProject(t *testing.T, projectFiles map[string]string) string {
	t.Helper()

	dir := t.TempDir()

	writeTagConfig(t, dir, &scaffold.TagConfig{
		SchemaVersion: 1,
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: oldSHA,
		},
		Variables: map[string]any{
			"project_name": "myproject",
		},
	})

	for path, content := range projectFiles {
		fullPath := filepath.Join(dir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), types.DirMode))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
	}

	return dir
}

func TestUT_Updater_ApplyUpdate_HappyPath(t *testing.T) {
	templateConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"}}}`

	oldFixture := makeTemplateFixture(t, templateConfig, map[string]string{
		"README.md": "# {{ vars.project_name }}",
		"main.go":   "package main // v1",
	})

	newFixture := makeTemplateFixture(t, templateConfig, map[string]string{
		"README.md": "# {{ vars.project_name }}",
		"main.go":   "package main // v2",
		"new.go":    "package main // new file",
	})

	projectDir := setupFlowProject(t, map[string]string{
		"README.md": "# myproject",
		"main.go":   "package main // v1",
	})

	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: oldFixture,
		newSHA: newFixture,
	}}
	resolver := &mockResolver{sha: newSHA}
	renderer := NewHistoricalRenderer(fetcher)
	updater := NewUpdater(renderer, resolver)

	result, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: projectDir,
		Backup:     true,
	})

	require.NoError(t, err)
	assert.Equal(t, oldSHA, result.OldSHA)
	assert.Equal(t, newSHA, result.NewSHA)
	assert.Equal(t, 1, result.NewFiles, "new.go should be added")
	assert.Equal(t, 1, result.UpdatedFiles, "main.go should be updated")

	// Verify files on disk.
	content, err := os.ReadFile(filepath.Join(projectDir, "main.go"))
	require.NoError(t, err)
	assert.Equal(t, "package main // v2", string(content))

	content, err = os.ReadFile(filepath.Join(projectDir, "new.go"))
	require.NoError(t, err)
	assert.Equal(t, "package main // new file", string(content))

	// Verify .tagconfig.json updated with new SHA.
	cfg, loadErr := scaffold.LoadTagConfig(projectDir)
	require.NoError(t, loadErr)
	assert.Equal(t, newSHA, cfg.Template.CommitSHA)
}

func TestUT_Updater_ApplyUpdate_DryRun(t *testing.T) {
	templateConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"}}}`

	oldFixture := makeTemplateFixture(t, templateConfig, map[string]string{
		"main.go": "package main // v1",
	})

	newFixture := makeTemplateFixture(t, templateConfig, map[string]string{
		"main.go": "package main // v2",
	})

	projectDir := setupFlowProject(t, map[string]string{
		"main.go": "package main // v1",
	})

	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: oldFixture,
		newSHA: newFixture,
	}}
	resolver := &mockResolver{sha: newSHA}
	renderer := NewHistoricalRenderer(fetcher)
	updater := NewUpdater(renderer, resolver)

	result, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: projectDir,
		DryRun:     true,
	})

	require.NoError(t, err)
	assert.Equal(t, oldSHA, result.OldSHA)
	assert.Equal(t, newSHA, result.NewSHA)
	assert.Equal(t, 1, result.UpdatedFiles)

	// Verify no files changed on disk.
	content, err := os.ReadFile(filepath.Join(projectDir, "main.go"))
	require.NoError(t, err)
	assert.Equal(t, "package main // v1", string(content), "dry-run should not modify files")

	// Verify tagconfig not updated.
	cfg, loadErr := scaffold.LoadTagConfig(projectDir)
	require.NoError(t, loadErr)
	assert.Equal(t, oldSHA, cfg.Template.CommitSHA, "dry-run should not update tagconfig")
}

func TestUT_Updater_ApplyUpdate_WithVarChanges(t *testing.T) {
	oldConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"}}}`
	newConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"},"use_db":{"type":"boolean","default":false}}}`

	oldFixture := makeTemplateFixture(t, oldConfig, map[string]string{
		"main.go": "package main",
	})
	newFixture := makeTemplateFixture(t, newConfig, map[string]string{
		"main.go": "package main",
	})

	projectDir := setupFlowProject(t, map[string]string{
		"main.go": "package main",
	})

	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: oldFixture,
		newSHA: newFixture,
	}}
	resolver := &mockResolver{sha: newSHA}
	renderer := NewHistoricalRenderer(fetcher)
	updater := NewUpdater(renderer, resolver)

	result, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: projectDir,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.VarChanges, "should detect added use_db variable")

	foundUseDB := false
	for _, vc := range result.VarChanges {
		if vc.Name == "use_db" && vc.Type == VarAdded {
			foundUseDB = true
		}
	}
	assert.True(t, foundUseDB, "should report use_db as added")
}

func TestUT_Updater_ApplyUpdate_WithHookChanges(t *testing.T) {
	oldConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"}}}`
	newConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"}},"hooks":{"post_scaffold":["echo done"]}}`

	oldFixture := makeTemplateFixture(t, oldConfig, map[string]string{
		"main.go": "package main",
	})
	newFixture := makeTemplateFixture(t, newConfig, map[string]string{
		"main.go": "package main",
	})

	projectDir := setupFlowProject(t, map[string]string{
		"main.go": "package main",
	})

	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: oldFixture,
		newSHA: newFixture,
	}}
	resolver := &mockResolver{sha: newSHA}
	renderer := NewHistoricalRenderer(fetcher)
	updater := NewUpdater(renderer, resolver)

	result, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: projectDir,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.HookChanges, "should detect added post_scaffold hook")

	foundHook := false
	for _, hc := range result.HookChanges {
		if hc.Phase == "post_scaffold" && hc.Type == HookAdded {
			foundHook = true
		}
	}
	assert.True(t, foundHook, "should report post_scaffold hook as added")
}

func TestUT_Updater_ContinueUpdate_AfterConflictResolution(t *testing.T) {
	dir := setupFlowProject(t, map[string]string{
		"file.txt": "resolved content without conflict markers",
	})

	// Write conflict status.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, types.TemplatesDir), 0o755))
	conflictStatus := &ConflictStatus{
		SchemaVersion:   1,
		UpdateCommit:    newSHA,
		ConflictedFiles: []string{"file.txt"},
		ResolvedFiles:   []string{},
	}
	data, err := json.MarshalIndent(conflictStatus, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, types.TemplatesDir, "conflicts.json"),
		data, 0o644,
	))

	resolver := &mockResolver{}
	renderer := NewHistoricalRenderer(&mockFetcher{})
	updater := NewUpdater(renderer, resolver)

	result, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: dir,
		Mode:       UpdateModeContinue,
	})

	require.NoError(t, err)
	assert.Equal(t, newSHA, result.NewSHA)

	// Verify conflicts.json cleared.
	status, readErr := ReadConflictStatus(dir)
	require.NoError(t, readErr)
	assert.Nil(t, status, "conflict status should be cleared after continue")

	// Verify tagconfig updated with new commit.
	cfg, loadErr := scaffold.LoadTagConfig(dir)
	require.NoError(t, loadErr)
	assert.Equal(t, newSHA, cfg.Template.CommitSHA)
}

func TestUT_Updater_ContinueUpdate_UnresolvedMarkers(t *testing.T) {
	dir := setupFlowProject(t, map[string]string{
		"file.txt": "<<<<<<< LOCAL\nours\n=======\ntheirs\n>>>>>>> TEMPLATE",
	})

	// Write conflict status pointing to file.txt.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, types.TemplatesDir), 0o755))
	conflictStatus := &ConflictStatus{
		SchemaVersion:   1,
		UpdateCommit:    newSHA,
		ConflictedFiles: []string{"file.txt"},
	}
	data, err := json.MarshalIndent(conflictStatus, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, types.TemplatesDir, "conflicts.json"),
		data, 0o644,
	))

	resolver := &mockResolver{}
	renderer := NewHistoricalRenderer(&mockFetcher{})
	updater := NewUpdater(renderer, resolver)

	_, err = updater.Update(context.Background(), UpdateOptions{
		ProjectDir: dir,
		Mode:       UpdateModeContinue,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unresolved conflict markers")
}

func TestUT_Updater_AbortUpdate_WithBackup(t *testing.T) {
	projectDir := setupFlowProject(t, map[string]string{
		"main.go": "package main // original",
	})

	// Create a backup with manifest.
	results := []MergeResult{
		{Path: "main.go", Op: MergeUpdate, Content: []byte("package main // updated"), Mode: 0o644},
	}
	backupPath, backupErr := CreateBackupFromResults(projectDir, results, oldSHA, newSHA)
	require.NoError(t, backupErr)
	require.NotEmpty(t, backupPath)

	// Simulate the update having modified the file.
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "main.go"), []byte("package main // updated"), 0o644))

	// Write conflict status so abort has something to clear.
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, types.TemplatesDir), 0o755))
	conflictStatus := &ConflictStatus{
		SchemaVersion:   1,
		UpdateCommit:    newSHA,
		ConflictedFiles: []string{"main.go"},
	}
	csData, marshalErr := json.MarshalIndent(conflictStatus, "", "  ")
	require.NoError(t, marshalErr)
	require.NoError(t, os.WriteFile(
		filepath.Join(projectDir, types.TemplatesDir, "conflicts.json"),
		csData, 0o644,
	))

	resolver := &mockResolver{}
	renderer := NewHistoricalRenderer(&mockFetcher{})
	updater := NewUpdater(renderer, resolver)

	result, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: projectDir,
		Mode:       UpdateModeAbort,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify file restored to original content.
	content, readErr := os.ReadFile(filepath.Join(projectDir, "main.go"))
	require.NoError(t, readErr)
	assert.Equal(t, "package main // original", string(content))

	// Verify conflict status cleared.
	status, statusErr := ReadConflictStatus(projectDir)
	require.NoError(t, statusErr)
	assert.Nil(t, status)

	// Verify backup removed.
	_, statErr := os.Stat(backupPath)
	assert.True(t, os.IsNotExist(statErr), "backup should be removed after abort")
}

func TestUT_FinalizeUpdate_WritesConflictStatus(t *testing.T) {
	dir := t.TempDir()

	report := &ConflictReport{
		Conflicts: []ConflictedFile{
			{Path: "conflict.go", MarkerCount: 1},
		},
	}

	cfg := &scaffold.TagConfig{
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: oldSHA,
		},
		Variables: map[string]any{"project_name": "myproject"},
	}

	result := &UpdateResult{
		Applied: []MergeResult{
			{Path: "conflict.go", Op: MergeConflict, Content: []byte("<<<"), Mode: 0o644},
		},
	}

	err := finalizeUpdate(dir, result, report, cfg, newSHA, "", map[string]any{"project_name": "myproject"})
	require.NoError(t, err)

	// Verify conflict status was written.
	status, readErr := ReadConflictStatus(dir)
	require.NoError(t, readErr)
	require.NotNil(t, status)
	assert.Equal(t, newSHA, status.UpdateCommit)
	assert.Contains(t, status.ConflictedFiles, "conflict.go")

	// Verify tagconfig was NOT updated (because there are conflicts).
	_, statErr := os.Stat(filepath.Join(dir, types.TagConfigFile))
	assert.True(t, os.IsNotExist(statErr), "tagconfig should not be written when conflicts exist")
}

func TestUT_FinalizeUpdate_NoConflicts_UpdatesTagConfig(t *testing.T) {
	dir := t.TempDir()

	report := &ConflictReport{}

	cfg := &scaffold.TagConfig{
		Template: &scaffold.TagTemplate{
			Source:    "gh:acme/template",
			CommitSHA: oldSHA,
		},
		Variables: map[string]any{"project_name": "myproject"},
	}

	result := &UpdateResult{
		Applied: []MergeResult{
			{Path: "main.go", Op: MergeUpdate, Content: []byte("updated"), Mode: 0o644},
		},
	}

	err := finalizeUpdate(dir, result, report, cfg, newSHA, "v2.0", map[string]any{"project_name": "myproject"})
	require.NoError(t, err)

	// Verify tagconfig was updated.
	loadedCfg, loadErr := scaffold.LoadTagConfig(dir)
	require.NoError(t, loadErr)
	assert.Equal(t, newSHA, loadedCfg.Template.CommitSHA)
	assert.Equal(t, "v2.0", loadedCfg.Template.Ref)

	// Verify no conflict status file.
	status, readErr := ReadConflictStatus(dir)
	require.NoError(t, readErr)
	assert.Nil(t, status)
}

func TestUT_Updater_ApplyUpdate_NewRequiredVarMissing(t *testing.T) {
	oldConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"}}}`
	newConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"},"api_key":{"type":"string","required":true}}}`

	oldFixture := makeTemplateFixture(t, oldConfig, map[string]string{
		"main.go": "package main",
	})
	newFixture := makeTemplateFixture(t, newConfig, map[string]string{
		"main.go": "package main",
	})

	projectDir := setupFlowProject(t, map[string]string{
		"main.go": "package main",
	})

	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: oldFixture,
		newSHA: newFixture,
	}}
	resolver := &mockResolver{sha: newSHA}
	renderer := NewHistoricalRenderer(fetcher)
	updater := NewUpdater(renderer, resolver)

	_, err := updater.Update(context.Background(), UpdateOptions{
		ProjectDir: projectDir,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "new required variable")
	assert.Contains(t, err.Error(), "api_key")
}

func TestUT_DetectConfigChanges_VarsAndHooks(t *testing.T) {
	oldConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"}}}`
	newConfig := `{"name":"test","vars":{"project_name":{"type":"string","default":"myproject"},"use_db":{"type":"boolean","default":true}},"hooks":{"pre_scaffold":["make deps"]}}`

	oldFixture := makeTemplateFixture(t, oldConfig, nil)
	newFixture := makeTemplateFixture(t, newConfig, nil)

	fetcher := &commitFetcher{fixtures: map[string]string{
		oldSHA: oldFixture,
		newSHA: newFixture,
	}}
	renderer := NewHistoricalRenderer(fetcher)
	resolver := &mockResolver{sha: newSHA}
	updater := NewUpdater(renderer, resolver)

	rctx := &resolveUpdateContext{
		cfg: &scaffold.TagConfig{
			Template: &scaffold.TagTemplate{
				Source:    "gh:acme/template",
				CommitSHA: oldSHA,
			},
		},
		ref:       &remote.Reference{URL: "gh:acme/template"},
		latestSHA: newSHA,
	}

	changes, err := updater.detectConfigChanges(context.Background(), rctx)
	require.NoError(t, err)

	assert.NotEmpty(t, changes.vars, "should detect variable changes")
	assert.NotEmpty(t, changes.hooks, "should detect hook changes")

	// Check var changes contain use_db added.
	foundVar := false
	for _, vc := range changes.vars {
		if vc.Name == "use_db" && vc.Type == VarAdded {
			foundVar = true
		}
	}
	assert.True(t, foundVar)

	// Check hook changes contain pre_scaffold added.
	foundHook := false
	for _, hc := range changes.hooks {
		if hc.Phase == "pre_scaffold" && hc.Type == HookAdded {
			foundHook = true
		}
	}
	assert.True(t, foundHook)

	_ = resolver
}

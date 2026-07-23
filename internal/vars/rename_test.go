package vars

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTree materialises a map of relative path -> content under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
}

// snapshotTree reads every file under root into a map of relative path -> content.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		require.NoError(t, err)
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		require.NoError(t, relErr)
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		out[filepath.ToSlash(rel)] = string(content)
		return nil
	}))
	return out
}

// renameTree builds a template, plans a rename and applies it, returning the
// plan and the resulting tree.
func renameTree(t *testing.T, files map[string]string, oldName, newName string) (*RenamePlan, string) {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, files)

	plan, err := PlanRename(root, oldName, newName)
	require.NoError(t, err)
	require.NoError(t, plan.Apply())

	return plan, root
}

const basicConfig = `{
  "name": "demo",
  "vars": {
    "old": { "type": "string", "prompt": "Old" },
    "module": { "default": "github.com/acme/{{ vars.old | kebab }}" }
  }
}`

func TestUT_PlanRename_UpdatesConfigAndTemplates(t *testing.T) {
	t.Parallel()

	plan, root := renameTree(t, map[string]string{
		"tag.template.json": basicConfig,
		"README.md":         "# {{ vars.old }}\n\nThe old project.\n",
		"go.mod":            "module {{ vars.old | kebab }}\n",
	}, "old", "renamed")

	tree := snapshotTree(t, root)

	assert.Contains(t, tree["tag.template.json"], `"renamed": { "type": "string", "prompt": "Old" }`)
	assert.Contains(t, tree["tag.template.json"], `github.com/acme/{{ vars.renamed | kebab }}`)
	assert.Equal(t, "# {{ vars.renamed }}\n\nThe old project.\n", tree["README.md"])
	assert.Equal(t, "module {{ vars.renamed | kebab }}\n", tree["go.mod"])

	assert.Equal(t, "old", plan.OldName)
	assert.Equal(t, "renamed", plan.NewName)
}

func TestUT_PlanRename_ReportsTotals(t *testing.T) {
	t.Parallel()

	plan, _ := renameTree(t, map[string]string{
		"tag.template.json": `{"vars": {"old": 1}}`,
		"a.txt":             "{{ vars.old }} {{ vars.old }}\n",
		"b.txt":             "{% if vars.old %}x{% endif %}\n",
		"c.txt":             "nothing here\n",
	}, "old", "renamed")

	assert.Equal(t, 3, plan.FileCount(), "only files that actually changed are counted")
	assert.Equal(t, 4, plan.ReplacementCount(), "1 config key + 2 in a.txt + 1 in b.txt")

	var paths []string
	for _, f := range plan.Files {
		paths = append(paths, f.Path)
	}
	assert.NotContains(t, paths, "c.txt")
}

func TestUT_PlanRename_ReportsLineNumbers(t *testing.T) {
	t.Parallel()

	plan, _ := renameTree(t, map[string]string{
		"tag.template.json": `{"vars": {"old": 1}}`,
		"README.md":         "line one\nline two\n# {{ vars.old }}\nline four\n",
	}, "old", "renamed")

	var readme *FileChange
	for i := range plan.Files {
		if plan.Files[i].Path == "README.md" {
			readme = &plan.Files[i]
		}
	}
	require.NotNil(t, readme)
	require.Len(t, readme.Changes, 1)
	assert.Equal(t, 3, readme.Changes[0].Line)
	assert.Equal(t, "# {{ vars.old }}", readme.Changes[0].Before)
	assert.Equal(t, "# {{ vars.renamed }}", readme.Changes[0].After)
}

func TestUT_PlanRename_UpdatesGeneratorScopes(t *testing.T) {
	t.Parallel()

	_, root := renameTree(t, map[string]string{
		"tag.template.json":                         `{"vars": {"old": 1}}`,
		"_generators/api/tag.template.json":         `{"vars": {"old": 2}, "requires": ["old"]}`,
		"_generators/api/templates/handler.go.t":    "// {{ vars.old }}\n",
		"_generators/_bundles/crud/crud.json":       `{"requires": ["old"], "generators": []}`,
		".tag/_bundles/feature/feature.bundle.json": `{"requires": ["old"]}`,
	}, "old", "renamed")

	tree := snapshotTree(t, root)

	assert.Contains(t, tree["_generators/api/tag.template.json"], `"renamed": 2`)
	assert.Contains(t, tree["_generators/api/tag.template.json"], `"requires": ["renamed"]`)
	assert.Equal(t, "// {{ vars.renamed }}\n", tree["_generators/api/templates/handler.go.t"])
	assert.Contains(t, tree["_generators/_bundles/crud/crud.json"], `"requires": ["renamed"]`)
	assert.Contains(t, tree[".tag/_bundles/feature/feature.bundle.json"], `"requires": ["renamed"]`)
}

func TestUT_PlanRename_RenamesPathPlaceholders(t *testing.T) {
	t.Parallel()

	plan, root := renameTree(t, map[string]string{
		"tag.template.json":                             `{"vars": {"old": 1}}`,
		"{{ vars.old | snake }}/main.go":                "package main\n",
		"{{ vars.old | snake }}/{{ vars.old }}_test.go": "package main\n",
	}, "old", "renamed")

	assert.FileExists(t, filepath.Join(root, "{{ vars.renamed | snake }}", "main.go"))
	assert.FileExists(t, filepath.Join(root, "{{ vars.renamed | snake }}", "{{ vars.renamed }}_test.go"))
	assert.NoDirExists(t, filepath.Join(root, "{{ vars.old | snake }}"))
	assert.Equal(t, 2, plan.PathRenameCount())
}

func TestUT_PlanRename_PruneKeepsDirsThatStillHoldContent(t *testing.T) {
	t.Parallel()

	// The placeholder directory also contains a .tagignore'd file, which the
	// rename never moves. Pruning must not delete a directory that still has
	// content — os.Remove (not RemoveAll) guarantees that.
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tag.template.json":         `{"vars": {"old": 1}}`,
		".tagignore":                "keepme.txt\n",
		"{{ vars.old }}/main.go":    "package main\n",
		"{{ vars.old }}/keepme.txt": "not a template\n",
	})

	plan, err := PlanRename(root, "old", "renamed")
	require.NoError(t, err)
	require.NoError(t, plan.Apply())

	// main.go moved to the renamed dir; the ignored file stays put, so the old
	// directory must survive with its remaining content intact.
	assert.FileExists(t, filepath.Join(root, "{{ vars.renamed }}", "main.go"))
	assert.FileExists(t, filepath.Join(root, "{{ vars.old }}", "keepme.txt"))
}

func TestUT_PlanRename_DryRunLeavesTreeUntouched(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := map[string]string{
		"tag.template.json":              `{"vars": {"old": 1}}`,
		"README.md":                      "# {{ vars.old }}\n",
		"{{ vars.old | snake }}/main.go": "package main\n",
	}
	writeTree(t, root, files)
	before := snapshotTree(t, root)

	plan, err := PlanRename(root, "old", "renamed")
	require.NoError(t, err)

	assert.Positive(t, plan.ReplacementCount())
	assert.Equal(t, before, snapshotTree(t, root),
		"planning must not write anything — --dry-run depends on it")
}

func TestUT_PlanRename_SkipsNonTemplateContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tag.template.json": `{"vars": {"old": 1}}`,
		".tagignore":        "vendor/\nCLAUDE.md\n",
		"vendor/lib.go":     "// {{ vars.old }}\n",
		"CLAUDE.md":         "# {{ vars.old }}\n",
		"_dialects/go.yaml": "name: {{ vars.old }}\n",
		".git/config":       "{{ vars.old }}\n",
		"keep.md":           "# {{ vars.old }}\n",
	})
	require.NoError(t, os.WriteFile(filepath.Join(root, "logo.png"),
		[]byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x01, 0x02}, 0o644))

	plan, err := PlanRename(root, "old", "renamed")
	require.NoError(t, err)
	require.NoError(t, plan.Apply())

	tree := snapshotTree(t, root)
	assert.Equal(t, "// {{ vars.old }}\n", tree["vendor/lib.go"], ".tagignore must be honoured")
	assert.Equal(t, "# {{ vars.old }}\n", tree["CLAUDE.md"], ".tagignore must be honoured")
	assert.Equal(t, "name: {{ vars.old }}\n", tree["_dialects/go.yaml"])
	assert.Equal(t, "{{ vars.old }}\n", tree[".git/config"])
	assert.Equal(t, "# {{ vars.renamed }}\n", tree["keep.md"])
}

func TestUT_PlanRename_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		files   map[string]string
		oldName string
		newName string
		wantErr string
	}{
		{
			name:    "old name not declared",
			files:   map[string]string{"tag.template.json": `{"vars": {"other": 1}}`},
			oldName: "missing",
			newName: "renamed",
			wantErr: `not declared`,
		},
		{
			name:    "new name already declared",
			files:   map[string]string{"tag.template.json": `{"vars": {"old": 1, "taken": 2}}`},
			oldName: "old",
			newName: "taken",
			wantErr: `already declared`,
		},
		{
			name:    "identical names",
			files:   map[string]string{"tag.template.json": `{"vars": {"old": 1}}`},
			oldName: "old",
			newName: "old",
			wantErr: "identical",
		},
		{
			name:    "invalid new identifier",
			files:   map[string]string{"tag.template.json": `{"vars": {"old": 1}}`},
			oldName: "old",
			newName: "not-valid",
			wantErr: "not a valid variable name",
		},
		{
			name:    "invalid old identifier",
			files:   map[string]string{"tag.template.json": `{"vars": {"old": 1}}`},
			oldName: "1bad",
			newName: "renamed",
			wantErr: "not a valid variable name",
		},
		{
			name:    "missing config",
			files:   map[string]string{"README.md": "hi"},
			oldName: "old",
			newName: "renamed",
			wantErr: "tag.template.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeTree(t, root, tt.files)

			_, err := PlanRename(root, tt.oldName, tt.newName)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestUT_PlanRename_RejectsPathCollision(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tag.template.json":     `{"vars": {"old": 1}}`,
		"{{ vars.old }}.go":     "a\n",
		"{{ vars.renamed }}.go": "b\n",
	})

	_, err := PlanRename(root, "old", "renamed")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestUT_PlanRename_RejectsTwoPathsRenamingOntoEachOther(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Both names rewrite to "{{ vars.renamed }}-{{ vars.renamed }}.go"; applying
	// them in sequence would silently overwrite the first with the second.
	writeTree(t, root, map[string]string{
		"tag.template.json":                    `{"vars": {"old": 1}}`,
		"{{ vars.old }}-{{ vars.renamed }}.go": "a\n",
		"{{ vars.renamed }}-{{ vars.old }}.go": "b\n",
	})

	_, err := PlanRename(root, "old", "renamed")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "same path")
}

func TestUT_PlanRename_AllowsCaseOnlyRename(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tag.template.json": `{"vars": {"Old": 1}}`,
		"{{ vars.Old }}.go": "package main\n",
	})

	// On a case-insensitive filesystem the destination Lstat resolves to the
	// source itself; that must not be mistaken for a collision.
	plan, err := PlanRename(root, "Old", "old")
	require.NoError(t, err)
	require.NoError(t, plan.Apply())

	assert.FileExists(t, filepath.Join(root, "{{ vars.old }}.go"))
}

func TestUT_PlanRename_RejectsNewNameAlreadyDeclaredInGenerator(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tag.template.json": `{"vars": {"old": 1}}`,
		// Renaming old -> taken here would produce a duplicate "taken" key,
		// and a map-based parse would silently drop one declaration.
		"_generators/api/tag.template.json": `{"vars": {"old": 1, "taken": 2}}`,
	})

	_, err := PlanRename(root, "old", "taken")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already declared")
}

func TestUT_RenamePlan_ApplyRestoresAfterPathMoveFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tag.template.json":           `{"vars": {"old": 1}}`,
		"README.md":                   "# {{ vars.old }}\n",
		"{{ vars.old | snake }}/a.go": "package a\n",
		"{{ vars.old | snake }}/b.go": "package b\n",
	})
	before := snapshotTree(t, root)

	plan, err := PlanRename(root, "old", "renamed")
	require.NoError(t, err)

	// Break the last move so Apply fails after content writes AND after at
	// least one rename has already succeeded.
	moved := 0
	for i := range plan.Files {
		if plan.Files[i].NewPath != "" {
			moved++
			if moved == 2 {
				plan.Files[i].Path = filepath.Join("gone", "missing.go")
			}
		}
	}
	require.Equal(t, 2, moved, "fixture must produce two path moves")

	require.Error(t, plan.Apply())
	assert.Equal(t, before, snapshotTree(t, root),
		"a move-phase failure must roll back content writes and completed renames")
	assert.NoDirExists(t, filepath.Join(root, "{{ vars.renamed | snake }}"),
		"rollback must not leave the directory it created behind")
}

func TestUT_PlanRename_RejectsNonDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	file := filepath.Join(root, "afile")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))

	_, err := PlanRename(file, "old", "renamed")
	require.Error(t, err)
}

func TestUT_RenamePlan_ApplyRestoresOnFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tag.template.json": `{"vars": {"old": 1}}`,
		"a.md":              "# {{ vars.old }}\n",
		"b.md":              "# {{ vars.old }}\n",
	})
	before := snapshotTree(t, root)

	plan, err := PlanRename(root, "old", "renamed")
	require.NoError(t, err)

	// Point one entry at an unwritable location so Apply fails partway.
	plan.Files[len(plan.Files)-1].Path = filepath.Join("nope", "missing.md")

	require.Error(t, plan.Apply())
	assert.Equal(t, before, snapshotTree(t, root),
		"a failed Apply must leave the template exactly as it was")
}

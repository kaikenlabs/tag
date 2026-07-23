package vars

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_WriteRenamePlan_DryRunPreview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tag.template.json":              `{"vars": {"old": 1}}`,
		"README.md":                      "# {{ vars.old }}\n",
		"{{ vars.old | snake }}/main.go": "package main\n",
	})

	plan, err := PlanRename(root, "old", "renamed")
	require.NoError(t, err)

	var sb strings.Builder
	WriteRenamePlan(&sb, plan, true)
	out := sb.String()

	assert.Contains(t, out, `Renaming "old" → "renamed"`)
	assert.Contains(t, out, "README.md:1")
	assert.Contains(t, out, "- # {{ vars.old }}")
	assert.Contains(t, out, "+ # {{ vars.renamed }}")
	assert.Contains(t, out, "(path placeholder)")
	// 1 README reference + 1 config key + 1 path move.
	assert.Contains(t, out, "3 files, 3 replacements total")
	assert.NotContains(t, out, "Renamed \"old\"", "dry-run must not claim the rename happened")
}

func TestUT_WriteRenamePlan_AppliedSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"tag.template.json": `{"vars": {"old": 1}}`,
		"README.md":         "# {{ vars.old }}\n",
	})

	plan, err := PlanRename(root, "old", "renamed")
	require.NoError(t, err)

	var sb strings.Builder
	WriteRenamePlan(&sb, plan, false)
	out := sb.String()

	assert.Contains(t, out, `Renamed "old" → "renamed"`)
	assert.Contains(t, out, "tag.template.json")
	assert.Contains(t, out, "tag template lint")
}

func TestUT_WriteRenamePlan_NoChanges(t *testing.T) {
	t.Parallel()

	plan := &RenamePlan{Root: "/tmp/x", OldName: "old", NewName: "renamed"}

	var sb strings.Builder
	WriteRenamePlan(&sb, plan, true)

	assert.Contains(t, sb.String(), "No references found")
}

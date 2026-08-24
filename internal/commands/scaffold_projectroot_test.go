package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/fileaction"
	"github.com/kaikenlabs/tag/internal/scaffold"
)

func TestUT_NewScaffoldDoc_EmitsProjectRoot(t *testing.T) {
	doc := newScaffoldDoc(scaffold.ScaffoldResult{
		OutputDir:   "/abs/out",
		ProjectRoot: "/abs/out/my-proj",
		Opts:        scaffold.Options{TemplateRef: "./tmpl"},
		Files:       []scaffold.FileEntry{{Path: "my-proj/README.md", Action: fileaction.ActionCreate}},
	}, testVersion)

	data, err := json.Marshal(doc)
	require.NoError(t, err)

	// Exact key set, not a subset: this is the only whole-document assertion on
	// scaffoldDoc, so it is what makes a new or dropped root key a deliberate,
	// reviewable change rather than a silent one.
	assert.JSONEq(t, `{
		"schema_version": 1,
		"tag_version": "9.9.9-test",
		"output_dir": "/abs/out",
		"project_root": "/abs/out/my-proj",
		"template": "./tmpl",
		"files": [{"path": "my-proj/README.md", "action": "create"}],
		"created": 1,
		"dry_run": false
	}`, string(data))
}

func TestUT_DisplayScaffoldSummary_NamesProjectRootNotOutputDir(t *testing.T) {
	var buf bytes.Buffer
	displayScaffoldSummary(&buf, scaffold.ScaffoldResult{
		OutputDir:   "/abs/out",
		ProjectRoot: "/abs/out/my-proj",
		TemplateDir: t.TempDir(),
	})

	output := buf.String()
	assert.Contains(t, output, "Output: /abs/out/my-proj\n")
	assert.Contains(t, output, "cd /abs/out/my-proj\n")
	assert.NotContains(t, output, "Output: /abs/out\n")
	assert.NotContains(t, output, "cd /abs/out\n")
}

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/lint"
	"github.com/kaikenlabs/tag/internal/vars"
)

// renameVarFixture is a template that exercises every location the ticket lists:
// declaration, derived default, hook command, bundle requires, body references,
// frontmatter, conditionals, loops and path placeholders.
var renameVarFixture = map[string]string{
	"tag.template.json": `{
  "name": "demo",
  "vars": {
    "project_name": { "type": "string", "prompt": "Project name" },
    "features": { "type": "string", "default": "" },
    "module": { "default": "github.com/acme/{{ vars.project_name | kebab }}" }
  },
  "hooks": {
    "post_scaffold": ["echo built {{ vars.project_name }}"]
  }
}`,
	"README.md": `# {{ vars.project_name }}

The project_name below is prose and must survive untouched.

{% if vars.project_name %}Configured.{% endif %}
{% for f in vars.features %}- {{ f }}
{% endfor %}
`,
	"go.mod": "module {{ vars.module }}\n",
	"{{ vars.project_name | snake }}/main.go": `---
to: {{ vars.project_name | snake }}/main.go
---
package main

// {{ vars.project_name }}
func main() {}
`,
	"_generators/api/tag.template.json": `{
  "vars": { "name": { "type": "string" } },
  "requires": ["project_name"]
}`,
	"_generators/api/templates/handler.go.t": "// {{ vars.name }} for {{ vars.project_name }}\n",
	".tagignore":                             "vendor/\n",
	"vendor/third_party.go":                  "// {{ vars.project_name }} must not be touched\n",
}

func writeFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
}

func readFile(t *testing.T, parts ...string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(parts...))
	require.NoError(t, err)
	return string(content)
}

func TestIT_TemplateRenameVar_LeavesTemplateLintClean(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFixture(t, root, renameVarFixture)

	// The fixture must start clean, otherwise the post-rename assertion is vacuous.
	before, err := vars.Analyze(root)
	require.NoError(t, err)
	require.False(t, before.HasIssues(), "fixture should start with no variable issues")

	plan, err := vars.PlanRename(root, "project_name", "service_name")
	require.NoError(t, err)
	require.NoError(t, plan.Apply())

	// An unchanged tree also lints clean, so prove the rename actually happened
	// before drawing any conclusion from the linters below.
	require.Contains(t, readFile(t, root, "README.md"), "{{ vars.service_name }}")
	require.NotContains(t, readFile(t, root, "tag.template.json"), "project_name")

	// Every reference moved: nothing undeclared, nothing unused.
	after, err := vars.Analyze(root)
	require.NoError(t, err)
	assert.False(t, after.HasIssues(),
		"a complete rename must leave no undeclared or unused variables")

	linter, err := lint.NewLinter(root)
	require.NoError(t, err)
	result, err := linter.Run()
	require.NoError(t, err)
	assert.Equal(t, 0, result.ErrorCount(), "issues: %+v", result.Issues)
}

func TestIT_TemplateRenameVar_RewritesEveryLocation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFixture(t, root, renameVarFixture)

	plan, err := vars.PlanRename(root, "project_name", "service_name")
	require.NoError(t, err)
	require.NoError(t, plan.Apply())

	config := readFile(t, root, "tag.template.json")
	assert.Contains(t, config, `"service_name": { "type": "string", "prompt": "Project name" }`)
	assert.Contains(t, config, `github.com/acme/{{ vars.service_name | kebab }}`)
	assert.Contains(t, config, `echo built {{ vars.service_name }}`)
	assert.NotContains(t, config, "vars.project_name")

	readme := readFile(t, root, "README.md")
	assert.Contains(t, readme, "# {{ vars.service_name }}")
	assert.Contains(t, readme, "{% if vars.service_name %}")
	assert.Contains(t, readme, "The project_name below is prose and must survive untouched.",
		"prose must not be rewritten")

	generator := readFile(t, root, "_generators", "api", "tag.template.json")
	assert.Contains(t, generator, `"requires": ["service_name"]`)
	assert.Contains(t, readFile(t, root, "_generators", "api", "templates", "handler.go.t"),
		"// {{ vars.name }} for {{ vars.service_name }}")

	assert.Equal(t, "// {{ vars.project_name }} must not be touched\n",
		readFile(t, root, "vendor", "third_party.go"), ".tagignore must be honoured")

	renamedDir := filepath.Join(root, "{{ vars.service_name | snake }}")
	assert.DirExists(t, renamedDir)
	assert.NoDirExists(t, filepath.Join(root, "{{ vars.project_name | snake }}"))
	main := readFile(t, renamedDir, "main.go")
	assert.Contains(t, main, "to: {{ vars.service_name | snake }}/main.go")
	assert.Contains(t, main, "// {{ vars.service_name }}")
}

// Raw blocks and comments live in their own fixture because `tag template lint`
// does not model either construct: its variable scan treats their contents as
// real references, so a template that keeps a literal {{ vars.old }} inside
// {% raw %} lints as an undefined variable after a rename. Leaving the literal
// alone is still correct — rewriting it would change the generated output.
func TestIT_TemplateRenameVar_LeavesRawBlocksAndCommentsLiteral(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		"tag.template.json": `{"vars": {"project_name": {"type": "string"}}}`,
		"doc.md": `# {{ vars.project_name }}
{% raw %}Literal {{ vars.project_name }} stays literal.{% endraw %}
{# {{ vars.project_name }} in a comment #}
`,
	})

	plan, err := vars.PlanRename(root, "project_name", "service_name")
	require.NoError(t, err)
	require.NoError(t, plan.Apply())

	doc := readFile(t, root, "doc.md")
	assert.Contains(t, doc, "# {{ vars.service_name }}")
	assert.Contains(t, doc, "{% raw %}Literal {{ vars.project_name }} stays literal.{% endraw %}")
	assert.Contains(t, doc, "{# {{ vars.project_name }} in a comment #}")
}

func TestIT_TemplateRenameVar_DryRunIsAPreview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFixture(t, root, renameVarFixture)

	plan, err := vars.PlanRename(root, "project_name", "service_name")
	require.NoError(t, err)

	assert.Positive(t, plan.FileCount())
	assert.Positive(t, plan.ReplacementCount())

	// Planning alone must be inert: the tree is byte-identical to the fixture.
	for rel, want := range renameVarFixture {
		assert.Equal(t, want, readFile(t, root, filepath.FromSlash(rel)),
			"planning must not modify %s", rel)
	}
}

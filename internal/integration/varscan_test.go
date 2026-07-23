package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/lint"
	"github.com/kaikenlabs/tag/internal/vars"
)

// varScanFixtureBody exercises all four variable-scan defects from issue #337
// in a single template body:
//
//  1. a vars.NAME mention inside a string literal argument ("ghost"),
//  2. two references in one block that a greedy regex collapses to one
//     ("alpha" and "beta"),
//  3. an attribute path that merely ends in vars.NAME, not a reference to the
//     global vars namespace ("attrname"),
//  4. a block spanning multiple lines ("spans" and "lines").
const varScanFixtureBody = `Defect 1 (string literal): {{ replace("{{ vars.ghost }}") }}
Defect 2 (greedy regex): {{ vars.alpha ~ vars.beta }}
Defect 3 (attribute path): {{ cfg.vars.attrname }}
Defect 4 (multi-line block): {{ vars.spans
    ~ vars.lines }}
Subscript (issue #339): {{ vars["subbed"] }} and {{ vars['single'] }}
Subscript lookalike: {{ myvars["notref"] }}
`

// varScanFixtureCandidates is every name mentioned in varScanFixtureBody,
// real reference or lookalike.
var varScanFixtureCandidates = []string{
	"ghost", "alpha", "beta", "attrname", "spans", "lines",
	"subbed", "single", "notref",
}

// varScanFixtureRealRefs is the subset of varScanFixtureCandidates that are
// true vars.* references. "ghost", "attrname" and "notref" are deliberately
// excluded: they are lookalikes, not references. "subbed" and "single" are
// subscript references, real since issue #339.
var varScanFixtureRealRefs = []string{"alpha", "beta", "lines", "single", "spans", "subbed"}

// TestIT_TemplateVarScan_ThreeCommandsAgree proves lint, the variables report
// and rename-var agree on exactly the real references in one fixture built
// from all four defect shapes. Before the fix, the three disagreed: lint
// flagged "ghost" and "attrname" (defect 1 and 3) and missed "spans"/"lines"
// (defect 4 — its multi-line block was invisible to line-by-line scanning),
// while rename-var got all four right.
func TestIT_TemplateVarScan_ThreeCommandsAgree(t *testing.T) {
	t.Parallel()

	// Config A declares none of the candidate names, so every real reference
	// in the body is undeclared: lint must flag it and vars.Analyze must
	// report it undeclared. Neither may flag "ghost" or "attrname" — they are
	// not references, so they must never appear here, declared or not.
	rootA := t.TempDir()
	writeFixture(t, rootA, map[string]string{
		"tag.template.json": `{"name": "varscan-fixture-a", "vars": {}}`,
		"body.txt":          varScanFixtureBody,
	})

	linter, err := lint.NewLinter(rootA)
	require.NoError(t, err)
	lintResult, err := linter.Run()
	require.NoError(t, err)

	lintFlagged := map[string]bool{}
	for _, name := range varScanFixtureCandidates {
		quoted := fmt.Sprintf("%q", name)
		for _, issue := range lintResult.Issues {
			if issue.Rule == "undefined-variable" && strings.Contains(issue.Message, quoted) {
				lintFlagged[name] = true
			}
		}
	}

	report, err := vars.Analyze(rootA)
	require.NoError(t, err)
	varsReferenced := map[string]bool{}
	for _, uv := range report.Root.Undeclared {
		varsReferenced[uv.Name] = true
	}

	// Config B declares every candidate name, real reference or lookalike, so
	// PlanRename accepts each of them as an old name. Whether the rename
	// actually touches body.txt reveals whether rename-var considers the name
	// a real reference — mirroring lint and vars.Analyze above.
	varsDecl := make(map[string]any, len(varScanFixtureCandidates))
	for _, name := range varScanFixtureCandidates {
		varsDecl[name] = map[string]string{"type": "string", "default": "x"}
	}
	configB, err := json.Marshal(map[string]any{
		"name": "varscan-fixture-b",
		"vars": varsDecl,
	})
	require.NoError(t, err)

	rootB := t.TempDir()
	writeFixture(t, rootB, map[string]string{
		"tag.template.json": string(configB),
		"body.txt":          varScanFixtureBody,
	})

	renameTouchesBody := map[string]bool{}
	for _, name := range varScanFixtureCandidates {
		plan, planErr := vars.PlanRename(rootB, name, name+"_renamed")
		require.NoError(t, planErr, "PlanRename should accept declared name %q", name)
		// Every candidate is declared in tag.template.json, so the plan always
		// touches that file (renaming the declaration key) regardless of
		// whether the name is a real reference in body.txt. Only a change to
		// body.txt itself reveals whether rename-var considers the name a
		// real reference there.
		for _, fc := range plan.Files {
			if fc.Path == "body.txt" && fc.Replacements > 0 {
				renameTouchesBody[name] = true
			}
		}
	}

	want := map[string]bool{}
	for _, name := range varScanFixtureRealRefs {
		want[name] = true
	}

	require.NotEmpty(t, lintFlagged, "a mutually-empty result must not pass")
	require.NotEmpty(t, varsReferenced, "a mutually-empty result must not pass")
	require.NotEmpty(t, renameTouchesBody, "a mutually-empty result must not pass")

	assert.Equal(t, want, lintFlagged, "lint")
	assert.Equal(t, want, varsReferenced, "vars.Analyze")
	assert.Equal(t, want, renameTouchesBody, "rename-var")
}

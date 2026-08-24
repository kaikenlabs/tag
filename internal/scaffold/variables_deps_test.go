package scaffold

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/template"
)

// The three tests below pin the deliberate behaviour change from #393: variable
// dependencies are now found by internal/vars' block walker instead of a regex
// that required "{{" to sit immediately before "vars.". Every pre-existing
// topologicalSortVars test uses a plain "{{ vars.x }}" default, which the old
// regex already handled, so all of them pass identically before and after the
// swap and none of them can see this.

// TestUT_TopologicalSortVars_NonLeadingRefIsADependency covers the edge the
// old regex missed outright: a reference that is not the first token after "{{"
// was not a dependency at all, so the dependent could be prompted before its dependency.
func TestUT_TopologicalSortVars_NonLeadingRefIsADependency(t *testing.T) {
	t.Parallel()

	sorted, err := topologicalSortVars(map[string]VariableDef{
		"alpha": {Type: VarTypeString, Default: `{{ "svc-" ~ vars.zeta }}`},
		"zeta":  {Type: VarTypeString, Default: "z"},
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"zeta", "alpha"}, sorted,
		"zeta must sort before alpha: alpha's default references it after a string literal")
}

// TestUT_TopologicalSortVars_SelfReferenceAfterLiteralIsACycle is the other
// half of the same change, and the reason vars.DeclaredDeps retains
// self-references. Before #393 this template sorted cleanly and TAG went on to
// render a default referencing a variable that did not have a value yet.
func TestUT_TopologicalSortVars_SelfReferenceAfterLiteralIsACycle(t *testing.T) {
	t.Parallel()

	_, err := topologicalSortVars(map[string]VariableDef{
		"alpha": {Type: VarTypeString, Default: `{{ "svc-" ~ vars.alpha }}`},
	})

	require.ErrorIs(t, err, ErrCircularDependency)
	assert.Contains(t, err.Error(), "alpha")
}

// TestUT_TopologicalSortVars_StatementOnlyDefaultIsNotADependency guards the
// restriction that keeps the swap from widening beyond its intent.
// ContainsTemplateExpression looks for "vars." after a "{{", so a default made
// only of a statement block is a literal string to TAG and is never rendered.
// The block walker WOULD find a reference in it; treating that as a dependency
// would invent an ordering constraint for an expression TAG never evaluates.
func TestUT_TopologicalSortVars_StatementOnlyDefaultIsNotADependency(t *testing.T) {
	t.Parallel()

	sorted, err := topologicalSortVars(map[string]VariableDef{
		"alpha": {Type: VarTypeString, Default: "{% if vars.zeta %}x{% endif %}"},
		"zeta":  {Type: VarTypeString, Default: "z"},
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "zeta"}, sorted,
		"no dependency edge, so the lexicographic tie-break decides the order")
}

// TestUT_VariableCollector_NonLeadingDefaultOrdersDependencyFirst is the
// user-visible half of the sort change: the prompt order a person actually sees.
//
// The fixture must be alpha-depends-on-zeta, never the reverse. With the
// dependency reversed the lexicographic tie-break already produces the correct
// order, so the test would pass against the unfixed code and prove nothing.
func TestUT_VariableCollector_NonLeadingDefaultOrdersDependencyFirst(t *testing.T) {
	engine, err := template.NewEngine()
	require.NoError(t, err)

	promptOrder := []string{}
	collector := NewVariableCollector(&orderTrackingPrompter{
		inner:       NewMockPrompter(),
		promptOrder: &promptOrder,
	}, io.Discard)
	collector.WithEngine(engine)

	config := &TemplateConfig{
		Vars: map[string]VariableDef{
			"alpha": {
				Type:    VarTypeString,
				Prompt:  "Alpha",
				Default: `{{ "svc-" ~ vars.zeta }}`,
			},
			"zeta": {Type: VarTypeString, Prompt: "Zeta", Default: "my-project"},
		},
	}

	vars, err := collector.Collect(config, Options{}, true)
	require.NoError(t, err)

	require.Len(t, promptOrder, 2)
	assert.Equal(t, "Zeta", promptOrder[0])
	assert.Equal(t, "Alpha", promptOrder[1])
	assert.Equal(t, "svc-my-project", vars["alpha"])
}

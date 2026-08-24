package vars

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

// TestUT_DeclaredDeps_Table is the spec for what counts as a variable
// dependency. It is deliberately the only place that spec lives: topologicalSortVars
// and `template info`'s depends_on both read it, so a divergence here is a
// divergence everywhere.
func TestUT_DeclaredDeps_Table(t *testing.T) {
	t.Parallel()

	declared := func(names ...string) map[string]tmplconfig.VariableDef {
		defs := make(map[string]tmplconfig.VariableDef, len(names))
		for _, n := range names {
			defs[n] = tmplconfig.VariableDef{Type: tmplconfig.VarTypeString}
		}
		return defs
	}

	tests := []struct {
		name    string
		subject string
		def     tmplconfig.VariableDef
		others  []string
		want    []string
	}{
		{
			name:    "plain default has no dependencies",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: "hello"},
			others:  []string{"b"},
			want:    []string{},
		},
		{
			name:    "absent default has no dependencies",
			subject: "a",
			def:     tmplconfig.VariableDef{},
			others:  []string{"b"},
			want:    []string{},
		},
		{
			name:    "non-string default has no dependencies",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: 8080},
			others:  []string{"b"},
			want:    []string{},
		},
		{
			name:    "simple reference",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: "{{ vars.b }}"},
			others:  []string{"b"},
			want:    []string{"b"},
		},
		{
			// The old varRefPattern required `{{` immediately before `vars.`,
			// so a function wrapper made this dependency invisible.
			name:    "function-wrapped reference is a dependency",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: "{{ upper(vars.b) }}"},
			others:  []string{"b"},
			want:    []string{"b"},
		},
		{
			name:    "filtered reference is a dependency",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: "{{ vars.b | lower }}"},
			others:  []string{"b"},
			want:    []string{"b"},
		},
		{
			// The old regex was greedy and captured only the first name.
			name:    "every reference in a block, not just the first",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: `{{ vars.b ~ "_" ~ vars.c }}`},
			others:  []string{"b", "c"},
			want:    []string{"b", "c"},
		},
		{
			name:    "multi-line block is not invisible",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: "{{ vars.b\n ~ vars.c }}"},
			others:  []string{"b", "c"},
			want:    []string{"b", "c"},
		},
		{
			// ScanNames does resolve subscript refs, but ContainsTemplateExpression
			// looks for "vars." and this default has "vars[" — so TAG does not
			// treat it as an expression and never renders it. depends_on stays
			// aligned with default_is_expression rather than with the scanner.
			name:    "subscript default is not an expression, so no dependencies",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: `{{ vars["b"] }}`},
			others:  []string{"b"},
			want:    []string{},
		},
		{
			name:    "results are sorted and deduplicated",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: "{{ vars.c }}{{ vars.b }}{{ vars.c }}"},
			others:  []string{"b", "c"},
			want:    []string{"b", "c"},
		},
		{
			name:    "undeclared names are dropped",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: "{{ vars.b }}{{ vars.nope }}"},
			others:  []string{"b"},
			want:    []string{"b"},
		},
		{
			// Retained on purpose: topologicalSortVars turns this into
			// ErrCircularDependency. Dropping it here silently disables cycle
			// detection and no other test in the tree would notice.
			name:    "self-reference is retained",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: "{{ upper(vars.a) }}"},
			others:  nil,
			want:    []string{"a"},
		},
		{
			name:    "name inside a string literal is not a dependency",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: `{{ "vars.b" ~ vars.c }}`},
			others:  []string{"b", "c"},
			want:    []string{"c"},
		},
		{
			name:    "name inside a comment is not a dependency",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: "{{ vars.b }}{# {{ vars.c }} #}"},
			others:  []string{"b", "c"},
			want:    []string{"b"},
		},
		{
			// ContainsTemplateExpression requires BOTH "{{" and "vars." — a
			// statement-only default is not an expression to TAG, so its value
			// is used literally and never rendered. Scanning it would invent a
			// dependency for something TAG never evaluates.
			name:    "statement-only default is not an expression, so no dependencies",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: "{% if vars.b %}x{% endif %}"},
			others:  []string{"b"},
			want:    []string{},
		},
		{
			name:    "expression referencing no variable has no dependencies",
			subject: "a",
			def:     tmplconfig.VariableDef{Default: "{{ now() }}"},
			others:  []string{"b"},
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			defs := declared(tt.others...)
			defs[tt.subject] = tt.def

			got := DeclaredDeps(defs)

			require.Contains(t, got, tt.subject, "every declared variable must have an entry")
			require.NotNil(t, got[tt.subject],
				"entry must be an empty slice, never nil — it serialises straight to JSON as [] not null")
			assert.Equal(t, tt.want, got[tt.subject])
		})
	}
}

// TestUT_DeclaredDeps_EveryDeclaredVariableGetsAnEntry pins that the map is
// total over its input. `template info` indexes it per variable and would
// otherwise emit null for any variable the function chose to skip.
func TestUT_DeclaredDeps_EveryDeclaredVariableGetsAnEntry(t *testing.T) {
	t.Parallel()

	defs := map[string]tmplconfig.VariableDef{
		"a": {Default: "{{ vars.b }}"},
		"b": {Default: "plain"},
		"c": {},
	}

	got := DeclaredDeps(defs)

	require.Len(t, got, 3)
	for name, deps := range got {
		require.NotNil(t, deps, "variable %q has a nil dependency slice", name)
	}
	assert.Equal(t, []string{"b"}, got["a"])
	assert.Equal(t, []string{}, got["b"])
	assert.Equal(t, []string{}, got["c"])
}

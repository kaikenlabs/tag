package vars

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

// TestUT_ScanRefs_ShadowedVarsNamespace pins that a reference is attributed to
// the TEMPLATE variable only when `vars` actually names the root namespace at
// that point. `vars` is an ordinary identifier to Gonja, so a template can
// rebind it — and before scope tracking a lexical scan claimed every one of the
// shadowed cases below, which made `rename-var` corrupt such a template and made
// a dependency scan invent a fatal self-cycle.
func TestUT_ScanRefs_ShadowedVarsNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "for loop rebinding vars shadows its body",
			src:  `{% for vars in items %}{{ "x" ~ vars.alpha }}{% endfor %}`,
			want: nil,
		},
		{
			name: "for loop over a different name does not shadow",
			src:  `{% for item in items %}{{ vars.alpha }}{% endfor %}`,
			want: []string{"alpha"},
		},
		{
			name: "the shadowing block's own right-hand side is the outer scope",
			src:  `{% for vars in vars.items %}{{ vars.alpha }}{% endfor %}`,
			want: []string{"items"},
		},
		{
			name: "shadow ends at endfor",
			src:  `{% for vars in items %}{{ vars.alpha }}{% endfor %}{{ vars.beta }}`,
			want: []string{"beta"},
		},
		{
			name: "tuple target rebinding vars shadows",
			src:  `{% for k, vars in items %}{{ vars.alpha }}{% endfor %}`,
			want: nil,
		},
		{
			name: "nested non-shadowing loop inside a shadowing one stays shadowed",
			src:  `{% for vars in a %}{% for x in b %}{{ vars.alpha }}{% endfor %}{% endfor %}{{ vars.beta }}`,
			want: []string{"beta"},
		},
		{
			name: "nested shadowing loop pops correctly",
			src:  `{% for x in a %}{% for vars in b %}{{ vars.alpha }}{% endfor %}{{ vars.beta }}{% endfor %}`,
			want: []string{"beta"},
		},
		{
			name: "with rebinding vars shadows",
			src:  `{% with vars = other %}{{ vars.alpha }}{% endwith %}{{ vars.beta }}`,
			want: []string{"beta"},
		},
		{
			name: "with binding another name does not shadow",
			src:  `{% with tmp = 1 %}{{ vars.alpha }}{% endwith %}`,
			want: []string{"alpha"},
		},
		{
			name: "macro parameter named vars shadows its body",
			src:  `{% macro render(vars) %}{{ vars.alpha }}{% endmacro %}{{ vars.beta }}`,
			want: []string{"beta"},
		},
		{
			name: "set vars shadows the remainder",
			src:  `{{ vars.alpha }}{% set vars = other %}{{ vars.beta }}`,
			want: []string{"alpha"},
		},
		{
			name: "set of another name does not shadow",
			src:  `{% set tmp = 1 %}{{ vars.alpha }}`,
			want: []string{"alpha"},
		},
		{
			name: "if blocks never shadow and endif never pops",
			src:  `{% if vars.flag %}{{ vars.alpha }}{% endif %}{{ vars.beta }}`,
			want: []string{"flag", "alpha", "beta"},
		},
		{
			name: "subscript access is shadowed too",
			src:  `{% for vars in items %}{{ vars["alpha"] }}{% endfor %}`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ScanNames(tt.src))
		})
	}
}

// TestUT_RenameInExpressions_LeavesShadowedReferencesAlone is the other half:
// the rename walker must reach the same verdict as the scanner, or the two
// diverge and the guarantee that lint/variables/rename-var agree is lost.
// Before scope tracking this rewrote the loop-local reference and silently
// changed what the template rendered.
func TestUT_RenameInExpressions_LeavesShadowedReferencesAlone(t *testing.T) {
	t.Parallel()

	src := `{% for vars in items %}{{ vars.alpha }}{% endfor %}{{ vars.alpha }}`

	got, count := renameInExpressions(src, "alpha", "renamed")

	assert.Equal(t, 1, count, "only the unshadowed reference may be rewritten")
	assert.Equal(t, `{% for vars in items %}{{ vars.alpha }}{% endfor %}{{ vars.renamed }}`, got)
}

// TestUT_DeclaredDeps_ShadowedReferenceIsNotADependency is the defect that
// prompted scope tracking, at the layer a user actually feels it: this template
// scaffolded correctly before depends_on existed, and a lexical scan turned it
// into a fatal "variable references itself" error.
func TestUT_DeclaredDeps_ShadowedReferenceIsNotADependency(t *testing.T) {
	t.Parallel()

	deps := DeclaredDeps(map[string]tmplconfig.VariableDef{
		"alpha": {Default: `{% for vars in [{"alpha":"ok"}] %}{{ "x" ~ vars.alpha }}{% endfor %}`},
	})

	require.Contains(t, deps, "alpha")
	assert.Equal(t, []string{}, deps["alpha"],
		"the reference is to the loop variable, not to the declared variable alpha")
}

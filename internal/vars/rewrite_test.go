package vars

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_RenameInExpressions_ExpressionForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		src   string
		want  string
		count int
	}{
		{
			name:  "bare reference",
			src:   "# {{ vars.old }}",
			want:  "# {{ vars.new }}",
			count: 1,
		},
		{
			name:  "single filter",
			src:   "module {{ vars.old | kebab }}",
			want:  "module {{ vars.new | kebab }}",
			count: 1,
		},
		{
			name:  "chained filters",
			src:   "{{ vars.old | snake | upper }}",
			want:  "{{ vars.new | snake | upper }}",
			count: 1,
		},
		{
			name:  "no surrounding whitespace",
			src:   "{{vars.old}}",
			want:  "{{vars.new}}",
			count: 1,
		},
		{
			name:  "whitespace control markers",
			src:   "{{- vars.old -}}",
			want:  "{{- vars.new -}}",
			count: 1,
		},
		{
			name:  "attribute access on the variable",
			src:   "{{ vars.old.nested }}",
			want:  "{{ vars.new.nested }}",
			count: 1,
		},
		{
			name:  "multiple references on one line",
			src:   "{{ vars.old }}-{{ vars.old | upper }}",
			want:  "{{ vars.new }}-{{ vars.new | upper }}",
			count: 2,
		},
		{
			name:  "string concatenation operand",
			src:   `{{ vars.old ~ "-suffix" }}`,
			want:  `{{ vars.new ~ "-suffix" }}`,
			count: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, n := renameInExpressions(tt.src, "old", "new")
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.count, n)
		})
	}
}

func TestUT_RenameInExpressions_StatementForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		src   string
		want  string
		count int
	}{
		{
			name:  "if conditional",
			src:   "{% if vars.old %}x{% endif %}",
			want:  "{% if vars.new %}x{% endif %}",
			count: 1,
		},
		{
			// Guards the whitespace-before-dot check against over-rejecting:
			// a keyword preceding the reference is not an attribute path.
			name:  "keyword directly before the reference",
			src:   "{% if not vars.old %}x{% endif %}",
			want:  "{% if not vars.new %}x{% endif %}",
			count: 1,
		},
		{
			name:  "for loop",
			src:   "{% for item in vars.old %}{{ item }}{% endfor %}",
			want:  "{% for item in vars.new %}{{ item }}{% endfor %}",
			count: 1,
		},
		{
			name:  "set statement",
			src:   "{% set x = vars.old %}",
			want:  "{% set x = vars.new %}",
			count: 1,
		},
		{
			name:  "whitespace control on statement",
			src:   "{%- if vars.old -%}",
			want:  "{%- if vars.new -%}",
			count: 1,
		},
		{
			name: "multi-line statement tag",
			src: `{% if vars.old
      and vars.old != "" %}ok{% endif %}`,
			want: `{% if vars.new
      and vars.new != "" %}ok{% endif %}`,
			count: 2,
		},
		{
			name:  "elif branch",
			src:   "{% if a %}{% elif vars.old %}{% endif %}",
			want:  "{% if a %}{% elif vars.new %}{% endif %}",
			count: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, n := renameInExpressions(tt.src, "old", "new")
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.count, n)
		})
	}
}

func TestUT_RenameInExpressions_LeavesNonExpressionsAlone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
	}{
		{
			name: "plain prose containing the name",
			src:  "The old variable and vars.old outside a tag stay put.",
		},
		{
			name: "longer name with the old name as a prefix",
			src:  "{{ vars.old_suffix }}",
		},
		{
			name: "different variable ending with the old name",
			src:  "{{ vars.very_old }}",
		},
		{
			name: "another namespace",
			src:  "{{ myvars.old }}",
		},
		{
			name: "nested attribute that merely ends in vars",
			src:  "{{ cfg.vars.old }}",
		},
		{
			// Gonja accepts whitespace around the dot operator, so this is an
			// attribute of cfg — not the global vars namespace.
			name: "nested attribute with whitespace around the dot",
			src:  "{{ cfg . vars.old }}",
		},
		{
			name: "nested attribute with a tab around the dot",
			src:  "{{ cfg\t.\tvars.old }}",
		},
		{
			name: "gonja comment",
			src:  "{# {{ vars.old }} is deprecated #}",
		},
		{
			name: "multi-line gonja comment",
			src:  "{#\n  {{ vars.old }}\n#}",
		},
		{
			name: "raw block",
			src:  "{% raw %}{{ vars.old }}{% endraw %}",
		},
		{
			name: "raw block with whitespace control",
			src:  "{%- raw -%}\n{{ vars.old | kebab }}\n{%- endraw -%}",
		},
		{
			name: "string literal inside an expression",
			src:  `{{ "vars.old" }}`,
		},
		{
			name: "string literal comparison inside a statement",
			src:  `{% if x == "vars.old" %}y{% endif %}`,
		},
		{
			name: "single-quoted string literal",
			src:  `{{ f('vars.old') }}`,
		},
		{
			name: "unterminated expression is left verbatim",
			src:  "{{ vars.old",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, n := renameInExpressions(tt.src, "old", "new")
			assert.Equal(t, tt.src, got)
			assert.Equal(t, 0, n)
		})
	}
}

func TestUT_RenameInExpressions_DelimitersInsideStringLiterals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		src   string
		want  string
		count int
	}{
		{
			name:  "closing expression delimiter inside a string",
			src:   `{{ f("}}") ~ vars.old }}`,
			want:  `{{ f("}}") ~ vars.new }}`,
			count: 1,
		},
		{
			name:  "closing statement delimiter inside a string",
			src:   `{% set s = "%}" %}{{ vars.old }}`,
			want:  `{% set s = "%}" %}{{ vars.new }}`,
			count: 1,
		},
		{
			name:  "escaped quote inside a string",
			src:   `{{ f("a\"}}b") ~ vars.old }}`,
			want:  `{{ f("a\"}}b") ~ vars.new }}`,
			count: 1,
		},
		{
			name:  "expression after a raw block still renamed",
			src:   "{% raw %}{{ vars.old }}{% endraw %}{{ vars.old }}",
			want:  "{% raw %}{{ vars.old }}{% endraw %}{{ vars.new }}",
			count: 1,
		},
		{
			name:  "expression after a comment still renamed",
			src:   "{# vars.old #}{{ vars.old }}",
			want:  "{# vars.old #}{{ vars.new }}",
			count: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, n := renameInExpressions(tt.src, "old", "new")
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.count, n)
		})
	}
}

func TestUT_RenameInExpressions_RawBodyIsScannedLiterally(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		src   string
		want  string
		count int
	}{
		{
			name: "unbalanced quote inside a raw body does not swallow the rest of the file",
			src: `{% raw %}{% set x = "oops %}{% endraw %}` +
				`{{ vars.old }}`,
			want: `{% raw %}{% set x = "oops %}{% endraw %}` +
				`{{ vars.new }}`,
			count: 1,
		},
		{
			name:  "raw tag separated by tabs is still recognised",
			src:   "{%\traw\t%}{{ vars.old }}{%\tendraw\t%}{{ vars.old }}",
			want:  "{%\traw\t%}{{ vars.old }}{%\tendraw\t%}{{ vars.new }}",
			count: 1,
		},
		{
			name:  "raw tag with no surrounding whitespace",
			src:   "{%raw%}{{ vars.old }}{%endraw%}{{ vars.old }}",
			want:  "{%raw%}{{ vars.old }}{%endraw%}{{ vars.new }}",
			count: 1,
		},
		{
			name:  "unterminated raw block leaves the remainder verbatim",
			src:   "{% raw %}{{ vars.old }} forever",
			want:  "{% raw %}{{ vars.old }} forever",
			count: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, n := renameInExpressions(tt.src, "old", "new")
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.count, n)
		})
	}
}

func TestUT_RenameInExpressions_PreservesLineCount(t *testing.T) {
	t.Parallel()

	src := "a\n{{ vars.old }}\nb\n{% if vars.old %}\nc\n{% endif %}\n"
	got, n := renameInExpressions(src, "old", "renamed")

	assert.Equal(t, 2, n)
	assert.Equal(t, countNewlines(src), countNewlines(got),
		"rewrite must not add or remove newlines so line numbers stay stable")
}

func countNewlines(s string) int {
	n := 0
	for i := range len(s) {
		if s[i] == '\n' {
			n++
		}
	}
	return n
}

// TestUT_RenameInExpressions_SubscriptAccess covers issue #339: rename-var
// rewrites the key of a vars["old"] / vars['old'] subscript reference,
// preserving quote style and whitespace, and leaves lookalikes alone.
func TestUT_RenameInExpressions_SubscriptAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		src   string
		want  string
		count int
	}{
		{
			name:  "double-quoted subscript",
			src:   `{{ vars["old"] }}`,
			want:  `{{ vars["new"] }}`,
			count: 1,
		},
		{
			name:  "single-quoted subscript",
			src:   `{{ vars['old'] }}`,
			want:  `{{ vars['new'] }}`,
			count: 1,
		},
		{
			name:  "whitespace around brackets is preserved",
			src:   `{{ vars [ "old" ] }}`,
			want:  `{{ vars [ "new" ] }}`,
			count: 1,
		},
		{
			name:  "subscript and dot access in one block",
			src:   `{{ vars["old"] ~ vars.old }}`,
			want:  `{{ vars["new"] ~ vars.new }}`,
			count: 2,
		},
		{
			name:  "subscript in a statement block",
			src:   `{% if vars["old"] %}x{% endif %}`,
			want:  `{% if vars["new"] %}x{% endif %}`,
			count: 1,
		},
		{
			name:  "attribute path is left alone",
			src:   `{{ cfg.vars["old"] }}`,
			want:  `{{ cfg.vars["old"] }}`,
			count: 0,
		},
		{
			name:  "longer identifier is left alone",
			src:   `{{ myvars["old"] }}`,
			want:  `{{ myvars["old"] }}`,
			count: 0,
		},
		{
			name:  "a different subscript key is left alone",
			src:   `{{ vars["other"] }}`,
			want:  `{{ vars["other"] }}`,
			count: 0,
		},
		{
			name:  "non-literal subscript is left alone",
			src:   `{{ vars[old] }}`,
			want:  `{{ vars[old] }}`,
			count: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, n := renameInExpressions(tt.src, "old", "new")
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.count, n)
		})
	}
}

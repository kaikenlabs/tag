package vars

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_MaskLiterals_BlanksLiteralSpans(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "gonja comment",
			src:  "a{# {{ vars.x }} #}b",
			want: "ab",
		},
		{
			name: "multi-line gonja comment",
			src:  "a{# {{ vars.x }}\nstill a comment #}b",
			want: "a\nb",
		},
		{
			name: "raw block",
			src:  "a{% raw %}{{ vars.x }}{% endraw %}b",
			want: "ab",
		},
		{
			name: "raw tag with no surrounding whitespace",
			src:  "a{%raw%}{{ vars.x }}{%endraw%}b",
			want: "ab",
		},
		{
			name: "raw tag separated by tabs",
			src:  "a{%\traw\t%}{{ vars.x }}{%\tendraw\t%}b",
			want: "ab",
		},
		{
			name: "raw tag separated by newlines",
			src:  "a{%\nraw\n%}{{ vars.x }}{%\nendraw\n%}b",
			want: "a\n\n\n\nb",
		},
		{
			name: "raw block with whitespace control",
			src:  "a{%- raw -%}{{ vars.x }}{%- endraw -%}b",
			want: "ab",
		},
		{
			name: "multi-line raw block keeps its newlines",
			src:  "a{% raw %}{{ vars.x }}\n{{ vars.y }}\n{% endraw %}b",
			want: "a\n\nb",
		},
		{
			name: "comment nested inside a raw body is part of the raw span",
			src:  "a{% raw %}{# {{ vars.x }} #}{% endraw %}b",
			want: "ab",
		},
		{
			name: "empty raw block",
			src:  "a{% raw %}{% endraw %}b",
			want: "ab",
		},
		{
			name: "adjacent raw blocks",
			src:  "a{% raw %}{{ vars.x }}{% endraw %}{% raw %}{{ vars.y }}{% endraw %}b",
			want: "ab",
		},
		{
			name: "raw block spanning the whole input",
			src:  "{% raw %}{{ vars.x }}{% endraw %}",
			want: "",
		},
		{
			name: "CRLF inside a masked span keeps only the newline",
			src:  "a{# {{ vars.x }}\r\nstill a comment #}b",
			want: "a\nb",
		},
		{
			name: "unterminated raw block masks to end of input",
			src:  "a{% raw %}{{ vars.x }}\ntrailing",
			want: "a\n",
		},
		{
			name: "unterminated comment masks to end of input",
			src:  "a{# {{ vars.x }}\ntrailing",
			want: "a\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, MaskLiterals(tt.src))
		})
	}
}

func TestUT_MaskLiterals_LeavesRealReferencesIntact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "expression outside any block",
			src:  "{{ vars.x }}",
			want: "{{ vars.x }}",
		},
		{
			name: "expression after a closed raw block",
			src:  "{% raw %}{{ vars.x }}{% endraw %}{{ vars.later }}",
			want: "{{ vars.later }}",
		},
		{
			name: "expression after a closed comment",
			src:  "{# {{ vars.x }} #}{{ vars.later }}",
			want: "{{ vars.later }}",
		},
		{
			name: "unbalanced quote inside a raw body does not swallow the rest of the file",
			src:  `{% raw %}{% set x = "oops %}{% endraw %}{{ vars.later }}`,
			want: "{{ vars.later }}",
		},
		{
			name: "raw tag inside a string literal does not open a raw block",
			src:  `{{ f("{% raw %}") ~ vars.x }}{{ vars.later }}`,
			want: `{{ f("{% raw %}") ~ vars.x }}{{ vars.later }}`,
		},
		{
			name: "comment delimiter inside a string literal does not open a comment",
			src:  `{{ f("{#") ~ vars.x }}{{ vars.later }}`,
			want: `{{ f("{#") ~ vars.x }}{{ vars.later }}`,
		},
		{
			name: "references on the same line either side of a raw block",
			src:  "{{ vars.a }}{% raw %}{{ vars.b }}{% endraw %}{{ vars.c }}",
			want: "{{ vars.a }}{{ vars.c }}",
		},
		{
			name: "endraw without an opener is an ordinary statement block",
			src:  "{% endraw %}{{ vars.later }}",
			want: "{% endraw %}{{ vars.later }}",
		},
		{
			name: "statement blocks are left alone",
			src:  "{% if vars.x %}yes{% endif %}",
			want: "{% if vars.x %}yes{% endif %}",
		},
		{
			name: "plain text is untouched",
			src:  "raw endraw {# not a comment",
			want: "raw endraw ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, MaskLiterals(tt.src))
		})
	}
}

func TestUT_MaskLiterals_PreservesLineCount(t *testing.T) {
	t.Parallel()

	src := "# {{ vars.project_name }}\n" +
		"{# a comment\nspanning {{ vars.x }}\nthree lines #}\n" +
		"{% raw %}literal {{ vars.pod }}\nand more\n{% endraw %}\n" +
		"{{ vars.tail }}\n"

	assert.Equal(t, countNewlines(src), countNewlines(MaskLiterals(src)))
}

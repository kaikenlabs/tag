package vars

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_ScanRefs_FindsReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []ScannedRef
	}{
		{
			name: "expression",
			src:  "{{ vars.x }}",
			want: []ScannedRef{{Name: "x", Line: 1}},
		},
		{
			name: "statement",
			src:  "{% if vars.x %}yes{% endif %}",
			want: []ScannedRef{{Name: "x", Line: 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ScanRefs(tt.src))
		})
	}
}

func TestUT_ScanRefs_FindsEveryReferenceInABlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []ScannedRef
	}{
		{
			// defect 2: a greedy regex would only see the last reference.
			name: "concatenation",
			src:  "{{ vars.alpha ~ vars.beta }}",
			want: []ScannedRef{{Name: "alpha", Line: 1}, {Name: "beta", Line: 1}},
		},
		{
			name: "three references",
			src:  "{{ vars.a }}-{{ vars.b }}-{{ vars.c }}",
			want: []ScannedRef{{Name: "a", Line: 1}, {Name: "b", Line: 1}, {Name: "c", Line: 1}},
		},
		{
			name: "multi-line statement tag",
			src: `{% if vars.old
      and vars.old != "" %}ok{% endif %}`,
			want: []ScannedRef{{Name: "old", Line: 1}, {Name: "old", Line: 2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ScanRefs(tt.src))
		})
	}
}

func TestUT_ScanRefs_RejectsNonReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []ScannedRef
	}{
		{
			// defect 3: an attribute path is not a vars.* reference.
			name: "attribute path",
			src:  "{{ cfg.vars.attrname }}",
			want: nil,
		},
		{
			name: "attribute path with whitespace around the dot",
			src:  "{{ cfg . vars.attrname }}",
			want: nil,
		},
		{
			name: "another namespace merely ending in vars",
			src:  "{{ myvars.attrname }}",
			want: nil,
		},
		{
			// The full identifier is its own distinct name, not a truncated
			// match of "attrname".
			name: "longer name with attrname as a prefix",
			src:  "{{ vars.attrname_suffix }}",
			want: []ScannedRef{{Name: "attrname_suffix", Line: 1}},
		},
		{
			// Gonja reads vars.0 as index access, not as a variable named "0".
			// A name must start with a letter or underscore, matching
			// varNamePattern in rename.go — so rename-var could never target
			// this, and neither may the scanner.
			name: "digit-leading name is index access, not a reference",
			src:  "{{ vars.0 }}",
			want: nil,
		},
		{
			name: "digit-leading index after a real reference",
			src:  "{{ vars.items.0 }}",
			want: []ScannedRef{{Name: "items", Line: 1}},
		},
		{
			name: "vars with nothing after the dot",
			src:  "{{ vars. }}",
			want: nil,
		},
		{
			name: "attribute access on the variable is accepted",
			src:  "{{ vars.attrname.nested }}",
			want: []ScannedRef{{Name: "attrname", Line: 1}},
		},
		{
			name: "index access on the variable is accepted",
			src:  "{{ vars.attrname[0] }}",
			want: []ScannedRef{{Name: "attrname", Line: 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ScanRefs(tt.src))
		})
	}
}

func TestUT_ScanRefs_MultiLineBlockLineNumbers(t *testing.T) {
	t.Parallel()

	// defect 4: a block spanning lines must still be scanned, and each
	// reference must report the line "vars." actually begins on.
	src := "{{ vars.spans\n    ~ vars.lines }}"
	want := []ScannedRef{
		{Name: "spans", Line: 1},
		{Name: "lines", Line: 2},
	}

	assert.Equal(t, want, ScanRefs(src))
}

func TestUT_ScanRefs_IgnoresStringLiterals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []ScannedRef
	}{
		{
			// defect 1: a vars.NAME string literal inside a function call is
			// not a reference — only the ghost text, no real reference exists.
			name: "string literal argument",
			src:  `{{ replace("{{ vars.ghost }}") }}`,
			want: nil,
		},
		{
			name: "stmt close in literal",
			src:  `{% set s = "%}" %}{{ vars.real }}`,
			want: []ScannedRef{{Name: "real", Line: 1}},
		},
		{
			name: "expr close in literal",
			src:  `{{ f("}}") ~ vars.real }}`,
			want: []ScannedRef{{Name: "real", Line: 1}},
		},
		{
			name: "escaped quote",
			src:  `{{ f("a \" b") ~ vars.real }}`,
			want: []ScannedRef{{Name: "real", Line: 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ScanRefs(tt.src))
		})
	}
}

// TestUT_ScanRefs_SkipsCommentsAndRawBlocks is the regression suite for issue
// #332 (raw-block/comment masking), migrated case-for-case from the deleted
// mask_test.go: every input there is re-expressed here as an expected set of
// names rather than a masked string.
func TestUT_ScanRefs_SkipsCommentsAndRawBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "gonja comment",
			src:  "a{# {{ vars.x }} #}b",
			want: nil,
		},
		{
			name: "multi-line gonja comment",
			src:  "a{# {{ vars.x }}\nstill a comment #}b",
			want: nil,
		},
		{
			name: "raw block",
			src:  "a{% raw %}{{ vars.x }}{% endraw %}b",
			want: nil,
		},
		{
			name: "raw tag with no surrounding whitespace",
			src:  "a{%raw%}{{ vars.x }}{%endraw%}b",
			want: nil,
		},
		{
			name: "raw tag separated by tabs",
			src:  "a{%\traw\t%}{{ vars.x }}{%\tendraw\t%}b",
			want: nil,
		},
		{
			name: "endraw closed with a tab still terminates the block",
			src:  "a{% raw %}{{ vars.x }}{% endraw\t%}{{ vars.later }}",
			want: []string{"later"},
		},
		{
			name: "raw tag separated by newlines",
			src:  "a{%\nraw\n%}{{ vars.x }}{%\nendraw\n%}b",
			want: nil,
		},
		{
			name: "raw block with whitespace control",
			src:  "a{%- raw -%}{{ vars.x }}{%- endraw -%}b",
			want: nil,
		},
		{
			name: "multi-line raw block keeps its newlines",
			src:  "a{% raw %}{{ vars.x }}\n{{ vars.y }}\n{% endraw %}b",
			want: nil,
		},
		{
			name: "comment nested inside a raw body is part of the raw span",
			src:  "a{% raw %}{# {{ vars.x }} #}{% endraw %}b",
			want: nil,
		},
		{
			name: "empty raw block",
			src:  "a{% raw %}{% endraw %}b",
			want: nil,
		},
		{
			name: "adjacent raw blocks",
			src:  "a{% raw %}{{ vars.x }}{% endraw %}{% raw %}{{ vars.y }}{% endraw %}b",
			want: nil,
		},
		{
			name: "raw block spanning the whole input",
			src:  "{% raw %}{{ vars.x }}{% endraw %}",
			want: nil,
		},
		{
			name: "CRLF inside a masked span keeps only the newline",
			src:  "a{# {{ vars.x }}\r\nstill a comment #}b",
			want: nil,
		},
		{
			name: "unterminated raw block masks to end of input",
			src:  "a{% raw %}{{ vars.x }}\ntrailing",
			want: nil,
		},
		{
			name: "unterminated comment masks to end of input",
			src:  "a{# {{ vars.x }}\ntrailing",
			want: nil,
		},
		{
			name: "expression outside any block",
			src:  "{{ vars.x }}",
			want: []string{"x"},
		},
		{
			name: "expression after a closed raw block",
			src:  "{% raw %}{{ vars.x }}{% endraw %}{{ vars.later }}",
			want: []string{"later"},
		},
		{
			name: "expression after a closed comment",
			src:  "{# {{ vars.x }} #}{{ vars.later }}",
			want: []string{"later"},
		},
		{
			name: "unbalanced quote inside a raw body does not swallow the rest of the file",
			src:  `{% raw %}{% set x = "oops %}{% endraw %}{{ vars.later }}`,
			want: []string{"later"},
		},
		{
			name: "raw tag inside a string literal does not open a raw block",
			src:  `{{ f("{% raw %}") ~ vars.x }}{{ vars.later }}`,
			want: []string{"x", "later"},
		},
		{
			name: "comment delimiter inside a string literal does not open a comment",
			src:  `{{ f("{#") ~ vars.x }}{{ vars.later }}`,
			want: []string{"x", "later"},
		},
		{
			name: "references on the same line either side of a raw block",
			src:  "{{ vars.a }}{% raw %}{{ vars.b }}{% endraw %}{{ vars.c }}",
			want: []string{"a", "c"},
		},
		{
			name: "endraw without an opener is an ordinary statement block",
			src:  "{% endraw %}{{ vars.later }}",
			want: []string{"later"},
		},
		{
			name: "statement blocks are left alone",
			src:  "{% if vars.x %}yes{% endif %}",
			want: []string{"x"},
		},
		{
			name: "plain text is untouched",
			src:  "raw endraw {# not a comment",
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

func TestUT_ScanRefs_UnterminatedConstructs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []ScannedRef
	}{
		{
			name: "unterminated expression yields no refs",
			src:  "{{ vars.x",
			want: nil,
		},
		{
			name: "unterminated statement yields no refs",
			src:  "{% if vars.x",
			want: nil,
		},
		{
			// Refs found before the unterminated block still count; scanning
			// simply stops once it hits the construct it cannot close.
			name: "refs before an unterminated block are kept",
			src:  "{{ vars.a }}{{ vars.b",
			want: []ScannedRef{{Name: "a", Line: 1}},
		},
		{
			name: "unterminated comment skips to EOF",
			src:  "{# {{ vars.x }}\ntrailing",
			want: nil,
		},
		{
			name: "unterminated raw block skips to EOF",
			src:  "{% raw %}{{ vars.x }}\ntrailing",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ScanRefs(tt.src))
		})
	}
}

func TestUT_ScanRefs_PreservesLineNumbers(t *testing.T) {
	t.Parallel()

	src := "# {{ vars.project_name }}\n" +
		"{# a comment\nspanning {{ vars.x }}\nthree lines #}\n" +
		"{% raw %}literal {{ vars.pod }}\nand more\n{% endraw %}\n" +
		"{{ vars.tail }}\n"

	want := []ScannedRef{
		{Name: "project_name", Line: 1},
		{Name: "tail", Line: 8},
	}

	assert.Equal(t, want, ScanRefs(src))
}

func TestUT_ScanNames_DedupesInSourceOrder(t *testing.T) {
	t.Parallel()

	src := "{{ vars.b }} {{ vars.a }} {{ vars.b }} {{ vars.a }} {{ vars.c }}"

	assert.Equal(t,
		[]ScannedRef{
			{Name: "b", Line: 1},
			{Name: "a", Line: 1},
			{Name: "b", Line: 1},
			{Name: "a", Line: 1},
			{Name: "c", Line: 1},
		},
		ScanRefs(src),
	)
	assert.Equal(t, []string{"b", "a", "c"}, ScanNames(src))
}

func TestUT_ScanRefs_AgreesWithRenameWalker(t *testing.T) {
	t.Parallel()

	fixtures := []string{
		"{{ vars.a }}",
		"{{ vars.alpha ~ vars.beta }}",
		"{{ cfg.vars.attrname }}",
		"{{ myvars.attrname }}",
		"{{ vars.attrname_suffix }}",
		"{{ vars.attrname.nested }}",
		"{{ vars.attrname[0] }}",
		`{{ replace("{{ vars.ghost }}") }}`,
		"{{ vars.spans\n    ~ vars.lines }}",
		"{% if vars.old\n      and vars.old != \"\" %}ok{% endif %}",
		"{# {{ vars.commented }} #}{{ vars.real }}",
		"{% raw %}{{ vars.hidden }}{% endraw %}{{ vars.after }}",
		`{{ f("}}") ~ vars.real }}`,
		"{{ vars.old",
	}

	// The universe of candidate names across every fixture, plus a handful
	// that never appear anywhere, so the "no match" direction is exercised too.
	candidates := []string{
		"a", "alpha", "beta", "attrname", "attrname_suffix", "ghost",
		"spans", "lines", "old", "commented", "real", "hidden", "after",
		"nonexistent", "vars",
	}

	for fixtureIdx, src := range fixtures {
		// Count occurrences, not just presence: a walk that finds a repeated
		// name once instead of twice would pass a boolean comparison.
		found := make(map[string]int)
		for _, ref := range ScanRefs(src) {
			found[ref.Name]++
		}

		for _, name := range candidates {
			// Several fixtures embed newlines, so index the subtest rather than
			// naming it after the source — an embedded newline in a subtest name
			// makes the -run filter and CI output awkward to work with.
			t.Run(fmt.Sprintf("fixture%02d/%s", fixtureIdx, name), func(t *testing.T) {
				t.Parallel()
				_, renamed := renameInExpressions(src, name, "__renamed__")
				assert.Equal(t, found[name], renamed,
					"ScanRefs and renameInExpressions disagree on %q in %q", name, src)
			})
		}
	}
}

// TestUT_ScanRefs_DeliberateEdgeCases pins behaviour on malformed or unusual
// input that a reviewer would otherwise have to rediscover. Each case here is a
// decision, not an accident: the scanner matches renameInExpressions rather than
// the MaskLiterals pre-pass it replaced, because issue #337 exists to make
// lint, variables and rename-var agree on what counts as a reference.
func TestUT_ScanRefs_DeliberateEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []ScannedRef
	}{
		{
			// The deleted MaskLiterals blanked the whole raw span including its
			// opening tag, so it saw no reference here. renameInExpressions
			// rewrites the opener, so the scanner reads it too — agreement with
			// rename-var outranks matching the removed masking pass. The raw
			// BODY is still skipped, which is what issue #332 actually asked for.
			name: "reference in a raw opener is read; the raw body is not",
			src:  "{% raw vars.opener %}{{ vars.body }}{% endraw %}{{ vars.after }}",
			want: []ScannedRef{{Name: "opener", Line: 1}, {Name: "after", Line: 1}},
		},
		{
			// An unterminated block stops the scan, matching rename's "copy
			// verbatim rather than guess" handling. Anything after it goes
			// unscanned; `tag template lint` reports the syntax error separately
			// through its Gonja parse check, so the input never passes silently.
			name: "unterminated block stops the scan",
			src:  "{% if vars.flag\n{{ vars.after }}",
			want: nil,
		},
		{
			name: "whitespace control on an expression block",
			src:  "{{- vars.trimmed -}}",
			want: []ScannedRef{{Name: "trimmed", Line: 1}},
		},
		{
			name: "whitespace control on a statement block",
			src:  "{%- if vars.trimmed -%}x{%- endif -%}",
			want: []ScannedRef{{Name: "trimmed", Line: 1}},
		},
		{
			name: "CRLF line endings keep line numbers accurate",
			src:  "{{ vars.first }}\r\nplain\r\n{{ vars.third }}\r\n",
			want: []ScannedRef{{Name: "first", Line: 1}, {Name: "third", Line: 3}},
		},
		{
			name: "CRLF inside a multi-line block",
			src:  "{{ vars.spans\r\n    ~ vars.lines }}\r\n{{ vars.tail }}",
			want: []ScannedRef{
				{Name: "spans", Line: 1},
				{Name: "lines", Line: 2},
				{Name: "tail", Line: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ScanRefs(tt.src))
		})
	}
}

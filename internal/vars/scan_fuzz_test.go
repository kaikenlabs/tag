package vars

import (
	"regexp"
	"testing"
)

// candidateNamePattern is a deliberately naive superset matcher: it finds every
// `vars.NAME` byte sequence anywhere in the input, including ones that are not
// real references (inside comments, string literals, or attribute paths). It
// exists only to enumerate the universe of names a differential check must
// consider, so the check covers the "neither side should see this" direction
// as well as the "both sides should" one.
var candidateNamePattern = regexp.MustCompile(`vars\.([a-zA-Z_][a-zA-Z0-9_]*)`)

// FuzzScanRefsAgreesWithRenameWalker is the structural guard behind issue #337:
// `tag template lint`, `tag template variables` (via ScanRefs) and
// `tag template rename-var` (via renameInExpressions) must agree on what counts
// as a variable reference. They are two separate walks over the same grammar,
// so agreement is a property to be enforced, not an assumption.
//
// For every candidate name in the input, the number of references ScanRefs
// reports must equal the number of replacements renameInExpressions makes.
// Equality of counts, not merely of presence, also catches a walk that finds a
// reference twice or skips a repeat.
func FuzzScanRefsAgreesWithRenameWalker(f *testing.F) {
	seeds := []string{
		"{{ vars.a }}",
		"{{ vars.alpha ~ vars.beta }}",
		"{{ cfg.vars.attrname }}",
		"{{ myvars.attrname }}",
		"{{ vars.a }}{{ vars.a }}",
		`{{ replace("{{ vars.ghost }}") }}`,
		`{% if a == "%}" and vars.missing %}x{% endif %}`,
		"{{ vars.spans\n    ~ vars.lines }}",
		"{# {{ vars.commented }} #}{{ vars.real }}",
		"{% raw %}{{ vars.hidden }}{% endraw %}{{ vars.after }}",
		`{{ f("a \" }} b") ~ vars.real }}`,
		"{{ vars.unterminated",
		"{%- if vars.ws -%}x{%- endif -%}",
		"vars.plaintext",
		"{{ vars. }}",
		"{{ vars.a.b[0] }}",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		scanned := make(map[string]int)
		for _, ref := range ScanRefs(src) {
			scanned[ref.Name]++
		}

		// Every name ScanRefs reports must be a name the naive superset also
		// sees; otherwise ScanRefs invented a reference out of nothing.
		for name := range scanned {
			if !candidateNamePattern.MatchString("vars." + name) {
				t.Fatalf("ScanRefs reported name %q that is not a valid vars.NAME in %q", name, src)
			}
		}

		seen := make(map[string]struct{})
		for _, m := range candidateNamePattern.FindAllStringSubmatch(src, -1) {
			name := m[1]
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}

			_, renamed := renameInExpressions(src, name, "zzz")
			if scanned[name] != renamed {
				t.Fatalf(
					"disagreement on %q in %q: ScanRefs found %d, renameInExpressions replaced %d",
					name, src, scanned[name], renamed,
				)
			}
		}
	})
}

package vars

import "strings"

// Statement tags that can introduce a new binding scope. Only these four can
// rebind an identifier in Gonja; every other tag ({% if %}, {% include %}, ...)
// evaluates its expressions in the enclosing scope and cannot shadow anything.
const (
	forTag      = "for"
	endForTag   = "endfor"
	withTag     = "with"
	endWithTag  = "endwith"
	macroTag    = "macro"
	endMacroTag = "endmacro"
	setTag      = "set"
)

// varsShadow tracks whether the `vars` root namespace is currently rebound by an
// enclosing template construct.
//
// This exists because `vars` is an ordinary identifier to Gonja: a template may
// write {% for vars in items %} and, inside that loop, `vars.x` refers to the
// loop variable, not to the template variable `x`. A purely lexical scan cannot
// tell those apart, and getting it wrong is not cosmetic — `template rename-var`
// would rewrite the shadowed reference and corrupt the template, and a
// dependency scan would invent an edge (and, for a self-reference, a fatal
// circular-dependency error) for a variable that is never actually referenced.
//
// Both walkers — ScanRefs and renameInExpressions — drive an instance of this in
// lockstep, which is what keeps them in agreement; see
// FuzzScanRefsAgreesWithRenameWalker.
type varsShadow struct {
	// One entry per open for/with/macro block, true when that block rebinds
	// `vars`. Non-shadowing blocks are still pushed so their end tag pops the
	// right entry when they are nested inside a shadowing one.
	stack []bool
	// {% set vars = ... %} has no end tag; it shadows to the end of the input.
	sticky bool
}

// active reports whether `vars` is currently shadowed.
func (s *varsShadow) active() bool {
	if s.sticky {
		return true
	}
	for _, shadowing := range s.stack {
		if shadowing {
			return true
		}
	}
	return false
}

// observe updates the scope state for one {% ... %} block.
//
// It must be called AFTER the block's own references have been handled: the
// right-hand side of {% for vars in vars.items %} is evaluated in the ENCLOSING
// scope, so `vars.items` there is a genuine reference even though the loop body
// that follows is shadowed.
func (s *varsShadow) observe(block string) {
	switch blockTag(block) {
	case forTag:
		s.stack = append(s.stack, bindsVars(forTargets(blockBody(block))))
	case withTag:
		s.stack = append(s.stack, bindsVars(assignTargets(blockBody(block, withTag))))
	case macroTag:
		s.stack = append(s.stack, bindsVars(macroParams(blockBody(block, macroTag))))
	case endForTag, endWithTag, endMacroTag:
		if len(s.stack) > 0 {
			s.stack = s.stack[:len(s.stack)-1]
		}
	case setTag:
		if bindsVars(assignTargets(blockBody(block, setTag))) {
			s.sticky = true
		}
	}
}

// blockBody returns a {% ... %} block's inner text with the delimiters, any
// whitespace-control markers and the leading tag removed.
func blockBody(block string, tag ...string) string {
	if !strings.HasPrefix(block, stmtOpen) || !strings.HasSuffix(block, stmtClose) {
		return ""
	}
	inner := block[len(stmtOpen) : len(block)-len(stmtClose)]
	inner = strings.TrimSpace(strings.Trim(strings.TrimSpace(inner), "-"))

	lead := forTag
	if len(tag) > 0 {
		lead = tag[0]
	}
	return strings.TrimSpace(strings.TrimPrefix(inner, lead))
}

// forTargets returns the loop targets of a for block's body, i.e. everything
// before the ` in ` separator.
func forTargets(body string) []string {
	targets, _, found := strings.Cut(body, " in ")
	if !found {
		return nil
	}
	return strings.Split(targets, ",")
}

// assignTargets returns the left-hand identifiers of a `set`/`with` body, which
// may assign several names at once: `{% with a = 1, vars = 2 %}`.
func assignTargets(body string) []string {
	var targets []string
	for clause := range strings.SplitSeq(body, ",") {
		lhs, _, found := strings.Cut(clause, "=")
		if !found {
			// `{% set vars %}...{% endset %}` block form, and bare `with`.
			lhs = clause
		}
		targets = append(targets, lhs)
	}
	return targets
}

// macroParams returns the parameter names of a macro block's body.
func macroParams(body string) []string {
	_, params, found := strings.Cut(body, "(")
	if !found {
		return nil
	}
	params, _, _ = strings.Cut(params, ")")

	var names []string
	for param := range strings.SplitSeq(params, ",") {
		name, _, _ := strings.Cut(param, "=") // default values: f(vars="x")
		names = append(names, name)
	}
	return names
}

// bindsVars reports whether any of the given binding targets is `vars` itself.
func bindsVars(targets []string) bool {
	for _, target := range targets {
		if strings.TrimSpace(target) == varsToken {
			return true
		}
	}
	return false
}

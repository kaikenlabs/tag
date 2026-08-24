package vars

import (
	"slices"

	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

// DeclaredDeps returns, for every variable in defs, the sorted names of
// declared variables that its default expression references. Every key of defs
// gets an entry, and every entry is a non-nil slice so it serialises as [].
//
// References are found with ScanNames, the same walker `template lint`,
// `template variables` and `template rename-var` use, so all four agree on what
// a vars.* reference is. A regex over the default string would reintroduce two
// defects the walker exists to fix: a block spanning lines is invisible, and
// only the first reference in a block is seen.
//
// Two deliberate restrictions:
//
// A name not declared in defs is dropped. It is a template-authoring mistake
// that `template lint` reports; a dependency graph with dangling edges is worse
// than one that omits them, and topologicalSortVars has always ignored them.
//
// Scanning happens only when ContainsTemplateExpression accepts the default —
// the same predicate IsDerived and IsEvaluatedDefault use. A default TAG treats
// as a literal string is never rendered, so a vars.* inside it is not a
// dependency, and claiming otherwise would order prompts around an edge that
// does not exist. This keeps depends_on consistent with default_is_expression.
//
// A self-reference is retained. topologicalSortVars needs to see it to raise
// ErrCircularDependency; dropping it here as "not a real dependency" would
// silently disable cycle detection.
func DeclaredDeps(defs map[string]tmplconfig.VariableDef) map[string][]string {
	deps := make(map[string][]string, len(defs))
	for name, def := range defs {
		deps[name] = declaredRefs(def, defs)
	}
	return deps
}

func declaredRefs(def tmplconfig.VariableDef, defs map[string]tmplconfig.VariableDef) []string {
	refs := []string{}

	defaultStr, ok := def.Default.(string)
	if !ok || !tmplconfig.ContainsTemplateExpression(defaultStr) {
		return refs
	}

	for _, name := range ScanNames(defaultStr) {
		if _, declared := defs[name]; declared {
			refs = append(refs, name)
		}
	}
	slices.Sort(refs)

	return refs
}

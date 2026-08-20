package template

import (
	"fmt"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/nikolalohinski/gonja/v2/exec"

	"github.com/kaikenlabs/tag/internal/formats"
)

// RegisterFilters registers all custom filters with the given filter set.
// Returns an error if any filter fails to register.
func RegisterFilters(filters *exec.FilterSet) error {
	// Case transformation filters
	filtersToRegister := []struct {
		name string
		fn   exec.FilterFunction
	}{
		{"snake", filterSnake},
		{"pascal", filterPascal},
		{"camel", filterCamel},
		{"kebab", filterKebab},
		{"lower", filterLower},
		{"upper", filterUpper},
		{"title", filterTitle},
		// Inflection filters
		{"plural", filterPlural},
		{"singular", filterSingular},
		{"ordinalize", filterOrdinalize},
		{"titleize", filterTitleize},
		{"humanize", filterHumanize},
		// Verb tense filters
		{"past", filterPast},
		// String operation filters
		{"split", filterSplit},
		{"join", filterJoin},
		{"contains", filterContains},
		{"hasprefix", filterHasPrefix},
		{"hassuffix", filterHasSuffix},
		{"replace", filterReplace},
		{"trim", filterTrim},
		{"default", filterDefault},
		{"truncate", filterTruncate},
		{"indent", filterIndent},
		// Aliases for common variations
		{"snake_case", filterSnake},
		{"pascal_case", filterPascal},
		{"camel_case", filterCamel},
		{"kebab_case", filterKebab},
		{"pluralize", filterPlural},
		{"singularize", filterSingular},
		{"past_tense", filterPast},
	}

	for _, f := range filtersToRegister {
		if err := registerFilter(filters, f.name, f.fn); err != nil {
			return err
		}
	}

	return nil
}

// registerFilter registers a filter, replacing it if it already exists.
func registerFilter(filters *exec.FilterSet, name string, fn exec.FilterFunction) error {
	if err := filters.Register(name, fn); err != nil {
		// If filter already exists, replace it
		if replaceErr := filters.Replace(name, fn); replaceErr != nil {
			return fmt.Errorf("failed to register filter %s: %w", name, replaceErr)
		}
	}
	return nil
}

// makeSimpleFilter creates a zero-arg filter that transforms the input string.
func makeSimpleFilter(name string, fn func(string) string) exec.FilterFunction {
	return func(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
		if in.IsError() {
			return in
		}
		if err := params.Take(); err != nil {
			return exec.AsValue(fmt.Errorf("%s: %w", name, err))
		}
		return exec.AsValue(fn(in.String()))
	}
}

// Case transformation filters
var (
	filterSnake  = makeSimpleFilter("snake", formats.CaseSnake)
	filterPascal = makeSimpleFilter("pascal", formats.CasePascal)
	filterCamel  = makeSimpleFilter("camel", formats.CaseCamel)
	filterKebab  = makeSimpleFilter("kebab", formats.CaseKebab)
	filterLower  = makeSimpleFilter("lower", strings.ToLower)
	filterUpper  = makeSimpleFilter("upper", strings.ToUpper)
	filterTitle  = makeSimpleFilter("title", formats.CaseTitle)
)

// Inflection filters
var (
	filterPlural     = makeSimpleFilter("plural", flect.Pluralize)
	filterSingular   = makeSimpleFilter("singular", flect.Singularize)
	filterOrdinalize = makeSimpleFilter("ordinalize", flect.Ordinalize)
	filterTitleize   = makeSimpleFilter("titleize", flect.Titleize)
	filterHumanize   = makeSimpleFilter("humanize", flect.Humanize)
	filterPast       = makeSimpleFilter("past", formats.CasePast)
)

// String operation filters

func filterSplit(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	p := params.Args
	if len(p) == 0 {
		// Default split by whitespace
		return exec.AsValue(strings.Fields(in.String()))
	}
	if len(p) > 1 {
		return exec.AsValue(fmt.Errorf("split: expected 0 or 1 argument, got %d", len(p)))
	}
	delimiter := p[0].String()
	return exec.AsValue(strings.Split(in.String(), delimiter))
}

func filterJoin(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	p := params.Args
	separator := ""
	if len(p) > 0 {
		separator = p[0].String()
	}

	// Handle slice/array input
	if in.IsList() {
		var parts []string
		in.Iterate(func(idx, count int, key, value *exec.Value) bool {
			parts = append(parts, key.String())
			return true
		}, func() {})
		return exec.AsValue(strings.Join(parts, separator))
	}

	// If it's not a list, return as-is
	return in
}

func filterContains(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	p := params.Args
	if len(p) != 1 {
		return exec.AsValue(fmt.Errorf("contains: expected 1 argument, got %d", len(p)))
	}
	substr := p[0].String()
	return exec.AsValue(strings.Contains(in.String(), substr))
}

func filterHasPrefix(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	p := params.Args
	if len(p) != 1 {
		return exec.AsValue(fmt.Errorf("hasprefix: expected 1 argument, got %d", len(p)))
	}
	prefix := p[0].String()
	return exec.AsValue(strings.HasPrefix(in.String(), prefix))
}

func filterHasSuffix(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	p := params.Args
	if len(p) != 1 {
		return exec.AsValue(fmt.Errorf("hassuffix: expected 1 argument, got %d", len(p)))
	}
	suffix := p[0].String()
	return exec.AsValue(strings.HasSuffix(in.String(), suffix))
}

func filterReplace(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	p := params.Args
	if len(p) != 2 {
		return exec.AsValue(fmt.Errorf("replace: expected 2 arguments (old, new), got %d", len(p)))
	}
	old := p[0].String()
	newStr := p[1].String()
	return exec.AsValue(strings.ReplaceAll(in.String(), old, newStr))
}

func filterTrim(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	p := params.Args
	if len(p) == 0 {
		return exec.AsValue(strings.TrimSpace(in.String()))
	}
	if len(p) == 1 {
		cutset := p[0].String()
		return exec.AsValue(strings.Trim(in.String(), cutset))
	}
	return exec.AsValue(fmt.Errorf("trim: expected 0 or 1 argument, got %d", len(p)))
}

func filterDefault(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	p := params.Args
	if len(p) != 1 {
		return exec.AsValue(fmt.Errorf("default: expected 1 argument, got %d", len(p)))
	}

	// Return default if input is nil, undefined, empty string, or error
	if in.IsNil() || in.IsError() || (in.IsString() && in.String() == "") {
		return p[0]
	}
	return in
}

// filterIndent indents each line of the input by the given number of spaces.
// Usage: {{ value | indent(4) }} — indents all lines except the first.
// With two args: {{ value | indent(4, true) }} — indents all lines including the first.
func filterIndent(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	p := params.Args
	if len(p) < 1 || len(p) > 2 {
		return exec.AsValue(fmt.Errorf("indent: expected 1-2 arguments (width, [indent_first]), got %d", len(p)))
	}

	width := p[0].Integer()
	indentFirst := false
	if len(p) == 2 {
		indentFirst = p[1].Bool()
	}

	pad := strings.Repeat(" ", width)
	lines := strings.Split(in.String(), "\n")

	var result []string
	for i, line := range lines {
		switch {
		case i == 0 && !indentFirst:
			result = append(result, line)
		case line == "":
			result = append(result, line)
		default:
			result = append(result, pad+line)
		}
	}

	return exec.AsValue(strings.Join(result, "\n"))
}

func filterTruncate(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	p := params.Args
	if len(p) < 1 || len(p) > 2 {
		return exec.AsValue(fmt.Errorf("truncate: expected 1-2 arguments, got %d", len(p)))
	}

	length := p[0].Integer()
	if length < 0 {
		return exec.AsValue(fmt.Errorf("truncate: length must be non-negative, got %d", length))
	}

	ellipsis := "..."
	if len(p) == 2 {
		ellipsis = p[1].String()
	}

	s := in.String()
	// Use runes to properly handle UTF-8 multi-byte characters
	runes := []rune(s)
	if len(runes) <= length {
		return in
	}
	return exec.AsValue(string(runes[:length]) + ellipsis)
}

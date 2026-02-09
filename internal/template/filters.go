package template

import (
	"fmt"
	"strings"

	"github.com/gobuffalo/flect"
	"github.com/kaikenlabs/tag/internal/formats"
	"github.com/nikolalohinski/gonja/v2/exec"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// titleCaser is a cached English title caser to avoid allocation per filter call.
var titleCaser = cases.Title(language.English)

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
		// Aliases for common variations
		{"snake_case", filterSnake},
		{"pascal_case", filterPascal},
		{"camel_case", filterCamel},
		{"kebab_case", filterKebab},
		{"pluralize", filterPlural},
		{"singularize", filterSingular},
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

// Case transformation filters

func filterSnake(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("snake: %w", err))
	}
	return exec.AsValue(formats.CaseSnake(in.String()))
}

func filterPascal(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("pascal: %w", err))
	}
	return exec.AsValue(formats.CasePascal(in.String()))
}

func filterCamel(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("camel: %w", err))
	}
	return exec.AsValue(formats.CaseCamel(in.String()))
}

func filterKebab(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("kebab: %w", err))
	}
	return exec.AsValue(formats.CaseKebab(in.String()))
}

func filterLower(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("lower: %w", err))
	}
	return exec.AsValue(strings.ToLower(in.String()))
}

func filterUpper(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("upper: %w", err))
	}
	return exec.AsValue(strings.ToUpper(in.String()))
}

func filterTitle(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("title: %w", err))
	}
	return exec.AsValue(titleCaser.String(in.String()))
}

// Inflection filters

func filterPlural(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("plural: %w", err))
	}
	return exec.AsValue(flect.Pluralize(in.String()))
}

func filterSingular(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("singular: %w", err))
	}
	return exec.AsValue(flect.Singularize(in.String()))
}

func filterOrdinalize(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("ordinalize: %w", err))
	}
	return exec.AsValue(flect.Ordinalize(in.String()))
}

func filterTitleize(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("titleize: %w", err))
	}
	return exec.AsValue(flect.Titleize(in.String()))
}

func filterHumanize(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	if err := params.Take(); err != nil {
		return exec.AsValue(fmt.Errorf("humanize: %w", err))
	}
	return exec.AsValue(flect.Humanize(in.String()))
}

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

func filterTruncate(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
	if in.IsError() {
		return in
	}
	p := params.Args
	if len(p) < 1 || len(p) > 2 {
		return exec.AsValue(fmt.Errorf("truncate: expected 1-2 arguments, got %d", len(p)))
	}

	length := int(p[0].Integer())
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

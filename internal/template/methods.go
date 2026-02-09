package template

import (
	"strings"

	"github.com/nikolalohinski/gonja/v2/builtins/methods/pystring"
	"github.com/nikolalohinski/gonja/v2/exec"
	"golang.org/x/exp/utf8string"
)

// createCustomStringMethods creates a custom string method set with our modifications.
// This overrides the 'replace' method to make the count argument optional
// (matching Python's str.replace(old, new[, count]) signature).
// All other methods are copied from Gonja's builtins.
//
//nolint:gocyclo,funlen,maintidx // map literal of pystring method wrappers, splitting would reduce readability
func createCustomStringMethods() *exec.MethodSet[string] {
	return exec.NewMethodSet(map[string]exec.Method[string]{
		// Copy all standard string methods from Gonja builtins
		"capitalize": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).Capitalize(), nil
		},
		"capwords": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).CapWords(), nil
		},
		"casefold": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).Casefold(), nil
		},
		"center": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var (
				width    int
				fillchar string
			)
			if err := arguments.Take(
				exec.PositionalArgument("width", nil, exec.IntArgument(&width)),
				exec.PositionalArgument("fillchar", exec.AsValue(' '), exec.StringArgument(&fillchar)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).Center(width, utf8string.NewString(fillchar).At(0)), nil
		},
		"count": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var (
				sub   string
				start int
				end   int
			)
			if err := arguments.Take(
				exec.PositionalArgument("sub", nil, exec.StringArgument(&sub)),
				exec.PositionalArgument("start", exec.AsValue(0), exec.IntArgument(&start)),
				exec.PositionalArgument("end", exec.AsValue(len(self)), exec.IntArgument(&end)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).Count(pystring.New(self), &start, &end), nil
		},
		"endswith": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var (
				suffix string
				start  int
				end    int
			)
			if err := arguments.Take(
				exec.PositionalArgument("suffix", nil, exec.StringArgument(&suffix)),
				exec.PositionalArgument("start", exec.AsValue(0), exec.IntArgument(&start)),
				exec.PositionalArgument("end", exec.AsValue(len(self)), exec.IntArgument(&end)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).EndsWith(pystring.New(suffix), &start, &end), nil
		},
		"find": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var (
				sub   string
				start int
				end   int
			)
			if err := arguments.Take(
				exec.PositionalArgument("sub", nil, exec.StringArgument(&sub)),
				exec.PositionalArgument("start", exec.AsValue(0), exec.IntArgument(&start)),
				exec.PositionalArgument("end", exec.AsValue(len(self)), exec.IntArgument(&end)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).Find(pystring.New(sub), &start, &end), nil
		},
		"format": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			args := make([]any, 0, len(arguments.Args))
			for _, arg := range arguments.Args {
				args = append(args, arg.Interface())
			}
			kwargs := make(map[string]any)
			for key, value := range arguments.KwArgs {
				kwargs[key] = value.Interface()
			}
			return pystring.PyString(self).Format(args, kwargs)
		},
		"format_map": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			args := make([]any, 0, len(arguments.Args))
			for _, arg := range arguments.Args {
				args = append(args, arg.Interface())
			}
			kwargs := make(map[string]any)
			for key, value := range arguments.KwArgs {
				kwargs[key] = value.Interface()
			}
			return pystring.PyString(self).FormatMap(args, kwargs)
		},
		"isalnum": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).IsAlnum(), nil
		},
		"isalpha": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).IsAlpha(), nil
		},
		"isascii": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).IsASCII(), nil
		},
		"isdecimal": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).IsDecimal(), nil
		},
		"isdigit": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).IsDigit(), nil
		},
		"islower": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).IsLower(), nil
		},
		"isnumeric": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).IsNumeric(), nil
		},
		"isprintable": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).IsPrintable(), nil
		},
		"isspace": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).IsSpace(), nil
		},
		"istitle": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).IsTitle(), nil
		},
		"isupper": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).IsUpper(), nil
		},
		"join": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var strs []string
			if err := arguments.Take(
				exec.PositionalArgument("iterable", nil, exec.StringListArgument(&strs)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.JoinString(self, strs), nil
		},
		"ljust": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var (
				width    int
				fillchar string
			)
			if err := arguments.Take(
				exec.PositionalArgument("width", nil, exec.IntArgument(&width)),
				exec.PositionalArgument("fillchar", exec.AsValue(' '), exec.StringArgument(&fillchar)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).LJust(width, utf8string.NewString(fillchar).At(0)), nil
		},
		"lower": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).Lower(), nil
		},
		"lstrip": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var cutset string
			if err := arguments.Take(
				exec.PositionalArgument("cutset", exec.AsValue(' '), exec.StringArgument(&cutset)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).LStrip(cutset), nil
		},
		"partition": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var sep string
			if err := arguments.Take(
				exec.PositionalArgument("sep", exec.AsValue(' '), exec.StringArgument(&sep)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			p1, p2, p3 := pystring.PyString(self).Partition(sep)
			return []string{p1.String(), p2.String(), p3.String()}, nil
		},
		"removeprefix": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var prefix string
			if err := arguments.Take(
				exec.PositionalArgument("prefix", nil, exec.StringArgument(&prefix)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).RemovePrefix(prefix), nil
		},
		"removesuffix": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var suffix string
			if err := arguments.Take(
				exec.PositionalArgument("suffix", nil, exec.StringArgument(&suffix)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).RemoveSuffix(suffix), nil
		},
		// MODIFIED: replace now has optional count (default -1 = replace all)
		// This matches Python's str.replace(old, new[, count]) signature
		"replace": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var (
				old    string
				newStr string
				count  int
			)
			if err := arguments.Take(
				exec.PositionalArgument("old", nil, exec.StringArgument(&old)),
				exec.PositionalArgument("new", nil, exec.StringArgument(&newStr)),
				exec.PositionalArgument("count", exec.AsValue(-1), exec.IntArgument(&count)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}

			// count < 0 means replace all (Python behavior)
			if count < 0 {
				return strings.ReplaceAll(self, old, newStr), nil
			}
			return strings.Replace(self, old, newStr, count), nil
		},
		"rfind": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var (
				sub   string
				start int
				end   int
			)
			if err := arguments.Take(
				exec.PositionalArgument("sub", nil, exec.StringArgument(&sub)),
				exec.PositionalArgument("start", exec.AsValue(0), exec.IntArgument(&start)),
				exec.PositionalArgument("end", exec.AsValue(len(self)), exec.IntArgument(&end)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).RFind(sub, &start, &end), nil
		},
		"rjust": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var (
				width    int
				fillchar string
			)
			if err := arguments.Take(
				exec.PositionalArgument("width", nil, exec.IntArgument(&width)),
				exec.PositionalArgument("fillchar", exec.AsValue(' '), exec.StringArgument(&fillchar)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).RJust(width, utf8string.NewString(fillchar).At(0)), nil
		},
		"rpartition": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var sep string
			if err := arguments.Take(
				exec.PositionalArgument("sep", exec.AsValue(' '), exec.StringArgument(&sep)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			p1, p2, p3 := pystring.PyString(self).RPartition(sep)
			return []string{p1.String(), p2.String(), p3.String()}, nil
		},
		"rsplit": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var (
				sep      string
				maxsplit int
			)
			if err := arguments.Take(
				exec.PositionalArgument("sep", exec.AsValue(' '), exec.StringArgument(&sep)),
				exec.PositionalArgument("maxsplit", exec.AsValue(-1), exec.IntArgument(&maxsplit)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).RSplit(sep, maxsplit), nil
		},
		"rstrip": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var cutset string
			if err := arguments.Take(
				exec.PositionalArgument("cutset", exec.AsValue(' '), exec.StringArgument(&cutset)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).RStrip(cutset), nil
		},
		"split": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var (
				sep      string
				maxsplit int
			)
			if err := arguments.Take(
				exec.PositionalArgument("sep", exec.AsValue(' '), exec.StringArgument(&sep)),
				exec.PositionalArgument("maxsplit", exec.AsValue(-1), exec.IntArgument(&maxsplit)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).Split(sep, maxsplit), nil
		},
		"splitlines": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var keepends bool
			if err := arguments.Take(
				exec.PositionalArgument("keepends", exec.AsValue(false), exec.BoolArgument(&keepends)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).SplitLines(keepends), nil
		},
		"startswith": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var (
				prefix   string
				prefixes []string
				start    int
				end      int
			)
			if err := arguments.Take(
				exec.PositionalArgument("prefix", nil, exec.OrArgument(
					exec.StringArgument(&prefix),
					exec.StringListArgument(&prefixes),
				)),
				exec.PositionalArgument("start", exec.AsValue(0), exec.IntArgument(&start)),
				exec.PositionalArgument("end", exec.AsValue(len(self)), exec.IntArgument(&end)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			if prefixes != nil {
				for _, p := range prefixes {
					if pystring.PyString(self).StartsWith(p, &start, &end) {
						return true, nil
					}
				}
				return false, nil
			}
			return pystring.PyString(self).StartsWith(prefix, &start, &end), nil
		},
		"strip": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var cutset string
			if err := arguments.Take(
				exec.PositionalArgument("cutset", exec.AsValue(""), exec.StringArgument(&cutset)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).Strip(cutset), nil
		},
		"swapcase": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).SwapCase(), nil
		},
		"title": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).Title(), nil
		},
		"upper": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			if err := arguments.Take(); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).Upper(), nil
		},
		"zfill": func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
			var width int
			if err := arguments.Take(
				exec.PositionalArgument("width", nil, exec.IntArgument(&width)),
			); err != nil {
				return nil, exec.ErrInvalidCall(err)
			}
			return pystring.PyString(self).ZFill(width), nil
		},
	})
}

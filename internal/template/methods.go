package template

import (
	"strings"

	"github.com/nikolalohinski/gonja/v2/builtins"
	"github.com/nikolalohinski/gonja/v2/exec"
)

// builtinStrMethods lists all known Gonja builtin string method names.
// When Gonja adds new methods, they will be picked up automatically via Get().
var builtinStrMethods = []string{
	"capitalize", "capwords", "casefold", "center", "count",
	"encode", "endswith", "expandtabs", "find", "format", "format_map",
	"isalnum", "isalpha", "isascii", "isdecimal", "isdigit",
	"islower", "isnumeric", "isprintable", "isspace", "istitle", "isupper",
	"join", "ljust", "lower", "lstrip",
	"partition", "removeprefix", "removesuffix", "replace",
	"rfind", "rjust", "rpartition", "rsplit", "rstrip",
	"split", "splitlines", "startswith", "strip", "swapcase",
	"title", "upper", "zfill",
}

// createCustomStringMethods creates a custom string method set with our modifications.
// All builtin methods are harvested from Gonja via Get(), then "replace" is overridden
// to make the count argument optional (matching Python's str.replace(old, new[, count])).
func createCustomStringMethods() *exec.MethodSet[string] {
	m := make(map[string]exec.Method[string], len(builtinStrMethods))
	for _, name := range builtinStrMethods {
		if fn, ok := builtins.Methods.Str.Get(name); ok {
			m[name] = fn
		}
	}

	// Override replace: make count optional with default -1 (replace all),
	// matching Python's str.replace(old, new[, count]) signature.
	m["replace"] = func(self string, _ *exec.Value, arguments *exec.VarArgs) (any, error) {
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
		if count < 0 {
			return strings.ReplaceAll(self, old, newStr), nil
		}
		return strings.Replace(self, old, newStr, count), nil
	}

	return exec.NewMethodSet(m)
}

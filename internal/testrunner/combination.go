package testrunner

import (
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"

	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

// ExtractBooleanVars returns the sorted names of all boolean variables from the template config,
// excluding any in skipVars.
func ExtractBooleanVars(cfg *tmplconfig.TemplateConfig, skipVars []string) []string {
	skip := make(map[string]struct{}, len(skipVars))
	for _, v := range skipVars {
		skip[v] = struct{}{}
	}

	var boolVars []string
	for name, def := range cfg.Vars {
		if def.Type == tmplconfig.VarTypeBoolean {
			if _, skipped := skip[name]; !skipped {
				boolVars = append(boolVars, name)
			}
		}
	}
	sort.Strings(boolVars)
	return boolVars
}

// CombinationCount returns the number of combinations that would be generated
// for the given boolean vars and pin vars, without allocating the full slice.
func CombinationCount(boolVars []string, pinVars map[string]string) int {
	permuted := 0
	for _, v := range boolVars {
		if _, pinned := pinVars[v]; !pinned {
			permuted++
		}
	}
	return 1 << permuted
}

// GenerateCombinations produces all 2^N boolean combinations for the given variable names.
// Pinned variables are excluded from permutation and added with their fixed value.
func GenerateCombinations(boolVars []string, pinVars map[string]string) []Combination {
	// Separate pinned from permuted variables.
	var permuted []string
	for _, v := range boolVars {
		if _, pinned := pinVars[v]; !pinned {
			permuted = append(permuted, v)
		}
	}

	total := 1 << len(permuted)
	combos := make([]Combination, 0, total)

	for i := range total {
		vars := make(map[string]string, len(boolVars))

		// Set permuted variables based on bit pattern.
		for j, name := range permuted {
			if (i>>j)&1 == 1 {
				vars[name] = "true"
			} else {
				vars[name] = "false"
			}
		}

		// Set pinned variables.
		maps.Copy(vars, pinVars)

		combos = append(combos, Combination{
			Index: i,
			Vars:  vars,
		})
	}

	return combos
}

// FilterCombinations applies a filter expression to narrow down combinations.
// Filter can be:
//   - A numeric index (e.g., "7")
//   - Comma-separated key=value pairs (e.g., "use_postgres=true,use_amqp=true")
//
// Returns an error if the filter expression is malformed.
func FilterCombinations(combos []Combination, filter string) ([]Combination, error) {
	if filter == "" {
		return combos, nil
	}

	// Numeric filter: match by combo index.
	if idx, err := strconv.Atoi(filter); err == nil {
		for _, c := range combos {
			if c.Index == idx {
				return []Combination{c}, nil
			}
		}
		return nil, nil
	}

	// Key=value filter: every specified pair must appear in the combo.
	pairs := strings.Split(filter, ",")
	filterMap := make(map[string]string, len(pairs))
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return nil, fmt.Errorf("invalid filter %q: expected key=value format", p)
		}
		filterMap[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	var result []Combination
	for _, c := range combos {
		match := true
		for k, v := range filterMap {
			if cv, ok := c.Vars[k]; !ok || cv != v {
				match = false
				break
			}
		}
		if match {
			result = append(result, c)
		}
	}
	return result, nil
}

// ComboLabel returns a human-readable label for a combination.
func ComboLabel(c Combination, boolVars []string) string {
	parts := make([]string, 0, len(boolVars))
	for _, name := range boolVars {
		parts = append(parts, name+"="+c.Vars[name])
	}
	return strings.Join(parts, " ")
}

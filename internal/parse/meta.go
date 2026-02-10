package parse

import (
	"fmt"
	"log/slog"
	"strings"
)

const (
	keyValueDelimiter = "="
	doubleQuote       = '"'
	singleQuote       = '\''
)

// ParseKeyValues parses a slice of "key=value" strings into a map.
// Values are trimmed of whitespace and boundary quotes (single or double).
// If strict is true, malformed entries cause an error.
// Otherwise they are skipped with a warning.
func ParseKeyValues(args []string, strict bool) (map[string]string, error) {
	result := make(map[string]string)

	for _, arg := range args {
		key, value, ok := parseKeyValue(arg)
		if !ok {
			if strict {
				return nil, fmt.Errorf("invalid meta flag format: %q (expected key=value)", arg)
			}
			slog.Warn("skipping malformed --meta entry", "entry", arg)
			continue
		}
		result[key] = value
	}

	return result, nil
}

// parseKeyValue splits a "key=value" string into its parts.
// It trims whitespace around the entry and key, and strips boundary quotes from the value.
func parseKeyValue(part string) (string, string, bool) {
	part = strings.TrimSpace(part)
	if part == "" {
		return "", "", false
	}

	kv := strings.SplitN(part, keyValueDelimiter, 2)
	if len(kv) != 2 {
		return "", "", false
	}

	key := strings.TrimSpace(kv[0])
	value := stripQuotes(kv[1])

	if key == "" || (value == "" && kv[1] == "") {
		return "", "", false
	}

	return key, value, true
}

// stripQuotes trims whitespace and removes matching boundary quotes from a value.
func stripQuotes(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}

	first, last := value[0], value[len(value)-1]
	// Strip matching boundary quotes (single or double)
	if (first == doubleQuote && last == doubleQuote) ||
		(first == singleQuote && last == singleQuote) {
		return value[1 : len(value)-1]
	}

	// Strip leading unmatched quote
	if first == doubleQuote || first == singleQuote {
		return value[1:]
	}

	return value
}

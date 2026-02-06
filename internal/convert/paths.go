package convert

import (
	"regexp"
	"strings"
)

// cookiecutterPathRegex matches {{ cookiecutter.var }} and {{ cookiecutter.var | filter }} patterns.
// Handles various whitespace combinations.
var cookiecutterPathRegex = regexp.MustCompile(
	`\{\{\s*cookiecutter\.([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:\|\s*([a-zA-Z_][a-zA-Z0-9_]*))?\s*\}\}`,
)

// ConvertPath converts a path from Cookiecutter format to TAG format.
// {{ cookiecutter.var }} becomes {{ vars.var }}
// {{ cookiecutter.var | filter }} becomes {{ vars.var | filter }}
func ConvertPath(path string) (string, bool) {
	converted := false

	result := cookiecutterPathRegex.ReplaceAllStringFunc(path, func(match string) string {
		converted = true
		submatches := cookiecutterPathRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		varName := submatches[1]
		filterName := ""
		if len(submatches) >= 3 && submatches[2] != "" {
			filterName = submatches[2]
		}

		if filterName != "" {
			return "{{ vars." + varName + " | " + filterName + " }}"
		}
		return "{{ vars." + varName + " }}"
	})

	return result, converted
}

// ConvertPathWithDetails returns detailed information about path conversions.
func ConvertPathWithDetails(path string) (string, []PathConversion) {
	var conversions []PathConversion

	matches := cookiecutterPathRegex.FindAllStringSubmatchIndex(path, -1)
	if len(matches) == 0 {
		return path, nil
	}

	// Process matches from end to start to preserve indices
	result := path
	for i := len(matches) - 1; i >= 0; i-- {
		matchIndices := matches[i]
		fullMatch := path[matchIndices[0]:matchIndices[1]]

		// Extract variable name
		varName := path[matchIndices[2]:matchIndices[3]]

		// Extract filter name if present
		filterName := ""
		if matchIndices[4] != -1 && matchIndices[5] != -1 {
			filterName = path[matchIndices[4]:matchIndices[5]]
		}

		var replacement string
		if filterName != "" {
			replacement = "{{ vars." + varName + " | " + filterName + " }}"
		} else {
			replacement = "{{ vars." + varName + " }}"
		}

		conversions = append([]PathConversion{{
			From: fullMatch,
			To:   replacement,
		}}, conversions...)

		result = result[:matchIndices[0]] + replacement + result[matchIndices[1]:]
	}

	return result, conversions
}

// HasCookiecutterPlaceholders checks if a path contains {{ cookiecutter.* }} placeholders.
func HasCookiecutterPlaceholders(path string) bool {
	return cookiecutterPathRegex.MatchString(path)
}

// ExtractCookiecutterVars extracts all variable names from a path.
func ExtractCookiecutterVars(path string) []string {
	matches := cookiecutterPathRegex.FindAllStringSubmatch(path, -1)
	vars := make([]string, 0, len(matches))
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) >= 2 && !seen[match[1]] {
			vars = append(vars, match[1])
			seen[match[1]] = true
		}
	}

	return vars
}

// NormalizePath ensures consistent path separators and removes leading/trailing slashes.
func NormalizePath(path string) string {
	// Convert Windows separators to Unix
	path = strings.ReplaceAll(path, "\\", "/")

	// Remove leading/trailing slashes
	path = strings.Trim(path, "/")

	// Collapse multiple slashes
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	return path
}

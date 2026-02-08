package convert

import (
	"regexp"
	"strings"
)

// expressionBlockRegex matches {{ ... }} expression blocks (Jinja2 style).
// This is used for broader namespace replacement to handle complex expressions.
var expressionBlockRegex = regexp.MustCompile(`\{\{[^}]+\}\}`)

// cookiecutterNamespaceRegex matches "cookiecutter." preceded by a non-word character or start of string.
// This ensures we don't match things like "not_cookiecutter.var".
var cookiecutterNamespaceRegex = regexp.MustCompile(`(^|[^a-zA-Z0-9_])cookiecutter\.`)

// simplePatternRegex matches simple {{ cookiecutter.var }} and {{ cookiecutter.var | filter }} patterns.
// Used for extracting variable names from simple patterns.
var simplePatternRegex = regexp.MustCompile(
	`\{\{\s*cookiecutter\.([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:\|\s*([a-zA-Z_][a-zA-Z0-9_]*))?\s*\}\}`,
)

// ConvertPath converts a path from Cookiecutter format to TAG format.
// Replaces all occurrences of "cookiecutter." with "vars." within {{ }} expression blocks.
// This handles both simple patterns like {{ cookiecutter.var }}
// and complex expressions like {{ cookiecutter.name.lower().replace(' ', '_') }}
func ConvertPath(path string) (string, bool) {
	converted := false

	result := expressionBlockRegex.ReplaceAllStringFunc(path, func(match string) string {
		// Check if this block contains cookiecutter namespace (with word boundary)
		if cookiecutterNamespaceRegex.MatchString(match) {
			converted = true
			// Replace cookiecutter. with vars. while preserving the preceding character
			return cookiecutterNamespaceRegex.ReplaceAllString(match, "${1}vars.")
		}
		return match
	})

	return result, converted
}

// ConvertPathWithDetails returns detailed information about path conversions.
// This handles all cookiecutter expressions, including complex ones with method calls.
func ConvertPathWithDetails(path string) (string, []PathConversion) {
	var conversions []PathConversion

	matches := expressionBlockRegex.FindAllStringIndex(path, -1)
	if len(matches) == 0 {
		return path, nil
	}

	// Process matches from end to start to preserve indices during replacement
	result := path
	for i := len(matches) - 1; i >= 0; i-- {
		start, end := matches[i][0], matches[i][1]
		fullMatch := path[start:end]

		// Check if this block contains cookiecutter namespace (with word boundary)
		if cookiecutterNamespaceRegex.MatchString(fullMatch) {
			replacement := cookiecutterNamespaceRegex.ReplaceAllString(fullMatch, "${1}vars.")

			// Append conversion (will be reversed later to get original order)
			conversions = append(conversions, PathConversion{
				From: fullMatch,
				To:   replacement,
			})

			result = result[:start] + replacement + result[end:]
		}
	}

	// Reverse conversions to get original order (since we processed end to start)
	for i, j := 0, len(conversions)-1; i < j; i, j = i+1, j-1 {
		conversions[i], conversions[j] = conversions[j], conversions[i]
	}

	return result, conversions
}

// HasCookiecutterPlaceholders checks if a path contains {{ cookiecutter.* }} placeholders.
// This detects both simple patterns and complex expressions.
func HasCookiecutterPlaceholders(path string) bool {
	// Check if any expression block contains cookiecutter namespace (with word boundary)
	matches := expressionBlockRegex.FindAllString(path, -1)
	for _, match := range matches {
		if cookiecutterNamespaceRegex.MatchString(match) {
			return true
		}
	}
	return false
}

// ExtractCookiecutterVars extracts variable names from simple {{ cookiecutter.var }} patterns.
// Note: This only extracts from simple patterns, not complex expressions like method calls.
// Complex expressions are converted but variable names aren't extracted for reporting.
func ExtractCookiecutterVars(path string) []string {
	matches := simplePatternRegex.FindAllStringSubmatch(path, -1)
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

package convert

import (
	"regexp"
)

// expressionBlockRegex matches {{ ... }} expression blocks (Jinja2 style).
// This is used for broader namespace replacement to handle complex expressions.
var expressionBlockRegex = regexp.MustCompile(`\{\{[^}]+\}\}`)

// cookiecutterNamespaceRegex matches "cookiecutter." preceded by a non-word character or start of string.
// This ensures we don't match things like "not_cookiecutter.var".
var cookiecutterNamespaceRegex = regexp.MustCompile(`(^|[^a-zA-Z0-9_])cookiecutter\.`)

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

// templateBlockRegex matches all Jinja2 template blocks: {{ }}, {% %}, and {# #}.
var templateBlockRegex = regexp.MustCompile(`\{\{[^}]+\}\}|\{%[^%]+%\}|\{#[^#]+#\}`)

// ConvertContent converts cookiecutter.* references to vars.* in template file content.
// Unlike ConvertPath which only handles {{ }} expression blocks, this also handles
// {% %} control blocks and {# #} comment blocks.
func ConvertContent(content string) (string, bool) {
	converted := false

	result := templateBlockRegex.ReplaceAllStringFunc(content, func(match string) string {
		if cookiecutterNamespaceRegex.MatchString(match) {
			converted = true
			return cookiecutterNamespaceRegex.ReplaceAllString(match, "${1}vars.")
		}
		return match
	})

	return result, converted
}

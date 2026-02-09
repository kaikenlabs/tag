package convert

import (
	"regexp"
)

// cookiecutterNamespaceRegex matches "cookiecutter." preceded by a non-word character or start of string.
// This ensures we don't match things like "not_cookiecutter.var".
var cookiecutterNamespaceRegex = regexp.MustCompile(`(^|[^a-zA-Z0-9_])cookiecutter\.`)

// templateBlockRegex matches all Jinja2 template blocks: {{ }}, {% %}, and {# #}.
var templateBlockRegex = regexp.MustCompile(`\{\{[^}]+\}\}|\{%[^%]+%\}|\{#[^#]+#\}`)

// ConvertPath converts a path from Cookiecutter format to TAG format.
// Replaces all occurrences of "cookiecutter." with "vars." within all Jinja2 blocks:
// {{ }} expression blocks, {% %} control blocks, and {# #} comment blocks.
// This handles simple patterns like {{ cookiecutter.var }}, complex expressions like
// {{ cookiecutter.name.lower().replace(' ', '_') }}, and conditional paths like
// {% if cookiecutter.use_feature=="yes" %}filename{% endif %}.
func ConvertPath(path string) (string, bool) {
	converted := false

	result := templateBlockRegex.ReplaceAllStringFunc(path, func(match string) string {
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

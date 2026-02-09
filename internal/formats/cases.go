package formats

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchSymbol   = regexp.MustCompile(`[\s_-]`)
)

var titleCase = cases.Title(language.English)

// CaseSnake transforms string to `snake_case`
func CaseSnake(str string) string {
	tmp := parseAgainstMatchers(str, "_")
	return strings.ToLower(tmp)
}

// CasePascal transforms string to `PascalCase`
func CasePascal(str string) string {
	tmp := parseAgainstMatchers(str, " ")
	tmp = titleCase.String(tmp)
	return strings.ReplaceAll(tmp, " ", "")
}

// CaseKebab transforms string to `kebab-case`
func CaseKebab(str string) string {
	tmp := parseAgainstMatchers(str, "-")
	return strings.ToLower(tmp)
}

// CaseCamel transforms string to `camelCase`
func CaseCamel(str string) string {
	tmp := CasePascal(str)
	return lowercaseFirst(tmp)
}

// parseAgainstMatchers run matchers against a string + transform
func parseAgainstMatchers(str, sep string) string {
	if matchSymbol.MatchString(str) {
		return matchSymbol.ReplaceAllString(str, sep)
	}
	expression := fmt.Sprintf("${1}%s${2}", sep)
	return matchFirstCap.ReplaceAllString(str, expression)
}

// lowercaseFirst converts the first letter to lower case
func lowercaseFirst(str string) string {
	for i, v := range str {
		return string(unicode.ToLower(v)) + str[i+1:]
	}
	return ""
}

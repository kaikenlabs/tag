package formats

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchSymbol   = regexp.MustCompile(`[\s_-]`)
)

// CaseSnake transforms string to `snake_case`
func CaseSnake(str string) string {
	tmp := parseAgainstMatchers(str, "_")
	return strings.ToLower(tmp)
}

// CaseTitle transforms string to `Title Case`
func CaseTitle(str string) string {
	return cases.Title(language.English).String(str)
}

// CasePascal transforms string to `PascalCase`
func CasePascal(str string) string {
	tmp := parseAgainstMatchers(str, " ")
	tmp = CaseTitle(tmp)
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
// lowercaseFirst converts the first letter to lower case
func lowercaseFirst(str string) string {
	return mapFirstRune(str, unicode.ToLower)
}

// uppercaseFirst converts the first letter to upper case
func uppercaseFirst(str string) string {
	return mapFirstRune(str, unicode.ToUpper)
}

// mapFirstRune applies f to the first rune of str, leaving the rest untouched.
// It decodes rather than slicing at byte 1, which would split a multibyte rune
// and emit invalid UTF-8.
func mapFirstRune(str string, f func(rune) rune) string {
	r, size := utf8.DecodeRuneInString(str)
	if size == 0 {
		return ""
	}
	return string(f(r)) + str[size:]
}

// irregularVerbs maps base forms to past tense for common software-domain verbs.
var irregularVerbs = map[string]string{
	// Unchanged forms
	"set": "set", "put": "put", "cut": "cut", "hit": "hit",
	"let": "let", "shut": "shut", "reset": "reset", "read": "read",
	// Common irregulars in code contexts
	"build": "built", "send": "sent", "spend": "spent", "bind": "bound",
	"find": "found", "hold": "held", "make": "made", "run": "ran",
	"write": "wrote", "get": "got", "begin": "began", "break": "broke",
	"choose": "chose", "do": "did", "go": "went", "have": "had",
	"keep": "kept", "know": "knew", "leave": "left", "lose": "lost",
	"meet": "met", "pay": "paid", "say": "said", "see": "saw",
	"sell": "sold", "speak": "spoke", "stand": "stood", "take": "took",
	"tell": "told", "think": "thought", "understand": "understood",
	"catch": "caught", "teach": "taught", "feel": "felt", "sleep": "slept",
	"win": "won", "give": "gave", "fall": "fell", "hear": "heard",
	"drive": "drove", "freeze": "froze", "undo": "undid", "redo": "redid",
	"override": "overrode", "withdraw": "withdrew",
}

// doubleConsonant lists verbs whose final consonant doubles before -ed.
var doubleConsonant = map[string]bool{
	"stop": true, "drop": true, "ship": true, "plan": true,
	"log": true, "wrap": true, "fit": true, "tag": true,
	"drag": true, "ban": true, "scan": true, "snap": true,
	"skip": true, "pin": true, "pop": true, "tap": true,
	"tip": true, "top": true, "trip": true, "zip": true,
	"commit": true, "submit": true, "permit": true, "omit": true,
	"emit": true, "admit": true, "occur": true, "prefer": true,
	"refer": true, "defer": true, "cancel": true, "control": true,
	"patrol": true, "rebel": true, "excel": true, "compel": true,
}

// toPastTense converts a single lowercase word to its past tense form.
func toPastTense(word string) string {
	lower := strings.ToLower(word)

	if past, ok := irregularVerbs[lower]; ok {
		return past
	}

	// Already ends in -ed — assume already past tense.
	if strings.HasSuffix(lower, "ed") {
		return lower
	}

	// -ic → -icked (panic → panicked)
	if strings.HasSuffix(lower, "ic") {
		return lower + "ked"
	}

	// Ends in -e → just add -d.
	if strings.HasSuffix(lower, "e") {
		return lower + "d"
	}

	// Consonant + y → -ied (copy → copied, try → tried).
	if strings.HasSuffix(lower, "y") && len(lower) > 1 && isConsonant(rune(lower[len(lower)-2])) {
		return lower[:len(lower)-1] + "ied"
	}

	// Whitelist-based consonant doubling.
	if doubleConsonant[lower] {
		return lower + string(lower[len(lower)-1]) + "ed"
	}

	// Default: add -ed.
	return lower + "ed"
}

func isConsonant(r rune) bool {
	return unicode.IsLetter(r) && !strings.ContainsRune("aeiou", unicode.ToLower(r))
}

// splitWords splits a string into words by camelCase/PascalCase boundaries, underscores,
// hyphens, or spaces. Consecutive uppercase letters are treated as an acronym.
func splitWords(s string) []string {
	// Handle explicit separators first.
	if matchSymbol.MatchString(s) {
		return strings.FieldsFunc(s, func(r rune) bool {
			return r == '_' || r == '-' || r == ' '
		})
	}

	// Rune-walk for camelCase/PascalCase with acronym support.
	var words []string
	runes := []rune(s)
	start := 0

	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		cur := runes[i]

		// Split on: lowercase → uppercase (e.g., "orderCancel" → "order" | "Cancel")
		if unicode.IsLower(prev) && unicode.IsUpper(cur) {
			words = append(words, string(runes[start:i]))
			start = i
			continue
		}

		// Split on: uppercase → uppercase+lowercase, keeping the last uppercase with the lowercase
		// (e.g., "HTTPServer" → "HTTP" | "Server")
		if i+1 < len(runes) && unicode.IsUpper(prev) && unicode.IsUpper(cur) && unicode.IsLower(runes[i+1]) {
			words = append(words, string(runes[start:i]))
			start = i
			continue
		}
	}

	if start < len(runes) {
		words = append(words, string(runes[start:]))
	}

	return words
}

// detectSeparator returns the separator character if the string uses one, or empty for camelCase/PascalCase.
func detectSeparator(s string) string {
	if strings.ContainsRune(s, '_') {
		return "_"
	}
	if strings.ContainsRune(s, '-') {
		return "-"
	}
	if strings.ContainsRune(s, ' ') {
		return " "
	}
	return ""
}

// CasePast transforms the last word in the input to past tense, preserving the original casing style.
// Examples: "OrderCancel" → "OrderCancelled", "order_cancel" → "order_cancelled",
// "orderCancel" → "orderCancelled", "cancel" → "cancelled".
func CasePast(str string) string {
	if str == "" {
		return ""
	}

	sep := detectSeparator(str)
	words := splitWords(str)

	if len(words) == 0 {
		return str
	}

	// Apply past tense to the last word.
	last := len(words) - 1
	words[last] = toPastTense(words[last])

	// Rejoin based on detected style.
	if sep != "" {
		// Separated style: preserve original casing of each token.
		return strings.Join(words, sep)
	}

	// PascalCase or camelCase.
	firstRune, _ := utf8.DecodeRuneInString(str)
	isCamel := unicode.IsLower(firstRune)

	var b strings.Builder
	for i, w := range words {
		if w == "" {
			continue
		}

		// Detect if the original word was all-uppercase (acronym like "HTTP").
		// A caseless script (CJK, Arabic) also lands here and is left unchanged.
		isAcronym := len(w) > 1 && w == strings.ToUpper(w)

		switch {
		case isAcronym:
			b.WriteString(w)
		case i == 0 && isCamel:
			b.WriteString(lowercaseFirst(w))
		default:
			b.WriteString(uppercaseFirst(w))
		}
	}
	return b.String()
}

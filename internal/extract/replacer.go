package extract

import (
	"bytes"
	"sort"
	"strings"
	"unicode"

	"github.com/gobuffalo/flect"
)

// BuildRules generates replacement rules for the given entity name.
// The name is normalized to lowercase. Rules are sorted longest-needle-first
// to ensure greedy matching (e.g. "Users" before "User").
func BuildRules(name string) []Rule {
	name = strings.ToLower(name)
	plural := flect.Pluralize(name)

	type entry struct {
		needle string
		expr   string
	}

	var entries []entry

	// Add plural forms only if they differ from singular.
	if plural != name {
		entries = append(entries,
			entry{strings.ToUpper(plural), "{{ name | plural | upper }}"},
			entry{pascalSimple(plural), "{{ name | plural | pascal }}"},
			entry{plural, "{{ name | plural }}"},
		)
	}

	// Singular forms.
	entries = append(entries,
		entry{strings.ToUpper(name), "{{ name | upper }}"},
		entry{pascalSimple(name), "{{ name | pascal }}"},
		entry{name, "{{ name }}"},
	)

	// Deduplicate (e.g. if pascal("user") == "User" and upper("user") == "USER"
	// are fine, but plural might equal singular for uncountable nouns).
	seen := make(map[string]bool, len(entries))
	rules := make([]Rule, 0, len(entries))

	for _, e := range entries {
		if seen[e.needle] {
			continue
		}
		seen[e.needle] = true
		rules = append(rules, Rule{Needle: e.needle, Expr: e.expr})
	}

	// Sort longest first for greedy matching.
	sort.Slice(rules, func(i, j int) bool {
		if len(rules[i].Needle) != len(rules[j].Needle) {
			return len(rules[i].Needle) > len(rules[j].Needle)
		}
		return rules[i].Needle < rules[j].Needle
	})

	return rules
}

// FindOccurrences scans content for all rule matches, returning them sorted
// by position. Uses word-boundary checks to avoid matching inside larger words.
// Matches inside existing template expressions ({{ ... }}) are skipped.
func FindOccurrences(content []byte, rules []Rule) []Occurrence {
	// Pre-compute line starts for line number lookup.
	lineStarts := buildLineStarts(content)

	// Track which byte positions are already claimed by a longer match.
	claimed := make([]bool, len(content))

	var occurrences []Occurrence

	for _, rule := range rules {
		needle := []byte(rule.Needle)
		needleLen := len(needle)

		for offset := 0; offset <= len(content)-needleLen; {
			idx := bytes.Index(content[offset:], needle)
			if idx < 0 {
				break
			}

			pos := offset + idx
			offset = pos + 1

			// Skip if any byte in this range is already claimed.
			if isRangeClaimed(claimed, pos, pos+needleLen) {
				continue
			}

			// Word boundary check.
			if !isWordBoundary(content, pos, needleLen) {
				continue
			}

			// Skip matches inside template expressions.
			if insideTemplateExpr(content, pos) {
				continue
			}

			lineNum := lineNumberAt(lineStarts, pos)
			lineText := extractLine(content, lineStarts, lineNum)

			occ := Occurrence{
				Start:   pos,
				End:     pos + needleLen,
				Rule:    rule,
				LineNum: lineNum,
				Context: lineText,
			}
			occurrences = append(occurrences, occ)

			// Claim these bytes so shorter rules don't overlap.
			for i := pos; i < pos+needleLen; i++ {
				claimed[i] = true
			}
		}
	}

	// Sort by position for stable processing.
	sort.Slice(occurrences, func(i, j int) bool {
		return occurrences[i].Start < occurrences[j].Start
	})

	return occurrences
}

// Apply replaces occurrences in content with their template expressions.
// Processes right-to-left to preserve byte offsets.
func Apply(content []byte, occurrences []Occurrence) []byte {
	if len(occurrences) == 0 {
		return content
	}

	result := make([]byte, len(content))
	copy(result, content)

	// Process in reverse order to preserve offsets.
	for i := len(occurrences) - 1; i >= 0; i-- {
		occ := occurrences[i]
		expr := []byte(occ.Rule.Expr)
		result = append(result[:occ.Start], append(expr, result[occ.End:]...)...)
	}

	return result
}

// BuildToPath parameterizes a file path by replacing name occurrences with
// template expressions. Only replaces in the basename and immediate parent
// directory components.
func BuildToPath(path string, rules []Rule) string {
	for _, rule := range rules {
		path = strings.ReplaceAll(path, rule.Needle, rule.Expr)
	}
	return path
}

// isWordBoundary checks whether a match at pos with the given length sits on
// word boundaries. This prevents matching "user" inside "superuser" or "username".
func isWordBoundary(content []byte, pos, matchLen int) bool {
	return isLeadingBoundary(content, pos) && isTrailingBoundary(content, pos+matchLen)
}

// isLeadingBoundary checks the character before the match position.
func isLeadingBoundary(content []byte, pos int) bool {
	if pos == 0 {
		return true
	}

	prev := rune(content[pos-1])
	cur := rune(content[pos])

	// Non-alphanumeric before match is always a boundary.
	if !unicode.IsLetter(prev) && !unicode.IsDigit(prev) {
		return true
	}

	// Digit before letter is a boundary (e.g. "2user").
	if unicode.IsDigit(prev) && unicode.IsLetter(cur) {
		return true
	}

	// PascalCase boundary: lowercase before uppercase (e.g. "getUser" → "User").
	if unicode.IsLower(prev) && unicode.IsUpper(cur) {
		return true
	}

	return false
}

// isTrailingBoundary checks the character after the match end.
func isTrailingBoundary(content []byte, endPos int) bool {
	if endPos >= len(content) {
		return true
	}

	next := rune(content[endPos])
	prev := rune(content[endPos-1])

	// Non-alphanumeric after match is always a boundary.
	if !unicode.IsLetter(next) && !unicode.IsDigit(next) {
		return true
	}

	// Letter before digit is a boundary (e.g. "user1").
	if unicode.IsLetter(prev) && unicode.IsDigit(next) {
		return true
	}

	// PascalCase boundary: lowercase end + uppercase next (e.g. "User" in "UserHandler").
	if unicode.IsLower(prev) && unicode.IsUpper(next) {
		return true
	}

	// Uppercase end + uppercase next is a boundary (e.g. "USER" in "USER_ID",
	// but also "USER" followed by "Handler" → "USERHandler").
	if unicode.IsUpper(prev) && unicode.IsUpper(next) {
		return true
	}

	return false
}

// insideTemplateExpr checks if a position is inside a {{ ... }} expression.
func insideTemplateExpr(content []byte, pos int) bool {
	// Look backward for the nearest {{ or }}.
	for i := pos - 1; i >= 1; i-- {
		if content[i] == '{' && content[i-1] == '{' {
			// Found opening {{ before pos. Check if there's a matching }} after pos.
			closeIdx := bytes.Index(content[pos:], []byte("}}"))
			return closeIdx >= 0
		}
		if content[i] == '}' && content[i-1] == '}' {
			// Found closing }} before pos — we're outside.
			return false
		}
	}
	return false
}

// buildLineStarts returns byte offsets where each line begins (0-indexed).
func buildLineStarts(content []byte) []int {
	starts := []int{0}
	for i, b := range content {
		if b == '\n' && i+1 < len(content) {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// lineNumberAt returns the 1-indexed line number for a byte position.
func lineNumberAt(lineStarts []int, pos int) int {
	line := sort.SearchInts(lineStarts, pos+1)
	return line
}

// extractLine returns the full text of a 1-indexed line number.
func extractLine(content []byte, lineStarts []int, lineNum int) string {
	if lineNum < 1 || lineNum > len(lineStarts) {
		return ""
	}
	start := lineStarts[lineNum-1]
	end := len(content)
	if lineNum < len(lineStarts) {
		end = lineStarts[lineNum] - 1 // exclude newline
	}
	return string(content[start:end])
}

// isRangeClaimed checks if any byte in [start, end) is already claimed.
func isRangeClaimed(claimed []bool, start, end int) bool {
	for i := start; i < end; i++ {
		if claimed[i] {
			return true
		}
	}
	return false
}

// pascalSimple converts a lowercase word to PascalCase (capitalize first letter).
func pascalSimple(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

package vars

import (
	"strings"
)

// Gonja block delimiters.
const (
	exprOpen   = "{{"
	exprClose  = "}}"
	stmtOpen   = "{%"
	stmtClose  = "%}"
	cmntOpen   = "{#"
	cmntClose  = "#}"
	rawTag     = "raw"
	endRawTag  = "endraw"
	varsPrefix = "vars."
	varsToken  = "vars"
)

// renameInExpressions rewrites `vars.<oldName>` to `vars.<newName>` inside Gonja
// expression ({{ }}) and statement ({% %}) blocks, returning the rewritten source
// and the number of replacements made.
//
// Plain text is never touched, which is what makes the rename safe on template
// bodies that happen to mention the variable name in prose. Three regions are
// deliberately left verbatim because their contents are not variable references:
// comments ({# #}), {% raw %} blocks (emitted literally, so rewriting them would
// change generated output), and string literals inside a block.
//
// The scan is byte-oriented and quote-aware, so a delimiter inside a string
// literal — `{{ f("}}") ~ vars.x }}` — does not terminate the block early.
// An unterminated block is copied verbatim rather than guessed at.
//
// Replacement never changes the number of newlines, so callers can derive line
// numbers by comparing the input and output line-by-line.
func renameInExpressions(src, oldName, newName string) (string, int) {
	var (
		b      strings.Builder
		count  int
		i      int
		shadow varsShadow
	)
	b.Grow(len(src))

	for i < len(src) {
		switch {
		case strings.HasPrefix(src[i:], cmntOpen):
			i = copyComment(&b, src, i)

		case strings.HasPrefix(src[i:], stmtOpen):
			end := scanBlock(src, i+len(stmtOpen), stmtClose)
			if end < 0 {
				b.WriteString(src[i:])
				return b.String(), count
			}
			block := src[i:end]
			// Rewritten before observe, mirroring ScanRefs exactly: a shadowing
			// block's own right-hand side is still the enclosing scope.
			if shadow.active() {
				b.WriteString(block)
			} else {
				b.WriteString(rewriteBlock(block, oldName, newName, &count))
			}
			shadow.observe(block)
			i = end
			// A {% raw %} block emits its body literally; skip to {% endraw %}.
			if blockTag(block) == rawTag {
				i = skipRawBody(&b, src, i)
			}

		case strings.HasPrefix(src[i:], exprOpen):
			end := scanBlock(src, i+len(exprOpen), exprClose)
			if end < 0 {
				b.WriteString(src[i:])
				return b.String(), count
			}
			if shadow.active() {
				b.WriteString(src[i:end])
			} else {
				b.WriteString(rewriteBlock(src[i:end], oldName, newName, &count))
			}
			i = end

		default:
			b.WriteByte(src[i])
			i++
		}
	}

	return b.String(), count
}

// copyComment copies the {# ... #} comment starting at start verbatim and
// returns the index just past it. An unterminated comment consumes the rest of
// src, matching Gonja's own "everything after {# is comment" behaviour.
func copyComment(b *strings.Builder, src string, start int) int {
	end := commentEnd(src, start)
	b.WriteString(src[start:end])
	return end
}

// commentEnd returns the index just past the {# ... #} comment starting at
// start, which must point at "{#". An unterminated comment consumes the
// remainder of src, matching Gonja's own "everything after {# is comment"
// behaviour. Shared by the rename walker and the reference scanner so both
// agree on where a comment ends.
func commentEnd(src string, start int) int {
	idx := strings.Index(src[start+len(cmntOpen):], cmntClose)
	if idx < 0 {
		return len(src)
	}
	return start + len(cmntOpen) + idx + len(cmntClose)
}

// skipRawBody copies everything from start up to and including the closing
// {% endraw %} tag verbatim, returning the index just past it. Nested raw blocks
// are not a thing in Gonja, so the first endraw wins.
//
// A raw body is literal text, so the search for the closing tag is literal too:
// quote-aware scanning here would let an unbalanced quote in the body swallow
// the real {% endraw %} and everything after it.
func skipRawBody(b *strings.Builder, src string, start int) int {
	end := rawBodyEnd(src, start)
	b.WriteString(src[start:end])
	return end
}

// rawBodyEnd scans forward from start for the {% endraw %} tag that closes an
// open {% raw %} block, returning the index just past it. The scan is literal
// rather than quote-aware: a raw body is plain text, so an unbalanced quote
// inside it must not swallow the real {% endraw %} and everything after it.
// An unterminated raw block consumes the remainder of src, matching Gonja's
// own "everything after the opener belongs to it" behaviour. Shared by the
// rename walker and the reference scanner so both agree on where a raw body
// ends.
func rawBodyEnd(src string, start int) int {
	i := start
	for i < len(src) {
		if !strings.HasPrefix(src[i:], stmtOpen) {
			i++
			continue
		}
		idx := strings.Index(src[i+len(stmtOpen):], stmtClose)
		if idx < 0 {
			return len(src)
		}
		end := i + len(stmtOpen) + idx + len(stmtClose)
		if blockTag(src[i:end]) == endRawTag {
			return end
		}
		i = end
	}
	return len(src)
}

// blockTag returns the first word of a {% ... %} block, with whitespace-control
// markers stripped. It returns "" for blocks that are not statement blocks.
func blockTag(block string) string {
	if !strings.HasPrefix(block, stmtOpen) || !strings.HasSuffix(block, stmtClose) {
		return ""
	}
	inner := block[len(stmtOpen) : len(block)-len(stmtClose)]
	inner = strings.TrimSpace(strings.Trim(strings.TrimSpace(inner), "-"))
	tag, _, _ := strings.Cut(inner, " ")
	return tag
}

// scanBlock returns the index just past the first close delimiter at or after
// start, skipping over single- and double-quoted string literals so that a
// delimiter inside a literal does not end the block. It returns -1 when the
// block is unterminated.
func scanBlock(src string, start int, closing string) int {
	i := start
	for i < len(src) {
		if c := src[i]; c == '\'' || c == '"' {
			i = skipQuoted(src, i)
			continue
		}
		if strings.HasPrefix(src[i:], closing) {
			return i + len(closing)
		}
		i++
	}
	return -1
}

// skipQuoted returns the index just past the string literal starting at src[i]
// (which must be a quote character), honouring backslash escapes. An
// unterminated literal consumes the remainder of src.
func skipQuoted(src string, i int) int {
	quote := src[i]
	i++
	for i < len(src) {
		switch src[i] {
		case '\\':
			i += 2
		case quote:
			return i + 1
		default:
			i++
		}
	}
	return len(src)
}

// subscriptRef describes a `vars["name"]` subscript reference located inside a
// block. name is the key's bytes; keyOpen and keyClose index the opening and
// closing quote of the key; end is the index just past the closing `]`.
type subscriptRef struct {
	name     string
	keyOpen  int
	keyClose int
	end      int
}

// matchSubscript reports whether block[i:] begins a `vars["name"]` subscript
// reference and, if so, describes it. i must point at the candidate `vars`
// token. It accepts single or double quotes and tolerates whitespace around the
// token, the brackets and the key, matching Gonja's subscript grammar so
// `vars [ "x" ]` is recognised. The key bytes are taken literally, so the name
// is not restricted to a bare identifier beyond its first byte — subscript
// access exists precisely to reach names like "param.in".
//
// Two forms are deliberately not matched: a non-literal subscript (`vars[expr]`,
// whose key is not statically known) and a key whose first byte is not a letter
// or underscore (`vars["0bad"]`, index access — the same letter/underscore-start
// rule dot access applies). Both keep the scanner and the rename walker
// enumerating the same names.
//
// The left-context guard (isWholeReference) is the caller's job, so
// `cfg.vars["x"]` and `myvars["x"]` are rejected exactly as their dot-access
// equivalents are.
func matchSubscript(block string, i int) (subscriptRef, bool) {
	if !strings.HasPrefix(block[i:], varsToken) {
		return subscriptRef{}, false
	}
	j := skipSpace(block, i+len(varsToken))
	if j >= len(block) || block[j] != '[' {
		return subscriptRef{}, false
	}
	j = skipSpace(block, j+1)
	if j >= len(block) || (block[j] != '\'' && block[j] != '"') {
		return subscriptRef{}, false
	}
	keyOpen := j
	if keyOpen+1 >= len(block) || !isIdentStartByte(block[keyOpen+1]) {
		return subscriptRef{}, false
	}
	keyEnd := skipQuoted(block, keyOpen) // index just past the closing quote
	// skipQuoted consumes the rest of the block on an unterminated literal; a
	// real subscript must have a matching closing quote.
	if keyEnd <= keyOpen+1 || block[keyEnd-1] != block[keyOpen] {
		return subscriptRef{}, false
	}
	j = skipSpace(block, keyEnd)
	if j >= len(block) || block[j] != ']' {
		return subscriptRef{}, false
	}
	return subscriptRef{
		name:     block[keyOpen+1 : keyEnd-1],
		keyOpen:  keyOpen,
		keyClose: keyEnd - 1,
		end:      j + 1,
	}, true
}

// skipSpace returns the index of the first non-whitespace byte at or after i,
// skipping the spaces, tabs, carriage returns and newlines Gonja tolerates
// inside an expression.
func skipSpace(block string, i int) int {
	for i < len(block) {
		switch block[i] {
		case ' ', '\t', '\r', '\n':
			i++
		default:
			return i
		}
	}
	return i
}

// rewriteBlock replaces vars.<oldName> with vars.<newName> inside a single
// block, skipping string literals, and adds the number of replacements to count.
func rewriteBlock(block, oldName, newName string, count *int) string {
	needle := varsPrefix + oldName
	replacement := varsPrefix + newName

	var b strings.Builder
	b.Grow(len(block))

	i := 0
	for i < len(block) {
		if c := block[i]; c == '\'' || c == '"' {
			end := skipQuoted(block, i)
			b.WriteString(block[i:end])
			i = end
			continue
		}
		// Subscript access: vars["oldName"]. The key is a quoted run, so it must
		// be recognised here before the quote branch above would consume it
		// verbatim. Rewriting swaps only the key's bytes, preserving quote style
		// and surrounding whitespace.
		if m, ok := matchSubscript(block, i); ok && isWholeReference(block, i, len(varsToken)) {
			if m.name == oldName {
				b.WriteString(block[i : m.keyOpen+1])
				b.WriteString(newName)
				b.WriteString(block[m.keyClose:m.end])
				*count++
			} else {
				b.WriteString(block[i:m.end])
			}
			i = m.end
			continue
		}
		if strings.HasPrefix(block[i:], needle) && isWholeReference(block, i, len(needle)) {
			b.WriteString(replacement)
			i += len(needle)
			*count++
			continue
		}
		b.WriteByte(block[i])
		i++
	}

	return b.String()
}

// isWholeReference reports whether the `vars.<name>` match at block[i:i+width] is
// a complete reference rather than part of a longer identifier or a nested
// attribute path. It rejects `myvars.old`, `cfg.vars.old`, `cfg . vars.old` and
// `vars.old_suffix`, while allowing `vars.old.nested` and `vars.old[0]`.
func isWholeReference(block string, i, width int) bool {
	// Part of a longer identifier, e.g. myvars.old.
	if i > 0 && isIdentByte(block[i-1]) {
		return false
	}
	// An attribute of something else, e.g. cfg.vars.old. Gonja allows
	// whitespace around the dot operator, so skip back over it before deciding.
	// Only the dot check may skip whitespace: a preceding keyword such as the
	// "if" in `{% if vars.old %}` must still count as a real reference.
	if j := lastNonSpaceIndex(block, i); j >= 0 && block[j] == '.' {
		return false
	}
	// A longer variable name that merely starts with this one, e.g. vars.old_x.
	if next := i + width; next < len(block) && isIdentByte(block[next]) {
		return false
	}
	return true
}

// lastNonSpaceIndex returns the index of the last non-space, non-tab byte before
// i, or -1 when there is none.
func lastNonSpaceIndex(block string, i int) int {
	for j := i - 1; j >= 0; j-- {
		if block[j] != ' ' && block[j] != '\t' {
			return j
		}
	}
	return -1
}

func isIdentByte(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

// isIdentStartByte reports whether c may begin a variable name, matching
// varNamePattern in rename.go: a letter or underscore, never a digit.
func isIdentStartByte(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z')
}

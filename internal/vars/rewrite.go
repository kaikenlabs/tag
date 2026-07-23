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
		b     strings.Builder
		count int
		i     int
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
			b.WriteString(rewriteBlock(block, oldName, newName, &count))
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
			b.WriteString(rewriteBlock(src[i:end], oldName, newName, &count))
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
	idx := strings.Index(src[start+len(cmntOpen):], cmntClose)
	if idx < 0 {
		b.WriteString(src[start:])
		return len(src)
	}
	end := start + len(cmntOpen) + idx + len(cmntClose)
	b.WriteString(src[start:end])
	return end
}

// skipRawBody copies everything from start up to and including the closing
// {% endraw %} tag verbatim, returning the index just past it. Nested raw blocks
// are not a thing in Gonja, so the first endraw wins.
//
// A raw body is literal text, so the search for the closing tag is literal too:
// quote-aware scanning here would let an unbalanced quote in the body swallow
// the real {% endraw %} and everything after it.
func skipRawBody(b *strings.Builder, src string, start int) int {
	i := start
	for i < len(src) {
		if !strings.HasPrefix(src[i:], stmtOpen) {
			b.WriteByte(src[i])
			i++
			continue
		}
		idx := strings.Index(src[i+len(stmtOpen):], stmtClose)
		if idx < 0 {
			b.WriteString(src[i:])
			return len(src)
		}
		end := i + len(stmtOpen) + idx + len(stmtClose)
		block := src[i:end]
		b.WriteString(block)
		i = end
		if blockTag(block) == endRawTag {
			return i
		}
	}
	return i
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

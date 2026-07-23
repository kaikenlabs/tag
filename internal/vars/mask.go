package vars

import (
	"strings"
)

// MaskLiterals blanks the regions of a Gonja template whose contents are emitted
// literally instead of evaluated — comments ({# #}) and {% raw %} blocks — so a
// variable scan does not mistake their contents for references. A literal
// {{ vars.pod }} kept inside a raw block is output, not a reference to `pod`.
//
// A masked span is replaced by the newlines it contains and nothing else, so
// every line that survives keeps its original line number.
//
// Expression and statement blocks are copied verbatim and scanned quote-aware,
// which stops a delimiter inside a string literal — {{ f("{# ") }} — from opening
// a comment that was never there. An unterminated comment or raw block masks to
// the end of input, matching Gonja's own "everything after the opener belongs to
// it" behaviour.
func MaskLiterals(src string) string {
	var b strings.Builder
	b.Grow(len(src))

	i := 0
	for i < len(src) {
		switch {
		case strings.HasPrefix(src[i:], cmntOpen):
			i = maskComment(&b, src, i)

		case strings.HasPrefix(src[i:], stmtOpen):
			end := scanBlock(src, i+len(stmtOpen), stmtClose)
			if end < 0 {
				b.WriteString(src[i:])
				return b.String()
			}
			if blockTag(src[i:end]) == rawTag {
				i = maskRawBlock(&b, src, i)
				continue
			}
			b.WriteString(src[i:end])
			i = end

		case strings.HasPrefix(src[i:], exprOpen):
			end := scanBlock(src, i+len(exprOpen), exprClose)
			if end < 0 {
				b.WriteString(src[i:])
				return b.String()
			}
			b.WriteString(src[i:end])
			i = end

		default:
			b.WriteByte(src[i])
			i++
		}
	}

	return b.String()
}

// maskComment blanks the {# ... #} comment starting at start and returns the
// index just past it.
func maskComment(b *strings.Builder, src string, start int) int {
	idx := strings.Index(src[start+len(cmntOpen):], cmntClose)
	if idx < 0 {
		blank(b, src[start:])
		return len(src)
	}
	end := start + len(cmntOpen) + idx + len(cmntClose)
	blank(b, src[start:end])
	return end
}

// maskRawBlock blanks the {% raw %} tag starting at start, its literal body and
// the closing {% endraw %}, returning the index just past the closing tag.
//
// As in skipRawBody, the search for the close tag is literal rather than
// quote-aware: a raw body is plain text, so an unbalanced quote inside it must
// not swallow the real {% endraw %} and everything after it.
func maskRawBlock(b *strings.Builder, src string, start int) int {
	i := start
	for i < len(src) {
		if !strings.HasPrefix(src[i:], stmtOpen) {
			i++
			continue
		}
		idx := strings.Index(src[i+len(stmtOpen):], stmtClose)
		if idx < 0 {
			break
		}
		end := i + len(stmtOpen) + idx + len(stmtClose)
		if blockTag(src[i:end]) == endRawTag {
			blank(b, src[start:end])
			return end
		}
		i = end
	}

	blank(b, src[start:])
	return len(src)
}

// blank writes only the newlines contained in span, discarding everything else.
func blank(b *strings.Builder, span string) {
	b.WriteString(strings.Repeat("\n", strings.Count(span, "\n")))
}

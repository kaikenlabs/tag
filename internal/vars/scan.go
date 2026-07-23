package vars

import (
	"strings"
)

// ScannedRef is one vars.NAME reference found in template source.
type ScannedRef struct {
	Name string
	Line int // 1-based, the line where "vars." begins
}

// ScanRefs walks Gonja template source and returns every vars.NAME reference
// it contains, in source order, duplicates included.
//
// It walks src exactly like renameInExpressions (see rewrite.go): comments
// ({# #}) and {% raw %} bodies are skipped entirely, plain text is never
// inspected, blocks are located with the shared quote-aware scanBlock, and
// inside a block string literals are skipped via skipQuoted so a name that
// merely appears inside a literal is not mistaken for a reference.
//
// An unterminated block yields no further refs, mirroring rename's "copy
// verbatim rather than guess" handling of unterminated input. An unterminated
// comment or raw block skips to end of input.
//
// Line numbers are 1-based and CRLF-safe (a CRLF pair contains exactly one
// "\n" byte). A reference inside a multi-line block reports the line "vars."
// begins on, not the block's start line.
func ScanRefs(src string) []ScannedRef {
	var refs []ScannedRef
	line := 1
	i := 0

	for i < len(src) {
		switch {
		case strings.HasPrefix(src[i:], cmntOpen):
			end := commentEnd(src, i)
			line += strings.Count(src[i:end], "\n")
			i = end

		case strings.HasPrefix(src[i:], stmtOpen):
			end := scanBlock(src, i+len(stmtOpen), stmtClose)
			if end < 0 {
				return refs
			}
			block := src[i:end]
			refs = scanBlockRefs(refs, block, line)
			line += strings.Count(block, "\n")
			i = end
			// A {% raw %} block emits its body literally; skip to {% endraw %}.
			if blockTag(block) == rawTag {
				rawEnd := rawBodyEnd(src, i)
				line += strings.Count(src[i:rawEnd], "\n")
				i = rawEnd
			}

		case strings.HasPrefix(src[i:], exprOpen):
			end := scanBlock(src, i+len(exprOpen), exprClose)
			if end < 0 {
				return refs
			}
			block := src[i:end]
			refs = scanBlockRefs(refs, block, line)
			line += strings.Count(block, "\n")
			i = end

		default:
			if src[i] == '\n' {
				line++
			}
			i++
		}
	}

	return refs
}

// ScanNames returns the deduplicated variable names ScanRefs finds, in the
// order each first appears.
func ScanNames(src string) []string {
	seen := make(map[string]struct{})
	var names []string

	for _, ref := range ScanRefs(src) {
		if _, ok := seen[ref.Name]; ok {
			continue
		}
		seen[ref.Name] = struct{}{}
		names = append(names, ref.Name)
	}

	return names
}

// scanBlockRefs finds every vars.NAME reference inside a single {{ }} or
// {% %} block, skipping string literals, and appends them to refs. startLine
// is the line block[0] is on; each appended ref records the line it actually
// begins on, which can differ from startLine inside a multi-line block.
func scanBlockRefs(refs []ScannedRef, block string, startLine int) []ScannedRef {
	line := startLine
	i := 0

	for i < len(block) {
		c := block[i]

		if c == '\'' || c == '"' {
			end := skipQuoted(block, i)
			line += strings.Count(block[i:end], "\n")
			i = end
			continue
		}

		if strings.HasPrefix(block[i:], varsPrefix) {
			nameStart := i + len(varsPrefix)
			// A name must begin with a letter or underscore. Gonja reads the
			// digit-leading form (vars.0) as index access, and rename.go's
			// varNamePattern refuses such a name outright, so treating it as a
			// reference here would put the scanner at odds with rename-var.
			if nameStart < len(block) && isIdentStartByte(block[nameStart]) {
				nameEnd := nameStart
				for nameEnd < len(block) && isIdentByte(block[nameEnd]) {
					nameEnd++
				}
				if isWholeReference(block, i, nameEnd-i) {
					refs = append(refs, ScannedRef{Name: block[nameStart:nameEnd], Line: line})
					i = nameEnd
					continue
				}
			}
		}

		if c == '\n' {
			line++
		}
		i++
	}

	return refs
}

package writer

import (
	"strings"

	"github.com/kaikenlabs/tag/internal/types"
)

type Inject struct {
	Matcher string
	Clause  types.InjectClause
}

// Validate - exactly 1 clause must be met. Matcher must not be empty
func (i *Inject) Validate() error {
	hasClause := i.Clause == types.InjectBefore || i.Clause == types.InjectAfter
	if !hasClause {
		return ErrNoMatchingClause
	}
	if i.Matcher == "" {
		return ErrNoMatchingExpression
	}
	return nil
}

// lineStartOffset returns the index of the beginning of the line containing pos.
func lineStartOffset(s string, pos int) int {
	i := pos
	for i > 0 && s[i-1] != '\n' {
		i--
	}
	return i
}

// advancePastNewline returns pos advanced past the newline at that position, if any.
func advancePastNewline(s string, pos int) int {
	if pos < len(s) && s[pos] == '\n' {
		return pos + 1
	}
	if pos+1 < len(s) && s[pos] == '\r' && s[pos+1] == '\n' {
		return pos + 2
	}
	return pos
}

// mergeInjection injects data before or after the first occurrence of a matcher within source.
// If the matcher is not found, the source is returned unchanged with an error.
func mergeInjection(source, dataInjection []byte, inject Inject) ([]byte, error) {
	if err := inject.Validate(); err != nil {
		return source, err
	}

	src := string(source)
	idx := strings.Index(src, inject.Matcher)
	if idx == -1 {
		return source, ErrNoMatchingExpression
	}

	var before, after string
	switch inject.Clause {
	case types.InjectBefore:
		// Split at the start of the line containing the marker so that
		// the marker's leading whitespace stays with the marker.
		lineStart := lineStartOffset(src, idx)
		before = src[:lineStart]
		after = src[lineStart:]
	case types.InjectAfter:
		// Advance past the newline following the marker so injected
		// content starts on the next line, not appended to the marker line.
		end := advancePastNewline(src, idx+len(inject.Matcher))
		before = src[:end]
		after = src[end:]
	}

	data := string(dataInjection)

	// Detect line ending style from source.
	lineEnd := "\n"
	if strings.Contains(src, "\r\n") {
		lineEnd = "\r\n"
	}

	// Ensure injected content is separated from the marker by a newline.
	// For InjectBefore: data must end with a newline so the marker stays on its own line.
	// For InjectAfter: if marker is at EOF without trailing newline, insert a separator.
	if inject.Clause == types.InjectBefore && data != "" && data[len(data)-1] != '\n' {
		data += lineEnd
	}
	if inject.Clause == types.InjectAfter && data != "" && after == "" &&
		!strings.HasSuffix(before, "\n") && data[0] != '\n' && data[0] != '\r' {
		before += lineEnd
	}

	var result strings.Builder
	result.WriteString(before)
	result.WriteString(data)
	result.WriteString(after)
	return []byte(result.String()), nil
}

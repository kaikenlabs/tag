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
		// Back up to the start of the line containing the marker so that
		// the marker's leading whitespace stays with the marker, not with
		// the injected content.
		lineStart := idx
		for lineStart > 0 && src[lineStart-1] != '\n' {
			lineStart--
		}
		before = src[:lineStart]
		after = src[lineStart:]
	case types.InjectAfter:
		end := idx + len(inject.Matcher)
		// Advance past the newline following the marker so injected
		// content starts on the next line, not appended to the marker line.
		if end < len(src) && src[end] == '\n' {
			end++
		} else if end+1 < len(src) && src[end] == '\r' && src[end+1] == '\n' {
			end += 2
		}
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

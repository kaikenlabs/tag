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
		before = src[:idx]
		after = src[idx:]
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

	// Ensure injected content is separated from the marker by a newline.
	// For InjectBefore: data must end with a newline so the marker stays on its own line.
	// For InjectAfter: data must start on the line after the marker (handled above by
	// advancing past the marker's trailing newline).
	if inject.Clause == types.InjectBefore && data != "" && data[len(data)-1] != '\n' {
		data += "\n"
	}

	var result strings.Builder
	result.WriteString(before)
	result.WriteString(data)
	result.WriteString(after)
	return []byte(result.String()), nil
}

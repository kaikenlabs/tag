package writer

import (
	"strings"

	"github.com/kaikenlabs/tag/internal/types"
)

// InjectClause is an alias for types.InjectClause.
type InjectClause = types.InjectClause

const (
	InjectBefore = types.InjectBefore
	InjectAfter  = types.InjectAfter
)

type Inject struct {
	Matcher string
	Clause  InjectClause
}

// Validate - exactly 1 clause must be met. Matcher must not be empty
func (i *Inject) Validate() error {
	hasClause := i.Clause == InjectBefore || i.Clause == InjectAfter
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
	case InjectBefore:
		before = src[:idx]
		after = src[idx:]
	case InjectAfter:
		end := idx + len(inject.Matcher)
		before = src[:end]
		after = src[end:]
	}

	var result strings.Builder
	result.WriteString(before)
	result.Write(dataInjection)
	result.WriteString(after)
	return []byte(result.String()), nil
}

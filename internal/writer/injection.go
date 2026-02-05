package writer

import (
	"fmt"
	"strings"
)

type InjectClause string

const (
	InjectBefore InjectClause = "Before"
	InjectAfter  InjectClause = "After"
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
	if len(i.Matcher) <= 0 {
		return ErrNoMatchingExpression
	}
	return nil
}

// mergeInjection - injects data before, or after a matcher within a source file.
// if we can't find the matcher, don't do anything to the source file
func mergeInjection(source, dataInjection []byte, inject Inject) ([]byte, error) {
	var splitByMatcher []string
	if err := inject.Validate(); err != nil {
		return source, err
	}

	switch inject.Clause {
	case InjectAfter:
		splitByMatcher = strings.SplitAfter(string(source), inject.Matcher)
		if len(splitByMatcher) != 2 {
			return source, ErrNoMatchingExpression
		}
	case InjectBefore:
		idx := strings.Index(string(source), inject.Matcher)
		if idx == -1 {
			return source, ErrNoMatchingExpression
		}
		splitByMatcher = []string{
			string(source[:(idx - 1)]),
			fmt.Sprintf("\n%s", string(source[idx:])),
		}
	}

	formatedOutput := strings.Join([]string{
		splitByMatcher[0],
		string(dataInjection),
		splitByMatcher[1],
	}, "")
	return []byte(formatedOutput), nil
}

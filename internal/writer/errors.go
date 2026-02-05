package writer

import "errors"

var (
	ErrNoMatchingClause     = errors.New("no matching clause")
	ErrNoMatchingExpression = errors.New("no matching expression")
)

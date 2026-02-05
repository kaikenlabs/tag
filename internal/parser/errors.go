package parser

import "errors"

var (
	ErrParsingBool       = errors.New("unable to parse bool")
	ErrMalformedTemplate = errors.New("malformed template data")
)

package parser

import "text/template"

type TemplateEngine struct {
	templates       map[string]string
	sharedTemplates map[string]string
	funcs           template.FuncMap
}

type NameOptions struct {
	PascalCase string
	CamelCase  string
	SnakeCase  string
	KebabCase  string
	LowerCase  string
	UpperCase  string
}

type TemplateData struct {
	Name   string
	To     string
	Output []byte
	ParseData
}

type ParseActions string

const (
	ActionCreate ParseActions = "Create"
	ActionAppend ParseActions = "Append"
	ActionInject ParseActions = "Inject"
)

type InjectClause string

const (
	InjectBefore InjectClause = "Before"
	InjectAfter  InjectClause = "After"
)

type ParseData struct {
	Action        ParseActions
	InjectClause  InjectClause
	InjectMatcher string
	Args          string
	N             NameOptions
	Meta          map[string]string
	Notes         string
}

package template

// TemplateRenderer handles template parsing and execution.
type TemplateRenderer interface {
	ParseString(content string) (Template, error)
	ParseStringNamed(content, name string) (Template, error)
	ExecuteToString(content string, ctx Context) (string, error)
}

// MetadataParser handles legacy metadata block extraction.
type MetadataParser interface {
	RenderAndParseMetadata(metaRaw string, ctx Context) (*Metadata, error)
}

// TemplateExecutor combines rendering and metadata parsing capabilities.
type TemplateExecutor interface {
	TemplateRenderer
	MetadataParser
}

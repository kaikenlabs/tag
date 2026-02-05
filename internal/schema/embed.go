package schema

import _ "embed"

// TemplateConfigSchema is the embedded JSON Schema for tag.template.json validation.
//
//go:embed tag.template.schema.json
var TemplateConfigSchema string

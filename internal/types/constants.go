package types

// Template and configuration file/directory constants used across the codebase.
const (
	// TemplateConfigFile is the name of the TAG template configuration file.
	TemplateConfigFile = "tag.template.json"

	// CookiecutterConfigFile is the name of the Cookiecutter configuration file.
	CookiecutterConfigFile = "cookiecutter.json"

	// GeneratorsDir is the directory within a template that contains generators.
	GeneratorsDir = "_generators"

	// TemplatesDir is the directory within a scaffolded project that holds generators.
	TemplatesDir = ".tag.templates"

	// SharedDir is the default subdirectory for shared templates.
	SharedDir = "_shared"

	// BundlesDir is the default subdirectory for bundles.
	BundlesDir = "_bundles"
)

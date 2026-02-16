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
	TemplatesDir = ".tag"

	// SharedDir is the default subdirectory for shared templates.
	SharedDir = "_shared"

	// BundlesDir is the default subdirectory for bundles.
	BundlesDir = "_bundles"

	// BundleExtension is the file extension for bundle definition files.
	BundleExtension = ".json"

	// CacheMetaFile is the remote cache metadata file written by the resolver.
	CacheMetaFile = "_meta.json"

	// TemplateReadme is the template documentation file displayed after scaffolding.
	TemplateReadme = "README.md"

	// TemplateHowto is the optional how-to documentation file included in templates.
	TemplateHowto = "HOWTO.md"
)

// InjectClause represents where to inject content relative to a marker.
type InjectClause string

const (
	InjectBefore InjectClause = "Before"
	InjectAfter  InjectClause = "After"
)

// HooksConfig defines pre and post scaffold hooks.
type HooksConfig struct {
	PreScaffold  []string `json:"pre_scaffold,omitempty"`
	PostScaffold []string `json:"post_scaffold,omitempty"`
}

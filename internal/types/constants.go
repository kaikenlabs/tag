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

	// TagIgnoreFile is the name of the gitignore-style exclusion file for templates.
	TagIgnoreFile = ".tagignore"

	// HistoryFile is the filename for the generation history manifest.
	HistoryFile = "history.json"

	// HistoryBackupsDir is the subdirectory inside TemplatesDir where backups are stored.
	HistoryBackupsDir = "history/backups"

	// TagConfigFile is the name of the project-level scaffold configuration file.
	TagConfigFile = ".tagconfig.json"

	// TagConfigSchemaVersion is the current schema version for .tagconfig.json.
	TagConfigSchemaVersion = 1
)

// TemplateType indicates the source type of a scaffolded template.
type TemplateType string

const (
	// TemplateTypeLocal indicates a template sourced from a local directory.
	TemplateTypeLocal TemplateType = "local"

	// TemplateTypeRemote indicates a template sourced from a remote git/zip origin.
	TemplateTypeRemote TemplateType = "remote"
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

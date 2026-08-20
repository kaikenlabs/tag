package convert

// Options configures the conversion process.
type Options struct {
	Source      string // Local path or remote reference to Cookiecutter template
	Destination string // Output directory for converted TAG template
	DryRun      bool   // Preview mode - show what would be converted without writing
	Force       bool   // Overwrite existing output directory
}

// Result captures the outcomes of a conversion. The slice fields are always
// present as [] rather than omitted or null: Convert initialises them empty
// when it builds the Result, rather than normalising nils on the way out.
type Result struct {
	Source             string               `json:"source"`
	Destination        string               `json:"destination"`
	VariablesConverted int                  `json:"variables_converted"`
	DirsRenamed        int                  `json:"dirs_renamed"`
	FilesRenamed       int                  `json:"files_renamed"`
	FilesProcessed     int                  `json:"files_processed"`
	HooksCopied        int                  `json:"hooks_copied"`
	Incompatibilities  []Incompatibility    `json:"incompatibilities"` // Content incompatibilities found
	Warnings           []string             `json:"warnings"`          // General warnings
	DryRun             bool                 `json:"dry_run"`
	Files              []PathConversion     `json:"files"`     // Per-file path conversions (files only, not directories)
	Variables          []VariableConversion `json:"variables"` // Per-variable conversions
}

// Severity indicates the importance of an incompatibility. It is a defined
// string type, so it serialises as "info"/"warning"/"error" with no extra
// mapping code.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Incompatibility represents a Jinja2/Gonja syntax difference found in content.
type Incompatibility struct {
	Path       string   `json:"path"`                 // File path relative to template root
	Line       int      `json:"line"`                 // Line number (1-based)
	Kind       string   `json:"kind"`                 // Type of incompatibility (e.g., "filter-syntax", "dict-iteration")
	Message    string   `json:"message"`              // Human-readable description
	Original   string   `json:"original,omitempty"`   // Original syntax found
	Suggestion string   `json:"suggestion,omitempty"` // Suggested Gonja equivalent
	Severity   Severity `json:"severity"`             // Severity level
}

// HookFinding represents a detected hook file.
type HookFinding struct {
	Path     string // Path to the hook file
	Kind     string // Type of hook (e.g., "python", "shell")
	Message  string // Warning message about compatibility
	IsCopied bool   // Whether the hook was/would be copied
}

// PathConversion represents a path rename operation.
type PathConversion struct {
	From string `json:"from"` // Original path with {{ cookiecutter.var }}
	To   string `json:"to"`   // Converted path — ConvertPath rewrites the namespace to
	// {{ vars.var }} in place; it does NOT emit a __var__ double-underscore form.
}

// VariableConversion represents a converted variable.
//
// Field-set note (the repo has been bitten by this before: adding --format
// json to `cache ls` would have printed a presigned URL's token that the text
// table never showed). Diffing this document against the text output,
// `default` is the one field that appears only in JSON — printConversionResult
// never prints variable defaults. It is kept deliberately: the per-variable
// mapping is the substance of what #355 asks for, and the values come from the
// template's own cookiecutter.json (project names, licences, author
// placeholders), which is checked-in template metadata rather than a secret
// store. `original` and `suggestion` on Incompatibility are already shown in
// the text output, truncated to 60 characters; JSON carries them in full
// because truncation is a display concern, not a redaction.
type VariableConversion struct {
	Name         string `json:"name"`                 // Variable name
	OriginalType string `json:"original_type"`        // Type in cookiecutter.json (inferred)
	TagType      string `json:"tag_type"`             // Type in tag.template.json
	Default      any    `json:"default,omitempty"`    // Default value
	IsChoice     bool   `json:"is_choice,omitempty"`  // Whether this is a choice variable
	IsPrivate    bool   `json:"is_private,omitempty"` // Whether this starts with _
}

package convert

// Options configures the conversion process.
type Options struct {
	Source      string // Local path or remote reference to Cookiecutter template
	Destination string // Output directory for converted TAG template
	DryRun      bool   // Preview mode - show what would be converted without writing
	Force       bool   // Overwrite existing output directory
}

// Result captures the outcomes of a conversion.
type Result struct {
	Source             string            // Original source path/ref
	Destination        string            // Final output directory
	VariablesConverted int               // Number of variables converted
	DirsRenamed        int               // Number of directories renamed
	FilesRenamed       int               // Number of files renamed
	FilesProcessed     int               // Total files processed
	HooksCopied        int               // Number of hook files copied
	Incompatibilities  []Incompatibility // Content incompatibilities found
	Warnings           []string          // General warnings
	DryRun             bool              // Whether this was a dry run
}

// Severity indicates the importance of an incompatibility.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Incompatibility represents a Jinja2/Gonja syntax difference found in content.
type Incompatibility struct {
	Path       string   // File path relative to template root
	Line       int      // Line number (1-based)
	Kind       string   // Type of incompatibility (e.g., "filter-syntax", "dict-iteration")
	Message    string   // Human-readable description
	Original   string   // Original syntax found
	Suggestion string   // Suggested Gonja equivalent
	Severity   Severity // Severity level
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
	From string // Original path with {{ cookiecutter.var }}
	To   string // Converted path with __var__
}

// VariableConversion represents a converted variable.
type VariableConversion struct {
	Name         string // Variable name
	OriginalType string // Type in cookiecutter.json (inferred)
	TagType      string // Type in tag.template.json
	Default      any    // Default value
	IsChoice     bool   // Whether this is a choice variable
	IsPrivate    bool   // Whether this starts with _
}

// HasErrors returns true if there are any error-level incompatibilities.
func (r *Result) HasErrors() bool {
	for _, inc := range r.Incompatibilities {
		if inc.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are warnings or incompatibilities.
func (r *Result) HasWarnings() bool {
	if len(r.Warnings) > 0 {
		return true
	}
	for _, inc := range r.Incompatibilities {
		if inc.Severity == SeverityWarning {
			return true
		}
	}
	return false
}

// WarningCount returns the number of warning-level issues.
func (r *Result) WarningCount() int {
	count := len(r.Warnings)
	for _, inc := range r.Incompatibilities {
		if inc.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

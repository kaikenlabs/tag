package scaffold

import (
	"github.com/kaikenlabs/tag/internal/tmplconfig"
)

// Type aliases for backward compatibility within the scaffold package.
// External packages should import tmplconfig directly.
type (
	TemplateConfig = tmplconfig.TemplateConfig
	VariableDef    = tmplconfig.VariableDef
	VariableType   = tmplconfig.VariableType
)

const (
	VarTypeString  = tmplconfig.VarTypeString
	VarTypeBoolean = tmplconfig.VarTypeBoolean
	VarTypeNumber  = tmplconfig.VarTypeNumber
	VarTypeChoice  = tmplconfig.VarTypeChoice
)

// ParseTemplateConfig parses a tag.template.json file from bytes.
var ParseTemplateConfig = tmplconfig.ParseTemplateConfig

// Options represents scaffold command options.
type Options struct {
	TemplateDir          string            // Path to template directory
	OutputDir            string            // Output directory (-o flag)
	ProjectName          string            // Project name argument
	ValuesFile           string            // Path to values JSON file (--values flag)
	Meta                 map[string]string // Individual variable overrides (-m/--meta flags)
	NoInput              bool              // Skip interactive prompts (--no-input flag)
	Force                bool              // Overwrite existing output (--force flag)
	Replay               bool              // Use saved replay values (--replay flag)
	NoSave               bool              // Don't save inputs for replay (--no-save flag)
	TemplateRef          string            // Original template reference (for replay ID generation)
	AcceptHooks          bool              // Accept hooks without prompting (--accept-hooks flag)
	IsRemote             bool              // Whether the template source is remote
	AllowRecursiveRender bool              // Allow recursive template rendering in variable values (--allow-recursive-render flag)
	IsTTY                bool              // Whether stdin is a TTY (set automatically if not provided)
	TemplateName         string            // Library name (set by tag scaffold)
	TemplateVersion      string            // From tag.template.json (set after config load)
}

package tmplconfig

import (
	"github.com/kaikenlabs/tag/internal/types"
)

// TemplateConfig represents the structure of tag.template.json.
type TemplateConfig struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	// Keywords lists topic keywords for discoverability in 'tag lib search'.
	// Mirrors the GitHub topic convention: all lowercase, hyphen-separated.
	Keywords []string `json:"keywords,omitempty"`
	// Categories groups this template into high-level buckets (e.g. "backend", "cli", "data").
	Categories []string               `json:"categories,omitempty"`
	Vars       map[string]VariableDef `json:"-"` // Custom unmarshaling needed
	RawVars    map[string]any         `json:"vars"`
	Hooks      *types.HooksConfig     `json:"hooks,omitempty"`
	// Requires lists template variable names that must be truthy in the project's
	// .tagconfig.json for this generator to run. Used for prerequisite checking.
	Requires []string `json:"requires,omitempty"`
}

// VariableType represents the type of a template variable.
type VariableType string

const (
	VarTypeString  VariableType = "string"
	VarTypeBoolean VariableType = "boolean"
	VarTypeNumber  VariableType = "number"
	VarTypeChoice  VariableType = "choice"
)

// VariableDef represents a variable definition in tag.template.json.
// Supports both short form (just a default value) and long form (full definition).
type VariableDef struct {
	Type     VariableType `json:"type,omitempty"`
	Prompt   string       `json:"prompt,omitempty"`
	Default  any          `json:"default,omitempty"`
	Required bool         `json:"required,omitempty"`
	Options  []string     `json:"options,omitempty"` // For choice type
	Secret   bool         `json:"secret,omitempty"`  // For password-style input
}

// IsPrivate returns true if the variable is a computed/private variable.
// Private variables start with an underscore and are not prompted.
func (v VariableDef) IsPrivate(name string) bool {
	return name != "" && name[0] == '_'
}

// IsDerived returns true if the variable's default is a template expression
// that references other variables, AND the variable has no explicit prompt.
// Derived variables are not prompted; their values are computed from other
// variables during rendering.
//
// Variables in expanded form with an explicit prompt are NOT derived — they
// are "evaluated defaults" and will be prompted interactively with the
// resolved expression as the suggested default (see IsEvaluatedDefault).
func (v VariableDef) IsDerived() bool {
	if v.Default == nil {
		return false
	}
	defaultStr, ok := v.Default.(string)
	if !ok {
		return false
	}
	if !ContainsTemplateExpression(defaultStr) {
		return false
	}
	// Variables with an explicit prompt use evaluated-default behavior instead.
	return v.Prompt == ""
}

// IsEvaluatedDefault returns true if the variable has an explicit prompt and
// a template-expression default. Such variables are prompted interactively
// with the resolved expression as the suggested default (rather than being
// silently derived). In non-TTY mode their expression is resolved the same
// way as derived variables.
func (v VariableDef) IsEvaluatedDefault() bool {
	if v.Prompt == "" {
		return false
	}
	if v.Default == nil {
		return false
	}
	defaultStr, ok := v.Default.(string)
	if !ok {
		return false
	}
	return ContainsTemplateExpression(defaultStr)
}

// GetPrompt returns the prompt message for the variable.
// If no prompt is set, returns a default prompt based on the variable name.
func (v VariableDef) GetPrompt(name string) string {
	if v.Prompt != "" {
		return v.Prompt
	}
	return "Enter value for " + name
}

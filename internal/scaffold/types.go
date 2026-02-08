package scaffold

import (
	"encoding/json"
	"fmt"
)

// TemplateConfig represents the structure of tag.template.json.
type TemplateConfig struct {
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Vars        map[string]VariableDef `json:"-"` // Custom unmarshaling needed
	RawVars     map[string]any         `json:"vars"`
	Hooks       *HooksConfig           `json:"hooks,omitempty"`
}

// HooksConfig defines pre and post scaffold hooks.
type HooksConfig struct {
	PreScaffold  []string `json:"pre_scaffold,omitempty"`
	PostScaffold []string `json:"post_scaffold,omitempty"`
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

// ParseTemplateConfig parses a tag.template.json file from bytes.
func ParseTemplateConfig(data []byte) (*TemplateConfig, error) {
	var config TemplateConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse template config: %w", err)
	}

	// Parse variables from RawVars into typed VariableDef
	config.Vars = make(map[string]VariableDef)
	for name, raw := range config.RawVars {
		varDef, err := parseVariableDef(name, raw)
		if err != nil {
			return nil, fmt.Errorf("invalid variable %q: %w", name, err)
		}
		config.Vars[name] = varDef
	}

	return &config, nil
}

// parseVariableDef parses a variable definition from its raw JSON form.
// Supports both short form (just a value) and long form (full object).
func parseVariableDef(name string, raw any) (VariableDef, error) {
	switch v := raw.(type) {
	case string:
		// Short form: "var_name": "default_value"
		return VariableDef{
			Type:    VarTypeString,
			Default: v,
		}, nil

	case bool:
		// Short form: "var_name": true/false
		return VariableDef{
			Type:    VarTypeBoolean,
			Default: v,
		}, nil

	case float64:
		// Short form: "var_name": 123 (JSON numbers are float64)
		return VariableDef{
			Type:    VarTypeNumber,
			Default: v,
		}, nil

	case map[string]any:
		// Long form: full object definition
		return parseLongFormVariable(name, v)

	default:
		return VariableDef{}, fmt.Errorf("unsupported variable format: %T", raw)
	}
}

// parseLongFormVariable parses a long-form variable definition object.
func parseLongFormVariable(name string, obj map[string]any) (VariableDef, error) {
	varDef := VariableDef{
		Type: VarTypeString, // Default type
	}

	// Parse type
	if t, ok := obj["type"].(string); ok {
		varDef.Type = VariableType(t)
	}

	// Parse prompt
	if p, ok := obj["prompt"].(string); ok {
		varDef.Prompt = p
	}

	// Parse default
	if d, exists := obj["default"]; exists {
		varDef.Default = d
	}

	// Parse required
	if r, ok := obj["required"].(bool); ok {
		varDef.Required = r
	}

	// Parse secret
	if s, ok := obj["secret"].(bool); ok {
		varDef.Secret = s
	}

	// Parse options (for choice type)
	if opts, ok := obj["options"].([]any); ok {
		varDef.Options = make([]string, 0, len(opts))
		for _, opt := range opts {
			if s, ok := opt.(string); ok {
				varDef.Options = append(varDef.Options, s)
			}
		}
	}

	// Validate choice type has options
	if varDef.Type == VarTypeChoice && len(varDef.Options) == 0 {
		return VariableDef{}, fmt.Errorf("choice variable %q must have options", name)
	}

	return varDef, nil
}

// IsPrivate returns true if the variable is a computed/private variable.
// Private variables start with an underscore and are not prompted.
func (v VariableDef) IsPrivate(name string) bool {
	return len(name) > 0 && name[0] == '_'
}

// IsDerived returns true if the variable's default is a template expression
// that references other variables. Derived variables are not prompted;
// their values are computed from other variables during rendering.
// This follows Cookiecutter's behavior for derived variables.
func (v VariableDef) IsDerived() bool {
	if v.Default == nil {
		return false
	}
	defaultStr, ok := v.Default.(string)
	if !ok {
		return false
	}
	// Check if default contains template expressions referencing variables
	// Supports both {{ vars.name }} and {{ cookiecutter.name }} syntax
	return containsTemplateExpression(defaultStr)
}

// containsTemplateExpression checks if a string contains Jinja2-style
// template expressions that reference variables.
func containsTemplateExpression(s string) bool {
	// Look for {{ vars. or {{ cookiecutter. patterns
	// Also handles whitespace variations like {{vars. or {{ cookiecutter.
	for i := 0; i < len(s)-2; i++ {
		if s[i] == '{' && s[i+1] == '{' {
			// Found opening braces, look for vars. or cookiecutter.
			rest := s[i+2:]
			// Skip whitespace
			j := 0
			for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t') {
				j++
			}
			if j < len(rest) {
				remaining := rest[j:]
				if len(remaining) >= 5 && remaining[:5] == "vars." {
					return true
				}
				// "cookiecutter." is 13 characters
				if len(remaining) >= 13 && remaining[:13] == "cookiecutter." {
					return true
				}
			}
		}
	}
	return false
}

// GetPrompt returns the prompt message for the variable.
// If no prompt is set, returns a default prompt based on the variable name.
func (v VariableDef) GetPrompt(name string) string {
	if v.Prompt != "" {
		return v.Prompt
	}
	return fmt.Sprintf("Enter value for %s", name)
}

// Options represents scaffold command options.
type Options struct {
	TemplateDir string            // Path to template directory
	OutputDir   string            // Output directory (-o flag)
	ProjectName string            // Project name argument
	ValuesFile  string            // Path to values JSON file (--values flag)
	Meta        map[string]string // Individual variable overrides (-m/--meta flags)
	NoInput     bool              // Skip interactive prompts (--no-input flag)
	Force       bool              // Overwrite existing output (--force flag)
	Replay      bool              // Use saved replay values (--replay flag)
	NoSave      bool              // Don't save inputs for replay (--no-save flag)
	TemplateRef string            // Original template reference (for replay ID generation)
	AcceptHooks bool              // Accept hooks without prompting (--accept-hooks flag)
	IsRemote    bool              // Whether the template source is remote
}

// CollectOptions contains options for variable collection.
// Most fields overlap with Options; use Options.CollectOpts() to convert.
type CollectOptions struct {
	ValuesFile  string            // Path to values JSON file
	Meta        map[string]string // CLI meta overrides
	NoPrompt    bool              // Skip interactive prompts
	IsTTY       bool              // Whether stdin is a TTY
	Replay      bool              // Load and use saved replay values
	TemplateRef string            // Original template reference (for replay ID lookup)
}

// CollectOpts builds a CollectOptions from the scaffold Options,
// reducing manual field-by-field copying at the call site.
func (o Options) CollectOpts() CollectOptions {
	return CollectOptions{
		ValuesFile:  o.ValuesFile,
		Meta:        o.Meta,
		NoPrompt:    o.NoInput,
		IsTTY:       IsTTY(),
		Replay:      o.Replay,
		TemplateRef: o.TemplateRef,
	}
}

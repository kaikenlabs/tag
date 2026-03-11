package tmplconfig

import (
	"encoding/json"
	"fmt"
	"strings"
)

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

// TestConfig represents the optional "test" section in tag.template.json.
type TestConfig struct {
	Commands    []string          `json:"commands,omitempty"`
	ProjectName string            `json:"project_name,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

// ParseTestConfig extracts the optional "test" section from raw template config JSON.
// Returns nil if no test section is present.
func ParseTestConfig(data []byte) (*TestConfig, error) {
	var raw struct {
		Test *TestConfig `json:"test"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse test config: %w", err)
	}
	return raw.Test, nil
}

// parseVariableDef parses a variable definition from its raw JSON form.
// Supports both short form (just a value) and long form (full object).
func parseVariableDef(name string, raw any) (VariableDef, error) {
	switch v := raw.(type) {
	case string:
		return VariableDef{
			Type:    VarTypeString,
			Default: v,
		}, nil

	case bool:
		return VariableDef{
			Type:    VarTypeBoolean,
			Default: v,
		}, nil

	case float64:
		return VariableDef{
			Type:    VarTypeNumber,
			Default: v,
		}, nil

	case map[string]any:
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

	if t, ok := obj["type"].(string); ok {
		varDef.Type = VariableType(t)
	}

	if p, ok := obj["prompt"].(string); ok {
		varDef.Prompt = p
	}

	if d, exists := obj["default"]; exists {
		varDef.Default = d
	}

	if r, ok := obj["required"].(bool); ok {
		varDef.Required = r
	}

	if s, ok := obj["secret"].(bool); ok {
		varDef.Secret = s
	}

	if opts, ok := obj["options"].([]any); ok {
		varDef.Options = make([]string, 0, len(opts))
		for _, opt := range opts {
			if s, ok := opt.(string); ok {
				varDef.Options = append(varDef.Options, s)
			}
		}
	}

	if varDef.Type == VarTypeChoice && len(varDef.Options) == 0 {
		return VariableDef{}, fmt.Errorf("choice variable %q must have options", name)
	}

	return varDef, nil
}

// ContainsTemplateExpression checks if a string contains Jinja2-style
// template expressions that reference variables.
func ContainsTemplateExpression(s string) bool {
	return strings.Contains(s, "{{") && strings.Contains(s, "vars.")
}

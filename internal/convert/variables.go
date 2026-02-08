package convert

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kaikenlabs/tag/internal/scaffold"
)

// ConvertCookiecutterConfig parses cookiecutter.json and converts it to TAG format.
// Returns the converted TemplateConfig, variable conversion details, and any warnings.
func ConvertCookiecutterConfig(data []byte) (*scaffold.TemplateConfig, []VariableConversion, []string, error) {
	var ccConfig map[string]any
	if err := json.Unmarshal(data, &ccConfig); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	config := &scaffold.TemplateConfig{
		RawVars: make(map[string]any),
	}
	var conversions []VariableConversion
	var warnings []string

	for name, value := range ccConfig {
		// Skip special cookiecutter keys
		if isSpecialCookiecutterKey(name) {
			warnings = append(warnings, fmt.Sprintf("special key '%s' is not converted (manual review may be needed)", name))
			continue
		}

		conv, tagValue, warn := convertVariable(name, value)
		if warn != "" {
			warnings = append(warnings, warn)
		}

		conversions = append(conversions, conv)
		config.RawVars[name] = tagValue
	}

	// Parse RawVars into typed Vars
	config.Vars = make(map[string]scaffold.VariableDef)
	for name, raw := range config.RawVars {
		varDef, err := parseConvertedVariable(name, raw)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("variable '%s': %v", name, err))
			continue
		}
		config.Vars[name] = varDef
	}

	return config, conversions, warnings, nil
}

// isSpecialCookiecutterKey checks if a key is a special cookiecutter configuration.
func isSpecialCookiecutterKey(key string) bool {
	specialKeys := map[string]bool{
		"_copy_without_render": true,
		"_skip_by_render":      true,
		"_extensions":          true,
		"_prompts":             true,
		"_jinja2_env_vars":     true,
	}
	return specialKeys[key]
}

// convertVariable converts a single cookiecutter variable to TAG format.
func convertVariable(name string, value any) (VariableConversion, any, string) {
	conv := VariableConversion{
		Name:      name,
		IsPrivate: strings.HasPrefix(name, "_"),
	}
	var warning string

	switch v := value.(type) {
	case string:
		conv.OriginalType = "string"
		conv.TagType = "string"
		conv.Default = v
		// Convert cookiecutter namespace to vars namespace in default values
		// This handles derived variables like: "{{cookiecutter.name.lower()}}"
		convertedValue, _ := ConvertPath(v)
		return conv, convertedValue, ""

	case bool:
		conv.OriginalType = "boolean"
		conv.TagType = "boolean"
		conv.Default = v
		return conv, map[string]any{
			"type":    "boolean",
			"default": v,
		}, ""

	case float64:
		conv.OriginalType = "number"
		conv.TagType = "number"
		conv.Default = v
		return conv, map[string]any{
			"type":    "number",
			"default": v,
		}, ""

	case []any:
		// Choice variable - array in cookiecutter.json
		conv.OriginalType = "choice"
		conv.TagType = "choice"
		conv.IsChoice = true

		options := make([]string, 0, len(v))
		for _, opt := range v {
			options = append(options, fmt.Sprintf("%v", opt))
		}
		conv.Default = options

		// First element is the default in cookiecutter
		defaultVal := ""
		if len(options) > 0 {
			defaultVal = options[0]
		}

		return conv, map[string]any{
			"type":    "choice",
			"options": options,
			"default": defaultVal,
		}, ""

	case map[string]any:
		// Nested object - not directly supported, convert to string with warning
		conv.OriginalType = "object"
		conv.TagType = "string"
		jsonBytes, _ := json.Marshal(v)
		conv.Default = string(jsonBytes)
		warning = fmt.Sprintf("variable '%s' is a nested object; converted to JSON string - manual review recommended", name)
		return conv, string(jsonBytes), warning

	case nil:
		// Null default
		conv.OriginalType = "null"
		conv.TagType = "string"
		conv.Default = ""
		warning = fmt.Sprintf("variable '%s' has null default; converted to empty string", name)
		return conv, "", warning

	default:
		// Unknown type
		conv.OriginalType = fmt.Sprintf("%T", value)
		conv.TagType = "string"
		conv.Default = fmt.Sprintf("%v", value)
		warning = fmt.Sprintf("variable '%s' has unknown type %T; converted to string", name, value)
		return conv, fmt.Sprintf("%v", value), warning
	}
}

// parseConvertedVariable parses a converted variable value into VariableDef.
func parseConvertedVariable(name string, raw any) (scaffold.VariableDef, error) {
	switch v := raw.(type) {
	case string:
		return scaffold.VariableDef{
			Type:    scaffold.VarTypeString,
			Default: v,
		}, nil

	case bool:
		return scaffold.VariableDef{
			Type:    scaffold.VarTypeBoolean,
			Default: v,
		}, nil

	case float64:
		return scaffold.VariableDef{
			Type:    scaffold.VarTypeNumber,
			Default: v,
		}, nil

	case map[string]any:
		varDef := scaffold.VariableDef{
			Type: scaffold.VarTypeString,
		}

		if t, ok := v["type"].(string); ok {
			varDef.Type = scaffold.VariableType(t)
		}
		if p, ok := v["prompt"].(string); ok {
			varDef.Prompt = p
		}
		if d, exists := v["default"]; exists {
			varDef.Default = d
		}
		if r, ok := v["required"].(bool); ok {
			varDef.Required = r
		}
		if opts, ok := v["options"].([]any); ok {
			varDef.Options = make([]string, 0, len(opts))
			for _, opt := range opts {
				varDef.Options = append(varDef.Options, fmt.Sprintf("%v", opt))
			}
		}
		if opts, ok := v["options"].([]string); ok {
			varDef.Options = opts
		}

		return varDef, nil

	default:
		return scaffold.VariableDef{}, fmt.Errorf("unsupported variable format: %T", raw)
	}
}

// GenerateTagTemplateJSON generates the tag.template.json content from config.
func GenerateTagTemplateJSON(config *scaffold.TemplateConfig, name, description string) ([]byte, error) {
	output := map[string]any{}

	if name != "" {
		output["name"] = name
	}
	if description != "" {
		output["description"] = description
	}
	if len(config.RawVars) > 0 {
		output["vars"] = config.RawVars
	}
	if config.Hooks != nil {
		hooks := map[string]any{}
		if len(config.Hooks.PreScaffold) > 0 {
			hooks["pre_scaffold"] = config.Hooks.PreScaffold
		}
		if len(config.Hooks.PostScaffold) > 0 {
			hooks["post_scaffold"] = config.Hooks.PostScaffold
		}
		if len(hooks) > 0 {
			output["hooks"] = hooks
		}
	}

	return json.MarshalIndent(output, "", "  ")
}

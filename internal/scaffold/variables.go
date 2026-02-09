package scaffold

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/kaikenlabs/tag/internal/replay"
)

// VariableCollector gathers variable values from all sources.
type VariableCollector interface {
	Collect(config *TemplateConfig, opts CollectOptions) (map[string]any, error)
}

// DefaultVariableCollector implements VariableCollector with the standard priority chain.
type DefaultVariableCollector struct {
	prompter Prompter
}

// NewVariableCollector creates a new variable collector with the given prompter.
func NewVariableCollector(prompter Prompter) *DefaultVariableCollector {
	return &DefaultVariableCollector{prompter: prompter}
}

// Collect gathers variables following the priority chain:
// defaults -> --replay -> --values file -> prompts -> --meta flags
//
// Each layer overwrites the previous, with --meta having highest priority.
//
//nolint:gocognit,cyclop // orchestration function coordinates multiple input sources
func (c *DefaultVariableCollector) Collect(config *TemplateConfig, opts CollectOptions) (map[string]any, error) {
	vars := make(map[string]any)
	// Track which variables were explicitly provided (from replay or values file)
	// These should not be re-prompted even in interactive mode
	explicitlyProvided := make(map[string]bool)

	// Get sorted variable names for deterministic ordering
	varNames := getSortedVarNames(config.Vars)

	// Step 1: Apply defaults (but don't mark them as explicitly provided)
	for _, name := range varNames {
		def := config.Vars[name]
		if def.Default != nil {
			vars[name] = def.Default
		}
	}

	// Step 2: Load replay values (if --replay flag is set)
	//nolint:nestif // replay variable resolution requires nested conditions
	if opts.Replay {
		if opts.TemplateRef == "" {
			return nil, errors.New("--replay requires template reference to be set")
		}
		replayData, err := replay.Load(opts.TemplateRef)
		if err != nil {
			if errors.Is(err, replay.ErrReplayNotFound) {
				return nil, errors.New("no saved replay data found for this template (use scaffold without --replay first)")
			}
			if errors.Is(err, replay.ErrReplayCorrupt) {
				return nil, fmt.Errorf("replay file is corrupt: %w (delete it and try again)", err)
			}
			return nil, fmt.Errorf("failed to load replay data: %w", err)
		}
		// Apply replay values
		for k, v := range replayData.Values {
			// Coerce replay values to expected types if we know them
			if def, ok := config.Vars[k]; ok {
				coerced, err := coerceAnyValue(v, def.Type)
				if err != nil {
					// If coercion fails, use the raw value (template may have changed)
					vars[k] = v
				} else {
					vars[k] = coerced
				}
			} else {
				// Unknown variable (template may have changed), store as-is
				vars[k] = v
			}
			explicitlyProvided[k] = true
		}
	}

	// Step 3: Load values from --values file (if provided)
	if opts.ValuesFile != "" {
		fileVars, err := loadValuesFile(opts.ValuesFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load values file: %w", err)
		}
		for k, v := range fileVars {
			vars[k] = v
			explicitlyProvided[k] = true
		}
	}

	// Step 4: Interactive prompts for variables (if TTY)
	// Prompt for all non-private, non-derived variables, even if they have defaults.
	// Skip only if the value was explicitly provided via replay or values file.
	if opts.IsTTY && !opts.NoPrompt {
		for _, name := range varNames {
			def := config.Vars[name]

			// Skip private/computed variables (start with _)
			if def.IsPrivate(name) {
				continue
			}

			// Skip derived variables (default contains template expression)
			// These are computed from other variables, following Cookiecutter behavior
			if def.IsDerived() {
				continue
			}

			// Skip if explicitly provided via replay or values file
			if explicitlyProvided[name] {
				continue
			}

			// Prompt for the variable (default will be pre-filled)
			value, err := c.promptForVariable(name, def)
			if err != nil {
				return nil, err
			}
			vars[name] = value
		}
	}

	// Step 5: Apply --meta overrides (highest priority)
	for k, v := range opts.Meta {
		// Try to coerce value to the expected type if we know it
		if def, ok := config.Vars[k]; ok {
			coerced, err := coerceValue(v, def.Type)
			if err != nil {
				return nil, NewVariableError(k, "invalid value type", err)
			}
			vars[k] = coerced
		} else {
			// Unknown variable, store as string
			vars[k] = v
		}
	}

	// Step 6: Validate all required variables have values
	if err := c.validateRequired(config, vars); err != nil {
		return nil, err
	}

	// Step 7: Process computed/private variables
	// These are variables starting with underscore that may reference other vars
	// For now, we skip them - they'll be processed during template rendering

	return vars, nil
}

// promptForVariable prompts the user for a variable value based on its type.
func (c *DefaultVariableCollector) promptForVariable(name string, def VariableDef) (any, error) {
	prompt := def.GetPrompt(name)

	switch def.Type {
	case VarTypeBoolean:
		defaultBool := false
		if def.Default != nil {
			if b, ok := def.Default.(bool); ok {
				defaultBool = b
			}
		}
		return c.prompter.Confirm(prompt, defaultBool)

	case VarTypeNumber:
		defaultNum := 0.0
		if def.Default != nil {
			switch v := def.Default.(type) {
			case float64:
				defaultNum = v
			case int:
				defaultNum = float64(v)
			}
		}
		return c.prompter.Number(prompt, defaultNum)

	case VarTypeChoice:
		defaultIndex := 0
		if def.Default != nil {
			if defaultStr, ok := def.Default.(string); ok {
				for i, opt := range def.Options {
					if opt == defaultStr {
						defaultIndex = i
						break
					}
				}
			}
		}
		return c.prompter.Select(prompt, def.Options, defaultIndex)

	default: // VarTypeString or unspecified
		defaultStr := ""
		if def.Default != nil {
			if s, ok := def.Default.(string); ok {
				defaultStr = s
			}
		}
		return c.prompter.Input(prompt, defaultStr, def.Secret)
	}
}

// validateRequired checks that all required variables have values.
func (c *DefaultVariableCollector) validateRequired(config *TemplateConfig, vars map[string]any) error {
	var missing []string

	for name, def := range config.Vars {
		if !def.Required {
			continue
		}

		// Skip private variables - they're computed
		if def.IsPrivate(name) {
			continue
		}

		val, exists := vars[name]
		if !exists || isEmptyValue(val) {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("%w: %s", ErrRequiredVariableMissing, strings.Join(missing, ", "))
	}

	return nil
}

// loadValuesFile loads variable values from a JSON file.
func loadValuesFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read values file: %w", err)
	}

	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("failed to parse values file: %w", err)
	}

	return values, nil
}

// coerceValue attempts to convert a string value to the expected type.
func coerceValue(value string, varType VariableType) (any, error) {
	switch varType {
	case VarTypeBoolean:
		return parseBool(value)

	case VarTypeNumber:
		return strconv.ParseFloat(value, 64)

	case VarTypeChoice, VarTypeString:
		return value, nil

	default:
		return value, nil
	}
}

// coerceAnyValue attempts to convert a value of any type to the expected type.
// This is used for replay values which are already typed from JSON.
func coerceAnyValue(value any, varType VariableType) (any, error) {
	if value == nil {
		return nil, nil //nolint:nilnil // nil value is not an error
	}

	switch varType {
	case VarTypeBoolean:
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			return parseBool(v)
		default:
			return nil, fmt.Errorf("cannot convert %T to boolean", value)
		}

	case VarTypeNumber:
		switch v := value.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case string:
			return strconv.ParseFloat(v, 64)
		default:
			return nil, fmt.Errorf("cannot convert %T to number", value)
		}

	case VarTypeChoice, VarTypeString:
		switch v := value.(type) {
		case string:
			return v, nil
		default:
			return fmt.Sprintf("%v", value), nil
		}

	default:
		return value, nil
	}
}

// parseBool parses a boolean from various string representations.
func parseBool(s string) (bool, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "yes", "y", "1", "on":
		return true, nil
	case "false", "no", "n", "0", "off", "":
		return false, nil
	default:
		return false, fmt.Errorf("cannot parse %q as boolean", s)
	}
}

// isEmptyValue checks if a value is considered "empty".
func isEmptyValue(val any) bool {
	if val == nil {
		return true
	}
	switch v := val.(type) {
	case string:
		return v == ""
	case bool:
		return false // booleans are never "empty"
	case float64:
		return false // numbers are never "empty"
	case int:
		return false
	default:
		return false
	}
}

// getSortedVarNames returns variable names in sorted order for deterministic processing.
func getSortedVarNames(vars map[string]VariableDef) []string {
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ParseMetaFlags parses a slice of "key=value" strings into a map.
func ParseMetaFlags(flags []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, flag := range flags {
		parts := strings.SplitN(flag, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid meta flag format: %q (expected key=value)", flag)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

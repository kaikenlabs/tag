package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/kaikenlabs/tag/internal/formats"
)

const (
	// MaxEnvValueLen is the maximum length for a single environment variable value.
	MaxEnvValueLen = 4096
)

// BuildHookEnv creates environment variables for hook execution.
// It merges the current environment with TAG-specific variables.
func BuildHookEnv(vars map[string]any, templateDir, outputDir string, w io.Writer) []string {
	// Start with current environment
	env := os.Environ()

	// Add TAG-specific variables
	env = append(env, "TAG_TEMPLATE_DIR="+templateDir, "TAG_OUTPUT_DIR="+outputDir)

	// Add project_name as a special variable
	if projectName, ok := vars["project_name"]; ok {
		env = append(env, "TAG_PROJECT_NAME="+stringifyValue(projectName))
	}

	// Add all user variables with TAG_VAR_ prefix (sanitized)
	for name, value := range vars {
		envKey := formatEnvKey(name)
		envValue := sanitizeEnvValue(name, stringifyValue(value), w)
		env = append(env, fmt.Sprintf("%s=%s", envKey, envValue))
	}

	return env
}

// BuildVarEnv creates environment variables containing only TAG_VAR_* entries
// and TAG_PROJECT_NAME. Unlike BuildHookEnv, it does not set TAG_TEMPLATE_DIR
// or TAG_OUTPUT_DIR, making it suitable for generate hooks where those paths
// are not applicable.
func BuildVarEnv(vars map[string]any, w io.Writer) []string {
	env := os.Environ()

	if projectName, ok := vars["project_name"]; ok {
		env = append(env, "TAG_PROJECT_NAME="+stringifyValue(projectName))
	}

	for name, value := range vars {
		envKey := formatEnvKey(name)
		envValue := sanitizeEnvValue(name, stringifyValue(value), w)
		env = append(env, fmt.Sprintf("%s=%s", envKey, envValue))
	}

	return env
}

// formatEnvKey converts a variable name to an environment variable key.
// Example: "project_name" -> "TAG_VAR_PROJECT_NAME"
// Example: "use-docker" -> "TAG_VAR_USE_DOCKER"
func formatEnvKey(name string) string {
	// Convert to uppercase and replace non-alphanumeric with underscores
	var result strings.Builder
	result.WriteString("TAG_VAR_")

	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(unicode.ToUpper(r))
		} else {
			result.WriteRune('_')
		}
	}

	return result.String()
}

// sanitizeEnvValue validates and sanitizes an environment variable value.
// It truncates overly long values and strips newlines to prevent header injection.
func sanitizeEnvValue(name, value string, w io.Writer) string {
	// Truncate overly long values
	if len(value) > MaxEnvValueLen {
		value = value[:MaxEnvValueLen]
		fmt.Fprintf(w, "Warning: variable %q value truncated to %d bytes for hook environment\n", name, MaxEnvValueLen)
	}

	// Strip null bytes, newlines, and carriage returns to prevent
	// environment variable injection and C-level string truncation.
	value = strings.ReplaceAll(value, "\x00", "")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")

	return value
}

// stringifyValue converts a variable value to a string for environment variables.
func stringifyValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		if formats.IsWholeNumber(v) {
			return strconv.FormatInt(int64(v), 10)
		}
		return fmt.Sprintf("%g", v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case []any, map[string]any:
		// JSON encode complex types
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	default:
		return fmt.Sprintf("%v", v)
	}
}

package schema

import (
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// Validator validates JSON documents against the embedded schema.
type Validator struct {
	schema *gojsonschema.Schema
}

// ValidationError represents a schema validation error with details.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 1 {
		return "validation error: " + e.Errors[0]
	}
	return "validation errors:\n  - " + strings.Join(e.Errors, "\n  - ")
}

// NewValidator creates a new schema validator using the embedded schema.
func NewValidator() (*Validator, error) {
	schemaLoader := gojsonschema.NewStringLoader(TemplateConfigSchema)
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded schema: %w", err)
	}

	return &Validator{schema: schema}, nil
}

// Validate validates JSON data against the tag.template.json schema.
// Returns nil if validation passes, or a ValidationError with details if it fails.
func (v *Validator) Validate(data []byte) error {
	documentLoader := gojsonschema.NewBytesLoader(data)
	result, err := v.schema.Validate(documentLoader)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if !result.Valid() {
		errs := make([]string, 0, len(result.Errors()))
		for _, desc := range result.Errors() {
			errs = append(errs, formatValidationError(desc))
		}
		return &ValidationError{Errors: errs}
	}

	return nil
}

// formatValidationError formats a single validation error for display.
func formatValidationError(err gojsonschema.ResultError) string {
	field := err.Field()
	if field == "(root)" {
		field = "root"
	}
	return fmt.Sprintf("%s: %s", field, err.Description())
}

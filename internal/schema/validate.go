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
		return fmt.Sprintf("validation error: %s", e.Errors[0])
	}
	return fmt.Sprintf("validation errors:\n  - %s", strings.Join(e.Errors, "\n  - "))
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

// MustNewValidator creates a new validator and panics on error.
// Use this only when you're certain the embedded schema is valid.
func MustNewValidator() *Validator {
	v, err := NewValidator()
	if err != nil {
		panic(fmt.Sprintf("failed to create schema validator: %v", err))
	}
	return v
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

// ValidateString validates a JSON string against the schema.
func (v *Validator) ValidateString(jsonStr string) error {
	return v.Validate([]byte(jsonStr))
}

// formatValidationError formats a single validation error for display.
func formatValidationError(err gojsonschema.ResultError) string {
	field := err.Field()
	if field == "(root)" {
		field = "root"
	}
	return fmt.Sprintf("%s: %s", field, err.Description())
}

// IsValidationError checks if an error is a ValidationError.
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

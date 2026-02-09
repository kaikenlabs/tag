package schema

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test-only helpers (moved from validate.go — only used by tests)
// =============================================================================

// MustNewValidator creates a new validator and panics on error.
// Use this only when you're certain the embedded schema is valid.
func MustNewValidator() *Validator {
	v, err := NewValidator()
	if err != nil {
		panic(fmt.Sprintf("failed to create schema validator: %v", err))
	}
	return v
}

// ValidateString validates a JSON string against the schema.
func (v *Validator) ValidateString(jsonStr string) error {
	return v.Validate([]byte(jsonStr))
}

// IsValidationError checks if an error is a ValidationError.
func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}

// =============================================================================
// Tests
// =============================================================================

func TestUT_SchemaValidator_ValidConfig(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "minimal valid config",
			json: `{}`,
		},
		{
			name: "config with name and description",
			json: `{"name": "my-template", "description": "A test template"}`,
		},
		{
			name: "config with version",
			json: `{"version": "1.0.0"}`,
		},
		{
			name: "config with short form variables",
			json: `{
				"vars": {
					"project_name": "my_project",
					"use_docker": true,
					"port": 8080
				}
			}`,
		},
		{
			name: "config with long form string variable",
			json: `{
				"vars": {
					"author": {
						"type": "string",
						"prompt": "Who is the author?",
						"default": "Your Name",
						"required": true
					}
				}
			}`,
		},
		{
			name: "config with choice variable",
			json: `{
				"vars": {
					"license": {
						"type": "choice",
						"prompt": "Select a license",
						"options": ["MIT", "BSD-3", "Apache-2.0"],
						"default": "MIT"
					}
				}
			}`,
		},
		{
			name: "config with boolean variable",
			json: `{
				"vars": {
					"use_docker": {
						"type": "boolean",
						"prompt": "Include Docker setup?",
						"default": false
					}
				}
			}`,
		},
		{
			name: "config with number variable",
			json: `{
				"vars": {
					"port": {
						"type": "number",
						"prompt": "Server port",
						"default": 8080
					}
				}
			}`,
		},
		{
			name: "config with hooks",
			json: `{
				"hooks": {
					"pre_scaffold": ["echo 'Starting'"],
					"post_scaffold": ["go mod tidy", "git init"]
				}
			}`,
		},
		{
			name: "full config",
			json: `{
				"name": "go-service",
				"description": "A Go microservice template",
				"version": "1.2.3",
				"vars": {
					"project_name": "my_project",
					"author": {
						"type": "string",
						"prompt": "Author name",
						"required": true
					},
					"license": {
						"type": "choice",
						"options": ["MIT", "Apache-2.0"]
					}
				},
				"hooks": {
					"post_scaffold": ["go mod tidy"]
				}
			}`,
		},
	}

	validator, err := NewValidator()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateString(tt.json)
			assert.NoError(t, err)
		})
	}
}

func TestUT_SchemaValidator_InvalidConfig(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		errContains string
	}{
		{
			name:        "invalid version format",
			json:        `{"version": "not-semver"}`,
			errContains: "version",
		},
		{
			name:        "invalid variable type",
			json:        `{"vars": {"foo": {"type": "invalid"}}}`,
			errContains: "type",
		},
		{
			name:        "extra top-level property",
			json:        `{"unknown_field": "value"}`,
			errContains: "unknown_field",
		},
		{
			name:        "invalid hooks type",
			json:        `{"hooks": "not an object"}`,
			errContains: "hooks",
		},
		{
			name:        "invalid hook command type",
			json:        `{"hooks": {"pre_scaffold": "not an array"}}`,
			errContains: "pre_scaffold",
		},
	}

	validator, err := NewValidator()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateString(tt.json)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestUT_SchemaValidator_InvalidJSON(t *testing.T) {
	validator, err := NewValidator()
	require.NoError(t, err)

	err = validator.ValidateString(`{invalid json}`)
	require.Error(t, err)
}

func TestUT_SchemaValidator_IsValidationError(t *testing.T) {
	validationErr := &ValidationError{Errors: []string{"test error"}}
	assert.True(t, IsValidationError(validationErr))

	genericErr := assert.AnError
	assert.False(t, IsValidationError(genericErr))
}

func TestUT_ValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   []string
		expected string
	}{
		{
			name:     "single error",
			errors:   []string{"field is required"},
			expected: "validation error: field is required",
		},
		{
			name:     "multiple errors",
			errors:   []string{"error one", "error two"},
			expected: "validation errors:\n  - error one\n  - error two",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ValidationError{Errors: tt.errors}
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestUT_MustNewValidator(t *testing.T) {
	// Should not panic
	validator := MustNewValidator()
	assert.NotNil(t, validator)
}

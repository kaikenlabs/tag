package template

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nikolalohinski/gonja/v2/exec"

	"github.com/kaikenlabs/tag/internal/dialect"
)

// openAPISchema represents an OpenAPI type mapping with optional format, nullable, and array wrapping.
type openAPISchema struct {
	Type     string
	Format   string
	Nullable bool
	Items    *openAPISchema
}

// yaml renders the schema as a zero-indented YAML fragment.
func (s openAPISchema) yaml() string {
	var lines []string

	lines = append(lines, "type: "+s.Type)
	if s.Format != "" {
		lines = append(lines, "format: "+s.Format)
	}
	if s.Nullable {
		lines = append(lines, "nullable: true")
	}
	if s.Items != nil {
		lines = append(lines, "items:")
		for line := range strings.SplitSeq(s.Items.yaml(), "\n") {
			lines = append(lines, "  "+line)
		}
	}

	return strings.Join(lines, "\n")
}

// resolveOpenAPIType maps a Go type string to an OpenAPI YAML fragment.
// Supports pointer (*T), slice ([]T), and common Go types.
func resolveOpenAPIType(goType string) (string, error) {
	schema, err := parseGoType(goType)
	if err != nil {
		return "", err
	}
	return schema.yaml(), nil
}

// parseGoType recursively parses a Go type string into an openAPISchema.
func parseGoType(s string) (openAPISchema, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return openAPISchema{}, errors.New("empty type string")
	}

	// Handle pointer prefix
	if rest, ok := strings.CutPrefix(s, "*"); ok {
		inner, err := parseGoType(rest)
		if err != nil {
			return openAPISchema{}, err
		}
		inner.Nullable = true
		return inner, nil
	}

	// Handle []byte specially before generic slice prefix — []byte is a single
	// OpenAPI type (string/byte), not an array of bytes.
	if s == "[]byte" {
		return openAPISchema{Type: "string", Format: "byte"}, nil
	}

	// Handle slice prefix
	if rest, ok := strings.CutPrefix(s, "[]"); ok {
		inner, err := parseGoType(rest)
		if err != nil {
			return openAPISchema{}, err
		}
		return openAPISchema{Type: "array", Items: &inner}, nil
	}

	// Base type mapping
	switch s {
	case "string":
		return openAPISchema{Type: "string"}, nil
	case "int", "int64":
		return openAPISchema{Type: "integer", Format: "int64"}, nil
	case "int32":
		return openAPISchema{Type: "integer", Format: "int32"}, nil
	case "int8", "int16":
		return openAPISchema{Type: "integer", Format: "int32"}, nil
	case "bool":
		return openAPISchema{Type: "boolean"}, nil
	case "float64":
		return openAPISchema{Type: "number", Format: "double"}, nil
	case "float32":
		return openAPISchema{Type: "number", Format: "float"}, nil
	case "time.Time":
		return openAPISchema{Type: "string", Format: "date-time"}, nil
	case "uuid.UUID":
		return openAPISchema{Type: "string", Format: "uuid"}, nil
	case "byte":
		return openAPISchema{Type: "string", Format: "byte"}, nil
	default:
		return openAPISchema{}, fmt.Errorf("unsupported Go type: %s", s)
	}
}

// RegisterToFilter registers a unified "to" filter that handles the "openapi" dialect
// natively and delegates all other dialect names to the provided registry (if non-nil).
func RegisterToFilter(filters *exec.FilterSet, reg *dialect.Registry) error {
	toFilter := func(_ *exec.Evaluator, in *exec.Value, params *exec.VarArgs) *exec.Value {
		if in.IsError() {
			return in
		}

		args := params.Args
		if len(args) != 1 {
			return exec.AsValue(fmt.Errorf("to: expected 1 argument (dialect name), got %d", len(args)))
		}

		dialectName := args[0].String()
		canonicalType := in.String()

		// Handle "openapi" natively
		if dialectName == "openapi" {
			result, err := resolveOpenAPIType(canonicalType)
			if err != nil {
				return exec.AsValue(fmt.Errorf("to: %w", err))
			}
			return exec.AsValue(result)
		}

		// Delegate to dialect registry for all other dialects
		if reg != nil {
			result, err := reg.Resolve(canonicalType, dialectName)
			if err != nil {
				return exec.AsValue(fmt.Errorf("to: %w", err))
			}
			return exec.AsValue(result)
		}

		return exec.AsValue(fmt.Errorf("to: unknown dialect %q (no dialect registry configured)", dialectName))
	}

	return registerFilter(filters, "to", toFilter)
}

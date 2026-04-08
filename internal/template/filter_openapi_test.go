package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ResolveOpenAPIType(t *testing.T) {
	tests := []struct {
		name    string
		goType  string
		want    string
		wantErr string
	}{
		// Primitive types
		{
			name:   "string",
			goType: "string",
			want:   "type: string",
		},
		{
			name:   "int",
			goType: "int",
			want:   "type: integer\nformat: int64",
		},
		{
			name:   "int64",
			goType: "int64",
			want:   "type: integer\nformat: int64",
		},
		{
			name:   "int32",
			goType: "int32",
			want:   "type: integer\nformat: int32",
		},
		{
			name:   "bool",
			goType: "bool",
			want:   "type: boolean",
		},
		{
			name:   "float64",
			goType: "float64",
			want:   "type: number\nformat: double",
		},
		{
			name:   "float32",
			goType: "float32",
			want:   "type: number\nformat: float",
		},

		// Qualified types
		{
			name:   "time.Time",
			goType: "time.Time",
			want:   "type: string\nformat: date-time",
		},
		{
			name:   "uuid.UUID",
			goType: "uuid.UUID",
			want:   "type: string\nformat: uuid",
		},

		// Pointer types (nullable)
		{
			name:   "pointer string",
			goType: "*string",
			want:   "type: string\nnullable: true",
		},
		{
			name:   "pointer int",
			goType: "*int",
			want:   "type: integer\nformat: int64\nnullable: true",
		},
		{
			name:   "pointer time.Time",
			goType: "*time.Time",
			want:   "type: string\nformat: date-time\nnullable: true",
		},
		{
			name:   "pointer uuid.UUID",
			goType: "*uuid.UUID",
			want:   "type: string\nformat: uuid\nnullable: true",
		},

		// Slice types (arrays)
		{
			name:   "slice of string",
			goType: "[]string",
			want:   "type: array\nitems:\n  type: string",
		},
		{
			name:   "slice of int",
			goType: "[]int",
			want:   "type: array\nitems:\n  type: integer\n  format: int64",
		},
		{
			name:   "slice of time.Time",
			goType: "[]time.Time",
			want:   "type: array\nitems:\n  type: string\n  format: date-time",
		},

		// Slice of pointer types
		{
			name:   "slice of pointer int",
			goType: "[]*int",
			want:   "type: array\nitems:\n  type: integer\n  format: int64\n  nullable: true",
		},
		{
			name:   "slice of pointer string",
			goType: "[]*string",
			want:   "type: array\nitems:\n  type: string\n  nullable: true",
		},

		// Byte types
		{
			name:   "byte",
			goType: "byte",
			want:   "type: string\nformat: byte",
		},
		{
			name:   "byte slice",
			goType: "[]byte",
			want:   "type: string\nformat: byte",
		},

		// Edge cases
		{
			name:   "whitespace trimmed",
			goType: "  string  ",
			want:   "type: string",
		},

		// Error cases
		{
			name:    "unknown type",
			goType:  "CustomStruct",
			wantErr: "unsupported Go type: CustomStruct",
		},
		{
			name:    "empty string",
			goType:  "",
			wantErr: "empty type string",
		},
		{
			name:    "map type unsupported",
			goType:  "map[string]string",
			wantErr: "unsupported Go type",
		},
		{
			name:    "interface unsupported",
			goType:  "interface{}",
			wantErr: "unsupported Go type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveOpenAPIType(tt.goType)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestUT_RegisterToFilter_OpenAPI(t *testing.T) {
	// Test that the filter works through the template engine
	engine, err := NewEngine()
	require.NoError(t, err)

	// Register the unified to() filter (no dialect registry, openapi only)
	err = RegisterToFilter(engine.env.Filters, nil)
	require.NoError(t, err)

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "simple string type",
			template: `{{ "string" | to("openapi") }}`,
			want:     "type: string",
		},
		{
			name:     "int type with format",
			template: `{{ "int" | to("openapi") }}`,
			want:     "type: integer\nformat: int64",
		},
		{
			name:     "pointer type nullable",
			template: `{{ "*bool" | to("openapi") }}`,
			want:     "type: boolean\nnullable: true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.ExecuteToString(tt.template, Context{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestUT_RegisterToFilter_UnknownDialect(t *testing.T) {
	engine, err := NewEngine()
	require.NoError(t, err)

	err = RegisterToFilter(engine.env.Filters, nil)
	require.NoError(t, err)

	// Using an unknown dialect without a registry should error
	_, err = engine.ExecuteToString(`{{ "string" | to("postgres") }}`, Context{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown dialect")
}

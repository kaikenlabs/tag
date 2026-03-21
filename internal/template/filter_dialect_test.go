package template

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/dialect"
)

func newTestRegistry(t *testing.T) *dialect.Registry {
	t.Helper()
	reg, err := dialect.LoadDefaults()
	require.NoError(t, err)
	return reg
}

func TestUT_FilterTo_BasicResolution(t *testing.T) {
	reg := newTestRegistry(t)
	engine, err := NewEngine(WithDialectRegistry(reg))
	require.NoError(t, err)

	tests := []struct {
		template string
		expected string
	}{
		{`{{ "string" | to("go") }}`, "string"},
		{`{{ "uuid" | to("postgres") }}`, "UUID"},
		{`{{ "datetime" | to("go") }}`, "time.Time"},
		{`{{ "bool" | to("typescript") }}`, "boolean"},
		{`{{ "int" | to("openapi") }}`, "integer"},
		{`{{ "datetime" | to("protobuf") }}`, "google.protobuf.Timestamp"},
		{`{{ "string" | to("mysql") }}`, "VARCHAR(255)"},
	}

	for _, tc := range tests {
		t.Run(tc.template, func(t *testing.T) {
			result, err := engine.ExecuteToString(tc.template, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestUT_FilterTo_UnknownType(t *testing.T) {
	reg := newTestRegistry(t)
	engine, err := NewEngine(WithDialectRegistry(reg))
	require.NoError(t, err)

	_, err = engine.ExecuteToString(`{{ "unknown_type" | to("go") }}`, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmapped canonical type")
}

func TestUT_FilterTo_UnknownDialect(t *testing.T) {
	reg := newTestRegistry(t)
	engine, err := NewEngine(WithDialectRegistry(reg))
	require.NoError(t, err)

	_, err = engine.ExecuteToString(`{{ "string" | to("nonexistent") }}`, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown dialect")
}

func TestUT_FilterTo_ChainWithOther(t *testing.T) {
	reg := newTestRegistry(t)
	engine, err := NewEngine(WithDialectRegistry(reg))
	require.NoError(t, err)

	result, err := engine.ExecuteToString(`{{ "string" | to("go") | upper }}`, nil)
	require.NoError(t, err)
	assert.Equal(t, "STRING", result)
}

func TestUT_FilterTo_MissingArg(t *testing.T) {
	reg := newTestRegistry(t)
	engine, err := NewEngine(WithDialectRegistry(reg))
	require.NoError(t, err)

	_, err = engine.ExecuteToString(`{{ "string" | to() }}`, nil)
	require.Error(t, err)
}

func TestUT_FilterTo_Isolation(t *testing.T) {
	// Engine 1: with full defaults
	reg1 := newTestRegistry(t)
	engine1, err := NewEngine(WithDialectRegistry(reg1))
	require.NoError(t, err)

	// Engine 2: with custom registry that overrides Go string
	reg2 := dialect.NewRegistry()
	require.NoError(t, reg2.Load([]byte(`
name: go
types:
  string: CustomString
`)))
	engine2, err := NewEngine(WithDialectRegistry(reg2))
	require.NoError(t, err)

	// Engine 1 should use built-in Go mapping
	result1, err := engine1.ExecuteToString(`{{ "string" | to("go") }}`, nil)
	require.NoError(t, err)
	assert.Equal(t, "string", result1)

	// Engine 2 should use custom mapping
	result2, err := engine2.ExecuteToString(`{{ "string" | to("go") }}`, nil)
	require.NoError(t, err)
	assert.Equal(t, "CustomString", result2)

	// Verify engine 1 is still unaffected
	result1Again, err := engine1.ExecuteToString(`{{ "string" | to("go") }}`, nil)
	require.NoError(t, err)
	assert.Equal(t, "string", result1Again)
}

func TestUT_FilterTo_NoRegistry(t *testing.T) {
	// Engine without dialect registry should not have to() filter
	engine, err := NewEngine()
	require.NoError(t, err)

	_, err = engine.ExecuteToString(`{{ "string" | to("go") }}`, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "to") || strings.Contains(err.Error(), "filter"),
		"error should indicate missing filter, got: %s", err.Error())
}

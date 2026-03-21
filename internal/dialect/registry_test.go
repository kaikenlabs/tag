package dialect

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_NewRegistry_Empty(t *testing.T) {
	reg := NewRegistry()

	assert.Empty(t, reg.Names())
	assert.Nil(t, reg.Get("go"))
}

func TestUT_Load_SingleDialect(t *testing.T) {
	reg := NewRegistry()
	err := reg.Load([]byte(`
name: test
description: Test dialect
types:
  string: STRING
  int: INT
`))

	require.NoError(t, err)
	assert.Equal(t, []string{"test"}, reg.Names())

	d := reg.Get("test")
	require.NotNil(t, d)
	assert.Equal(t, "Test dialect", d.Description)
	assert.Equal(t, "STRING", d.Types["string"])
	assert.Equal(t, "INT", d.Types["int"])
}

func TestUT_Load_InvalidYAML(t *testing.T) {
	reg := NewRegistry()
	err := reg.Load([]byte(`{invalid yaml`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse dialect YAML")
}

func TestUT_Load_MissingName(t *testing.T) {
	reg := NewRegistry()
	err := reg.Load([]byte(`
description: Missing name
types:
  string: STRING
`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required 'name' field")
}

func TestUT_Load_DeepMerge(t *testing.T) {
	reg := NewRegistry()

	// Load base dialect.
	require.NoError(t, reg.Load([]byte(`
name: go
description: Go base
types:
  string: string
  int: int
`)))

	// Load override — adds uuid, keeps string, overrides int.
	require.NoError(t, reg.Load([]byte(`
name: go
description: Go override
types:
  int: int64
  uuid: string
`)))

	d := reg.Get("go")
	require.NotNil(t, d)
	assert.Equal(t, "Go override", d.Description, "description should be overridden")
	assert.Equal(t, "string", d.Types["string"], "untouched type should remain")
	assert.Equal(t, "int64", d.Types["int"], "overridden type should be updated")
	assert.Equal(t, "string", d.Types["uuid"], "new type should be added")
}

func TestUT_LoadFS_BuiltinDialects(t *testing.T) {
	reg := NewRegistry()
	err := reg.LoadFS(builtinFS, builtinDir)
	require.NoError(t, err)

	names := reg.Names()
	assert.Equal(t, []string{"go", "mysql", "openapi", "postgres", "protobuf", "typescript"}, names)
}

func TestUT_LoadDir_ValidDir(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte(`
name: custom
description: Custom dialect
types:
  string: CustomString
`), 0o644))

	reg := NewRegistry()
	err := reg.LoadDir(dir)
	require.NoError(t, err)

	d := reg.Get("custom")
	require.NotNil(t, d)
	assert.Equal(t, "CustomString", d.Types["string"])
}

func TestUT_LoadDir_MissingDir(t *testing.T) {
	reg := NewRegistry()
	err := reg.LoadDir("/nonexistent/path/that/should/not/exist")
	assert.NoError(t, err, "missing directory should not produce an error")
	assert.Empty(t, reg.Names())
}

func TestUT_LoadDir_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not yaml"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "valid.yaml"), []byte(`
name: valid
types:
  string: S
`), 0o644))

	reg := NewRegistry()
	require.NoError(t, reg.LoadDir(dir))

	assert.Equal(t, []string{"valid"}, reg.Names())
}

func TestUT_LoadDir_DeterministicOrder(t *testing.T) {
	dir := t.TempDir()

	// Write files in reverse order; LoadDir should process alphabetically.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "z_dialect.yaml"), []byte(`
name: shared
types:
  string: Z
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a_dialect.yaml"), []byte(`
name: shared
types:
  string: A
`), 0o644))

	reg := NewRegistry()
	require.NoError(t, reg.LoadDir(dir))

	d := reg.Get("shared")
	require.NotNil(t, d)
	assert.Equal(t, "Z", d.Types["string"], "z_dialect.yaml (loaded last) should win")
}

func TestUT_LoadDefaults(t *testing.T) {
	reg, err := LoadDefaults()
	require.NoError(t, err)

	names := reg.Names()
	assert.Len(t, names, 6)
	assert.Contains(t, names, "go")
	assert.Contains(t, names, "postgres")
}

func TestUT_LoadWithOverrides_ThreeTier(t *testing.T) {
	userDir := t.TempDir()
	templateDir := t.TempDir()

	// User-global override: change Go string mapping.
	require.NoError(t, os.WriteFile(filepath.Join(userDir, "go.yaml"), []byte(`
name: go
types:
  string: MyString
`), 0o644))

	// Template-local override: change Go int mapping.
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "go.yaml"), []byte(`
name: go
types:
  int: MyInt
`), 0o644))

	reg, err := LoadWithOverrides(userDir, templateDir)
	require.NoError(t, err)

	goDialect := reg.Get("go")
	require.NotNil(t, goDialect)
	assert.Equal(t, "MyString", goDialect.Types["string"], "user-global override should apply")
	assert.Equal(t, "MyInt", goDialect.Types["int"], "template-local override should apply")
	assert.Equal(t, "time.Time", goDialect.Types["datetime"], "unoverridden built-in should remain")
}

func TestUT_LoadWithOverrides_EmptyDirs(t *testing.T) {
	reg, err := LoadWithOverrides("", "")
	require.NoError(t, err)
	assert.Len(t, reg.Names(), 6, "empty dirs should load only built-ins")
}

func TestUT_Resolve_AllBuiltinDialects(t *testing.T) {
	reg, err := LoadDefaults()
	require.NoError(t, err)

	tests := []struct {
		canonical string
		dialect   string
		expected  string
	}{
		// Go
		{"string", "go", "string"},
		{"uuid", "go", "string"},
		{"datetime", "go", "time.Time"},
		{"bytes", "go", "[]byte"},
		{"json", "go", "json.RawMessage"},
		{"decimal", "go", "decimal.Decimal"},
		// Postgres
		{"string", "postgres", "TEXT"},
		{"uuid", "postgres", "UUID"},
		{"datetime", "postgres", "TIMESTAMPTZ"},
		{"int64", "postgres", "BIGINT"},
		{"json", "postgres", "JSONB"},
		// MySQL
		{"string", "mysql", "VARCHAR(255)"},
		{"bool", "mysql", "TINYINT(1)"},
		{"uuid", "mysql", "CHAR(36)"},
		// TypeScript
		{"int", "typescript", "number"},
		{"bool", "typescript", "boolean"},
		{"bytes", "typescript", "Uint8Array"},
		{"json", "typescript", "unknown"},
		// OpenAPI
		{"int", "openapi", "integer"},
		{"float", "openapi", "number"},
		{"bool", "openapi", "boolean"},
		{"json", "openapi", "object"},
		// Protobuf
		{"int", "protobuf", "int32"},
		{"datetime", "protobuf", "google.protobuf.Timestamp"},
		{"json", "protobuf", "google.protobuf.Struct"},
	}

	for _, tc := range tests {
		t.Run(tc.dialect+"_"+tc.canonical, func(t *testing.T) {
			result, err := reg.Resolve(tc.canonical, tc.dialect)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestUT_Resolve_UnknownDialect(t *testing.T) {
	reg, err := LoadDefaults()
	require.NoError(t, err)

	_, err = reg.Resolve("string", "nonexistent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownDialect))
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "available:")
}

func TestUT_Resolve_UnmappedType(t *testing.T) {
	reg, err := LoadDefaults()
	require.NoError(t, err)

	_, err = reg.Resolve("unknown_type", "go")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnmappedType))
	assert.Contains(t, err.Error(), "unknown_type")
	assert.Contains(t, err.Error(), "available:")
}

func TestUT_Names_Sorted(t *testing.T) {
	reg := NewRegistry()

	require.NoError(t, reg.Load([]byte(`name: zebra
types:
  string: Z`)))
	require.NoError(t, reg.Load([]byte(`name: alpha
types:
  string: A`)))
	require.NoError(t, reg.Load([]byte(`name: middle
types:
  string: M`)))

	assert.Equal(t, []string{"alpha", "middle", "zebra"}, reg.Names())
}

func TestUT_Get_ExistingDialect(t *testing.T) {
	reg, err := LoadDefaults()
	require.NoError(t, err)

	d := reg.Get("go")
	require.NotNil(t, d)
	assert.Equal(t, "go", d.Name)
	assert.NotEmpty(t, d.Types)
}

func TestUT_Get_UnknownDialect(t *testing.T) {
	reg := NewRegistry()
	assert.Nil(t, reg.Get("nonexistent"))
}

func TestUT_BuiltinDialects_Complete(t *testing.T) {
	reg, err := LoadDefaults()
	require.NoError(t, err)

	canonicalTypes := []string{
		"string", "text", "int", "int32", "int64",
		"float", "float32", "float64", "bool", "byte",
		"bytes", "uuid", "datetime", "date", "decimal", "json",
	}

	for _, name := range reg.Names() {
		d := reg.Get(name)
		require.NotNil(t, d, "dialect %s should exist", name)

		for _, ct := range canonicalTypes {
			_, ok := d.Types[ct]
			assert.True(t, ok, "dialect %s is missing canonical type %q", name, ct)
		}
	}
}

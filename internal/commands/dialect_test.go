package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/dialect"
)

func TestUT_PrintDialectList_ShowsAllBuiltins(t *testing.T) {
	reg, err := dialect.LoadDefaults()
	require.NoError(t, err)

	var buf bytes.Buffer
	printDialectList(&buf, reg)

	output := buf.String()
	assert.Contains(t, output, "go")
	assert.Contains(t, output, "postgres")
	assert.Contains(t, output, "mysql")
	assert.Contains(t, output, "typescript")
	assert.Contains(t, output, "openapi")
	assert.Contains(t, output, "protobuf")
	assert.Contains(t, output, "built-in")
}

func TestUT_PrintDialectShow_ValidDialect(t *testing.T) {
	reg, err := dialect.LoadDefaults()
	require.NoError(t, err)

	d := reg.Get("postgres")
	require.NotNil(t, d)

	var buf bytes.Buffer
	printDialectShow(&buf, d)

	output := buf.String()
	assert.Contains(t, output, "Dialect: postgres")
	assert.Contains(t, output, "UUID")
	assert.Contains(t, output, "TIMESTAMPTZ")
	assert.Contains(t, output, "BIGINT")
	assert.Contains(t, output, "CANONICAL")
	assert.Contains(t, output, "TARGET")
}

func TestUT_PrintDialectShow_SortedOutput(t *testing.T) {
	d := &dialect.Dialect{
		Name:        "test",
		Description: "Test dialect",
		Types: map[string]string{
			"zebra": "Z",
			"alpha": "A",
		},
	}

	var buf bytes.Buffer
	printDialectShow(&buf, d)

	output := buf.String()
	// "alpha" should appear before "zebra"
	alphaIdx := strings.Index(output, "alpha")
	zebraIdx := strings.Index(output, "zebra")
	assert.Less(t, alphaIdx, zebraIdx, "types should be sorted alphabetically")
}

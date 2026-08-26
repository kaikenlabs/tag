package commands

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/types"
)

func TestUT_CompleteGeneratorNames_ScansBothLibraryRoots(t *testing.T) {
	setupLibEntryRoots(t, "roots-t7", map[string]string{
		filepath.Join(types.TemplatesDir, "dupgen", "gen.go"):          "package main\n",
		filepath.Join(types.GeneratorsDir, "dupgen", "gen.go"):         "package main\n",
		filepath.Join(types.TemplatesDir, "onlyTag", "gen.go"):         "package main\n",
		filepath.Join(types.GeneratorsDir, "onlyGenerators", "gen.go"): "package main\n",
	})

	cfg := createTestConfigWithLib(t, t.TempDir(), "roots-t7")

	var buf bytes.Buffer
	completeGeneratorNames(cfg, &buf)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 3, "each name appears exactly once: %v", lines)
	assert.Contains(t, lines, "dupgen")
	assert.Contains(t, lines, "onlyTag")
	assert.Contains(t, lines, "onlyGenerators")
}

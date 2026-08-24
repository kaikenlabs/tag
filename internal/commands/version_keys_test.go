package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// versionKeyedDocuments are the only two JSON documents allowed to carry
// schema_version / tag_version. Epic #388 scopes the keys to these two
// deliberately; extending them to the other 20 commands is a separate, larger
// decision, not something a later story should do by accident.
var versionKeyedDocuments = map[string]bool{
	"info.go":     true,
	"scaffold.go": true,
}

// TestUT_VersionKeys_ScopedToTwoDocuments proves the negative half of #395's
// acceptance criteria for ALL other commands at once.
//
// A behavioural probe cannot do this cheaply: several commands need a seeded
// project or template fixture, and four build their payload as a
// map[string]any, so no struct-tag scan alone would see a key added there. This
// scans the source for both forms — the struct tag and a map literal key —
// which covers every emitter in the package regardless of how it is built.
func TestUT_VersionKeys_ScopedToTwoDocuments(t *testing.T) {
	t.Parallel()

	needles := []string{
		`json:"schema_version"`,
		`json:"tag_version"`,
		`"schema_version":`,
		`"tag_version":`,
	}

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	// Counted per file and asserted below. Without this the whole test is
	// vacuous in the direction that matters most: the loop only asserts when a
	// match is found, so deleting both keys from both documents would leave it
	// green while the contract silently disappeared.
	found := map[string]int{}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		data, readErr := os.ReadFile(filepath.Clean(name))
		require.NoError(t, readErr)
		src := string(data)

		for _, needle := range needles {
			if !strings.Contains(src, needle) {
				continue
			}
			found[name]++
			assert.True(t, versionKeyedDocuments[name],
				"%s emits %s, but only `template info` and `scaffold` may carry version keys "+
					"(epic #388). If this is intentional, it is a contract decision: update "+
					"versionKeyedDocuments and docs/reference/json-contract.md.", name, needle)
		}
	}

	for name := range versionKeyedDocuments {
		assert.Positive(t, found[name],
			"%s must still emit schema_version/tag_version — epic #388 requires both keys on "+
				"this document, and removing them is a breaking contract change, not a cleanup", name)
	}
}

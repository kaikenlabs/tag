package replay

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_FilterSecrets_NoSecrets(t *testing.T) {
	t.Parallel()
	values := map[string]any{
		"name": "project",
		"port": 8080,
	}

	result := FilterSecrets(values, nil)
	assert.Len(t, result, 2)
	assert.Equal(t, "project", result["name"])
	assert.Equal(t, 8080, result["port"])
}

func TestUT_FilterSecrets_WithSecrets(t *testing.T) {
	t.Parallel()
	values := map[string]any{
		"name":    "project",
		"api_key": "secret123",
		"port":    8080,
	}
	secrets := map[string]bool{
		"api_key": true,
	}

	result := FilterSecrets(values, secrets)
	assert.Len(t, result, 2)
	assert.Equal(t, "project", result["name"])
	assert.Equal(t, 8080, result["port"])
	assert.NotContains(t, result, "api_key")
}

func TestUT_FilterSecrets_AllSecrets(t *testing.T) {
	t.Parallel()
	values := map[string]any{
		"api_key":  "secret1",
		"password": "secret2",
	}
	secrets := map[string]bool{
		"api_key":  true,
		"password": true,
	}

	result := FilterSecrets(values, secrets)
	assert.Empty(t, result)
}

func TestUT_FilterSecrets_EmptyValues(t *testing.T) {
	t.Parallel()
	result := FilterSecrets(map[string]any{}, nil)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestUT_FilterSecrets_ReturnsCopy(t *testing.T) {
	t.Parallel()
	original := map[string]any{"key": "value"}
	result := FilterSecrets(original, nil)

	// Modifying result should not affect original
	result["new_key"] = "new_value"
	assert.NotContains(t, original, "new_key")
}

func TestUT_Save_WhitespaceSource(t *testing.T) {
	err := Save("   ", "", nil, nil)
	assert.ErrorIs(t, err, ErrEmptyTemplateSource)
}

func TestUT_Save_ConcurrentSameTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvReplayDir, dir)

	const numWriters = 4
	const rounds = 5

	payload := make(map[string]any, 200)
	for i := range 200 {
		payload[fmt.Sprintf("key_%03d", i)] = strings.Repeat("x", 20)
	}

	for round := range rounds {
		var wg sync.WaitGroup
		errs := make([]error, numWriters)
		versions := make([]string, numWriters)
		for w := range numWriters {
			version := fmt.Sprintf("v%d.%d", round, w)
			versions[w] = version
			wg.Go(func() {
				errs[w] = Save("same-template-source", version, payload, nil)
			})
		}
		wg.Wait()

		for _, err := range errs {
			require.NoError(t, err)
		}

		templateID := GenerateTemplateID("same-template-source")
		filePath := filepath.Join(dir, templateID+".json")
		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var parsed ReplayData
		require.NoError(t, json.Unmarshal(data, &parsed), "round %d: final replay file must be valid JSON", round)

		assert.Equal(t, "same-template-source", parsed.Template, "round %d", round)
		assert.Contains(t, versions, parsed.Version, "round %d: surviving version must be exactly one writer's, not a mix", round)
		assert.Equal(t, payload, parsed.Values, "round %d: surviving values must exactly match one writer's payload, not a mix", round)
	}
}

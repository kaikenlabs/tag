package replay

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

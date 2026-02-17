package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_ValidateNameSafe_ValidNames(t *testing.T) {
	validNames := []string{
		"myGenerator",
		"my-generator",
		"my_generator",
		"MyGenerator123",
		".hidden-name",
		"name.with.dots",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			err := ValidateNameSafe(name)
			assert.NoError(t, err)
		})
	}
}

func TestUT_ValidateNameSafe_RejectsPathTraversal(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"dot dot", ".."},
		{"dot dot prefix", "../evil"},
		{"dot dot in middle", "a/../b"},
		{"dot dot suffix", "evil/.."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNameSafe(tt.input)
			require.Error(t, err)
		})
	}
}

func TestUT_ValidateNameSafe_RejectsForwardSlash(t *testing.T) {
	err := ValidateNameSafe("a/b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separator")
}

func TestUT_ValidateNameSafe_RejectsBackslash(t *testing.T) {
	err := ValidateNameSafe(`a\b`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separator")
}

func TestUT_ValidateNameSafe_RejectsSingleDot(t *testing.T) {
	err := ValidateNameSafe(".")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved name")
}

func TestUT_ValidateNameSafe_RejectsEmptyName(t *testing.T) {
	err := ValidateNameSafe("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestUT_ValidateNameSafe_RejectsLeadingSlash(t *testing.T) {
	err := ValidateNameSafe("/absolute")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separator")
}

func TestUT_ValidateNameSafe_RejectsTrailingSlash(t *testing.T) {
	err := ValidateNameSafe("name/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path separator")
}

func TestUT_GenerateAction_PathTraversal(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"../../evil", "target"}, nil)

	err := generateAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestUT_GenerateAction_PathTraversalInTarget(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"validGen", "../../evil"}, nil)

	err := generateAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestUT_NewAction_PathTraversal(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"../../evil"}, nil)

	err := newAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestUT_BundleAction_PathTraversal(t *testing.T) {
	tmpDir := setupTempDir(t)
	cfg := createTestConfig(t, tmpDir)

	ctx := createTestCLIContext(t, []string{"../../evil"}, nil)

	err := bundleAction(ctx, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

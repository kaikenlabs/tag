package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_SuggestConvertedTemplateName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple local path",
			input:    "./my-template",
			expected: "my-template-tag",
		},
		{
			name:     "github shorthand",
			input:    "gh:user/awesome-template",
			expected: "awesome-template-tag",
		},
		{
			name:     "github shorthand with version",
			input:    "gh:user/awesome-template@v1.0.0",
			expected: "awesome-template-tag",
		},
		{
			name:     "gitlab shorthand",
			input:    "gl:org/cookiecutter-django",
			expected: "django-tag",
		},
		{
			name:     "bitbucket shorthand",
			input:    "bb:team/cookiecutter-go-api",
			expected: "go-api-tag",
		},
		{
			name:     "cookiecutter prefix stripped",
			input:    "cookiecutter-python",
			expected: "python-tag",
		},
		{
			name:     "no cookiecutter prefix",
			input:    "my-project",
			expected: "my-project-tag",
		},
		{
			name:     "absolute path",
			input:    "/home/user/templates/cookiecutter-rust",
			expected: "rust-tag",
		},
		{
			name:     "nested subdir in shorthand",
			input:    "gh:user/templates/subdir",
			expected: "subdir-tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := suggestConvertedTemplateName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_ScaffoldAction_MissingArguments(t *testing.T) {
	ctx := createTestCLIContext(t, []string{}, nil)

	err := scaffoldAction(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template path is required")
}

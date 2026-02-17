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
			expected: "templates-tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := suggestConvertedTemplateName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_DeriveTemplateName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "github shorthand",
			input:    "gh:user/awesome-template",
			expected: "awesome-template",
		},
		{
			name:     "bitbucket shorthand",
			input:    "bb:whalar/go-ms-service-template",
			expected: "go-ms-service-template",
		},
		{
			name:     "gitlab shorthand",
			input:    "gl:org/my-template",
			expected: "my-template",
		},
		{
			name:     "github with version",
			input:    "gh:user/repo@v1.0.0",
			expected: "repo",
		},
		{
			name:     "cookiecutter prefix stripped",
			input:    "gh:user/cookiecutter-django",
			expected: "django",
		},
		{
			name:     "git URL",
			input:    "https://github.com/user/my-template.git",
			expected: "my-template",
		},
		{
			name:     "cookiecutter prefix only with .git suffix",
			input:    "gh:user/cookiecutter-.git",
			expected: "gh:user/cookiecutter-.git",
		},
		{
			name:     "cookiecutter prefix only",
			input:    "gh:user/cookiecutter-",
			expected: "gh:user/cookiecutter-",
		},
		{
			name:     "URL without .git suffix",
			input:    "https://github.com/user/my-template",
			expected: "my-template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveTemplateName(tt.input)
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

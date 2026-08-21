package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kaikenlabs/tag/internal/remote"
	"github.com/kaikenlabs/tag/internal/scaffold"
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
			result := remote.DeriveName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUT_LooksLikeBareName(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// Bare names → true
		{"go-api", true},
		{"my-template", true},
		{"flask", true},

		// Local paths → false
		{"./my-template", false},
		{"../templates/foo", false},
		{"/absolute/path", false},

		// Remote shorthands → false
		{"gh:user/repo", false},
		{"gl:org/template", false},
		{"bb:team/project", false},

		// URLs → false
		{"https://github.com/user/repo.git", false},
		{"http://example.com/template.zip", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, looksLikeBareName(tt.input))
		})
	}
}

func TestUT_ScaffoldAction_MissingArguments(t *testing.T) {
	ctx := createTestCLIContext(t, []string{}, nil)

	err := scaffoldAction(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template argument required")
}

func TestUT_ScaffoldFromRef_LibraryNameResolvesToLibrary(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "my-template")

	ctx := createTestCLIContext(t, []string{"my-template"}, nil)

	// scaffoldFromRef should detect "my-template" as a library name and
	// attempt to scaffold from the library. It will fail because the
	// library template directory has no tag.template.json, but the error
	// should come from scaffolding (not from the remote resolver).
	err := scaffoldFromRef(ctx, []string{"my-template"}, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scaffolding failed")
	assert.NotContains(t, err.Error(), "failed to resolve template")
}

func TestUT_ScaffoldFromRef_RemoteRefSkipsLibrary(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "my-template")

	ctx := createTestCLIContext(t, []string{"gh:user/my-template"}, nil)

	// A remote shorthand should NOT match the library, even if a library
	// entry with the same base name exists. It should go to the remote
	// resolver and fail with a resolver error (not a scaffold init error).
	err := scaffoldFromRef(ctx, []string{"gh:user/my-template"}, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve template")
}

func TestUT_ScaffoldFromRef_LocalPathSkipsLibrary(t *testing.T) {
	// setupFakeLibrary mutates package-level var — do NOT use t.Parallel()
	setupFakeLibrary(t, "my-template")

	ctx := createTestCLIContext(t, []string{"./my-template"}, nil)

	// A local path should NOT match the library. It should go to the
	// remote/local resolver.
	err := scaffoldFromRef(ctx, []string{"./my-template"}, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve template")
}

func TestUT_DisplayScaffoldSummary_WritesToOutput(t *testing.T) {
	var buf bytes.Buffer
	result := scaffold.ScaffoldResult{
		OutputDir:   "/tmp/my-app",
		TemplateDir: t.TempDir(),
		Vars: map[string]any{
			"project_name": "my-app",
		},
		Opts: scaffold.Options{
			TemplateName:    "go-api",
			TemplateRef:     "gh:user/go-api",
			TemplateVersion: "1.0.0",
		},
	}

	displayScaffoldSummary(&buf, result)

	output := buf.String()
	assert.Contains(t, output, "Scaffolding complete!")
	assert.Contains(t, output, "Output: /tmp/my-app")
	assert.Contains(t, output, "Project: my-app")
	assert.Contains(t, output, "Template: gh:user/go-api (1.0.0)")
	assert.Contains(t, output, "cd /tmp/my-app")
}

func TestUT_DisplayScaffoldSummary_NoTemplateOrigin(t *testing.T) {
	var buf bytes.Buffer
	result := scaffold.ScaffoldResult{
		OutputDir:   "/tmp/local-project",
		TemplateDir: t.TempDir(),
		Vars: map[string]any{
			"project_name": "local-project",
		},
		Opts: scaffold.Options{}, // No template name
	}

	displayScaffoldSummary(&buf, result)

	output := buf.String()
	assert.Contains(t, output, "Scaffolding complete!")
	assert.Contains(t, output, "Output: /tmp/local-project")
	assert.NotContains(t, output, "Template:")
}

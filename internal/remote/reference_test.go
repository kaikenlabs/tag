package remote

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Parse_GitHubShorthand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Reference
	}{
		{
			name:  "simple",
			input: "gh:user/repo",
			expected: &Reference{
				Original: "gh:user/repo",
				Type:     ReferenceTypeGit,
				Provider: ProviderGitHub,
				Host:     "github.com",
				Owner:    "user",
				Repo:     "repo",
				URL:      "https://github.com/user/repo.git",
			},
		},
		{
			name:  "with org",
			input: "gh:myorg/myrepo",
			expected: &Reference{
				Original: "gh:myorg/myrepo",
				Type:     ReferenceTypeGit,
				Provider: ProviderGitHub,
				Host:     "github.com",
				Owner:    "myorg",
				Repo:     "myrepo",
				URL:      "https://github.com/myorg/myrepo.git",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := Parse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Original, ref.Original)
			assert.Equal(t, tt.expected.Type, ref.Type)
			assert.Equal(t, tt.expected.Provider, ref.Provider)
			assert.Equal(t, tt.expected.Host, ref.Host)
			assert.Equal(t, tt.expected.Owner, ref.Owner)
			assert.Equal(t, tt.expected.Repo, ref.Repo)
			assert.Equal(t, tt.expected.URL, ref.URL)
		})
	}
}

func TestUT_Parse_GitLabShorthand(t *testing.T) {
	ref, err := Parse("gl:myorg/myrepo")
	require.NoError(t, err)

	assert.Equal(t, ReferenceTypeGit, ref.Type)
	assert.Equal(t, ProviderGitLab, ref.Provider)
	assert.Equal(t, "gitlab.com", ref.Host)
	assert.Equal(t, "myorg", ref.Owner)
	assert.Equal(t, "myrepo", ref.Repo)
	assert.Equal(t, "https://gitlab.com/myorg/myrepo.git", ref.URL)
}

func TestUT_Parse_BitbucketShorthand(t *testing.T) {
	ref, err := Parse("bb:myorg/myrepo")
	require.NoError(t, err)

	assert.Equal(t, ReferenceTypeGit, ref.Type)
	assert.Equal(t, ProviderBitbucket, ref.Provider)
	assert.Equal(t, "bitbucket.org", ref.Host)
	assert.Equal(t, "myorg", ref.Owner)
	assert.Equal(t, "myrepo", ref.Repo)
	assert.Equal(t, "https://bitbucket.org/myorg/myrepo.git", ref.URL)
}

func TestUT_Parse_VersionSuffix(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedVersion string
		expectedSubPath string
	}{
		{
			name:            "tag version",
			input:           "gh:user/repo@v1.0.0",
			expectedVersion: "v1.0.0",
			expectedSubPath: "",
		},
		{
			name:            "branch version",
			input:           "gh:user/repo@main",
			expectedVersion: "main",
			expectedSubPath: "",
		},
		{
			name:            "commit sha",
			input:           "gh:user/repo@abc123def",
			expectedVersion: "abc123def",
			expectedSubPath: "",
		},
		{
			name:            "version with subpath",
			input:           "gh:user/repo@v1.0.0/templates/go",
			expectedVersion: "v1.0.0",
			expectedSubPath: "templates/go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := Parse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedVersion, ref.Version)
			assert.Equal(t, tt.expectedSubPath, ref.SubPath)
		})
	}
}

func TestUT_Parse_SubPath(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedSubPath string
	}{
		{
			name:            "single level",
			input:           "gh:user/repo/templates",
			expectedSubPath: "templates",
		},
		{
			name:            "multi level",
			input:           "gh:user/repo/templates/go/api",
			expectedSubPath: "templates/go/api",
		},
		{
			name:            "trailing slash normalized",
			input:           "gh:user/repo/templates/",
			expectedSubPath: "templates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := Parse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedSubPath, ref.SubPath)
		})
	}
}

func TestUT_Parse_VersionAndSubPath(t *testing.T) {
	ref, err := Parse("gh:user/repo@v2.0.0/templates/python")
	require.NoError(t, err)

	assert.Equal(t, "v2.0.0", ref.Version)
	assert.Equal(t, "templates/python", ref.SubPath)
	assert.Equal(t, "user", ref.Owner)
	assert.Equal(t, "repo", ref.Repo)
}

func TestUT_Parse_FullGitURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Reference
	}{
		{
			name:  "github https",
			input: "https://github.com/user/repo.git",
			expected: &Reference{
				Type:     ReferenceTypeGit,
				Provider: ProviderGitHub,
				Host:     "github.com",
				Owner:    "user",
				Repo:     "repo",
				URL:      "https://github.com/user/repo.git",
			},
		},
		{
			name:  "github https without .git",
			input: "https://github.com/user/repo",
			expected: &Reference{
				Type:     ReferenceTypeGit,
				Provider: ProviderGitHub,
				Host:     "github.com",
				Owner:    "user",
				Repo:     "repo",
				URL:      "https://github.com/user/repo.git",
			},
		},
		{
			name:  "gitlab https",
			input: "https://gitlab.com/org/project.git",
			expected: &Reference{
				Type:     ReferenceTypeGit,
				Provider: ProviderGitLab,
				Host:     "gitlab.com",
				Owner:    "org",
				Repo:     "project",
				URL:      "https://gitlab.com/org/project.git",
			},
		},
		{
			name:  "generic host",
			input: "https://git.example.com/team/project.git",
			expected: &Reference{
				Type:     ReferenceTypeGit,
				Provider: ProviderGeneric,
				Host:     "git.example.com",
				Owner:    "team",
				Repo:     "project",
				URL:      "https://git.example.com/team/project.git",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := Parse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Type, ref.Type)
			assert.Equal(t, tt.expected.Provider, ref.Provider)
			assert.Equal(t, tt.expected.Host, ref.Host)
			assert.Equal(t, tt.expected.Owner, ref.Owner)
			assert.Equal(t, tt.expected.Repo, ref.Repo)
			assert.Equal(t, tt.expected.URL, ref.URL)
		})
	}
}

func TestUT_Parse_GitHubWebURL(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedVersion string
		expectedSubPath string
	}{
		{
			name:            "tree URL",
			input:           "https://github.com/user/repo/tree/main/templates",
			expectedVersion: "main",
			expectedSubPath: "templates",
		},
		{
			name:            "tree URL deep path",
			input:           "https://github.com/user/repo/tree/v1.0.0/src/templates/go",
			expectedVersion: "v1.0.0",
			expectedSubPath: "src/templates/go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := Parse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, ReferenceTypeGit, ref.Type)
			assert.Equal(t, tt.expectedVersion, ref.Version)
			assert.Equal(t, tt.expectedSubPath, ref.SubPath)
		})
	}
}

func TestUT_Parse_GitSSHURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "git+ssh scheme",
			input: "git+ssh://git@github.com/user/repo.git",
		},
		{
			name:  "git scheme",
			input: "git://github.com/user/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := Parse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, ReferenceTypeGit, ref.Type)
			assert.Equal(t, "user", ref.Owner)
			assert.Equal(t, "repo", ref.Repo)
		})
	}
}

func TestUT_Parse_SSHStyleURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *Reference
	}{
		{
			name:  "basic",
			input: "git@github.com:user/repo.git",
			expected: &Reference{
				Type:     ReferenceTypeGit,
				Provider: ProviderGitHub,
				Host:     "github.com",
				Owner:    "user",
				Repo:     "repo",
			},
		},
		{
			name:  "without .git",
			input: "git@github.com:user/repo",
			expected: &Reference{
				Type:     ReferenceTypeGit,
				Provider: ProviderGitHub,
				Host:     "github.com",
				Owner:    "user",
				Repo:     "repo",
			},
		},
		{
			name:  "with version",
			input: "git@github.com:user/repo.git@v1.0.0",
			expected: &Reference{
				Type:     ReferenceTypeGit,
				Provider: ProviderGitHub,
				Host:     "github.com",
				Owner:    "user",
				Repo:     "repo",
				Version:  "v1.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := Parse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Type, ref.Type)
			assert.Equal(t, tt.expected.Provider, ref.Provider)
			assert.Equal(t, tt.expected.Host, ref.Host)
			assert.Equal(t, tt.expected.Owner, ref.Owner)
			assert.Equal(t, tt.expected.Repo, ref.Repo)
			assert.Equal(t, tt.expected.Version, ref.Version)
		})
	}
}

func TestUT_Parse_ZipURL(t *testing.T) {
	ref, err := Parse("https://example.com/templates/my-template.zip")
	require.NoError(t, err)

	assert.Equal(t, ReferenceTypeZip, ref.Type)
	assert.Equal(t, ProviderGeneric, ref.Provider)
	assert.Equal(t, "example.com", ref.Host)
	assert.Equal(t, "https://example.com/templates/my-template.zip", ref.URL)
}

func TestUT_Parse_LocalPath(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, "my-template")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))

	ref, err := Parse(templateDir)
	require.NoError(t, err)

	assert.Equal(t, ReferenceTypeLocal, ref.Type)
	assert.Equal(t, ProviderGeneric, ref.Provider)
	assert.Equal(t, templateDir, ref.URL)
}

func TestUT_Parse_LocalZip(t *testing.T) {
	// Create a temp zip file
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "template.zip")
	require.NoError(t, os.WriteFile(zipPath, []byte("PK"), 0o644)) // Minimal zip header

	ref, err := Parse(zipPath)
	require.NoError(t, err)

	assert.Equal(t, ReferenceTypeZip, ref.Type)
	assert.Equal(t, ProviderGeneric, ref.Provider)
	assert.Equal(t, zipPath, ref.URL)
}

func TestUT_Parse_InvalidReference(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		errContains string
	}{
		{
			name:        "empty string",
			input:       "",
			errContains: "empty reference",
		},
		{
			name:        "whitespace only",
			input:       "   ",
			errContains: "empty reference",
		},
		{
			name:        "shorthand without repo",
			input:       "gh:user",
			errContains: "invalid shorthand format",
		},
		{
			name:        "shorthand empty",
			input:       "gh:",
			errContains: "missing repository path",
		},
		{
			name:        "nonexistent local path",
			input:       "/nonexistent/path/to/template",
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestUT_Reference_IsRemote(t *testing.T) {
	tests := []struct {
		name     string
		ref      *Reference
		expected bool
	}{
		{
			name: "git reference",
			ref: &Reference{
				Type: ReferenceTypeGit,
				URL:  "https://github.com/user/repo.git",
			},
			expected: true,
		},
		{
			name: "remote zip",
			ref: &Reference{
				Type: ReferenceTypeZip,
				URL:  "https://example.com/template.zip",
			},
			expected: true,
		},
		{
			name: "local zip",
			ref: &Reference{
				Type: ReferenceTypeZip,
				URL:  "/path/to/template.zip",
			},
			expected: false,
		},
		{
			name: "local directory",
			ref: &Reference{
				Type: ReferenceTypeLocal,
				URL:  "/path/to/template",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ref.IsRemote())
		})
	}
}

func TestUT_Reference_CacheKey(t *testing.T) {
	tests := []struct {
		name     string
		ref      *Reference
		expected string
	}{
		{
			name: "github simple",
			ref: &Reference{
				Provider: ProviderGitHub,
				Owner:    "user",
				Repo:     "repo",
			},
			expected: "gh_user_repo",
		},
		{
			name: "github with version",
			ref: &Reference{
				Provider: ProviderGitHub,
				Owner:    "user",
				Repo:     "repo",
				Version:  "v1.0.0",
			},
			expected: "gh_user_repo@v1.0.0",
		},
		{
			name: "gitlab",
			ref: &Reference{
				Provider: ProviderGitLab,
				Owner:    "org",
				Repo:     "project",
			},
			expected: "gl_org_project",
		},
		{
			name: "bitbucket",
			ref: &Reference{
				Provider: ProviderBitbucket,
				Owner:    "team",
				Repo:     "repo",
			},
			expected: "bb_team_repo",
		},
		{
			name: "generic URL uses hash",
			ref: &Reference{
				Provider: ProviderGeneric,
				URL:      "https://example.com/template.zip",
			},
			expected: "_url_", // Starts with _url_, rest is hash
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.ref.CacheKey()
			if tt.ref.Provider == ProviderGeneric {
				assert.True(t, len(key) > len("_url_"))
				assert.Equal(t, "_url_", key[:5])
			} else {
				assert.Equal(t, tt.expected, key)
			}
		})
	}
}

func TestUT_Reference_String(t *testing.T) {
	tests := []struct {
		name     string
		ref      *Reference
		expected string
	}{
		{
			name: "github simple",
			ref: &Reference{
				Original: "gh:user/repo",
				Provider: ProviderGitHub,
				Owner:    "user",
				Repo:     "repo",
			},
			expected: "gh:user/repo",
		},
		{
			name: "github with version",
			ref: &Reference{
				Original: "gh:user/repo@v1.0.0",
				Provider: ProviderGitHub,
				Owner:    "user",
				Repo:     "repo",
				Version:  "v1.0.0",
			},
			expected: "gh:user/repo@v1.0.0",
		},
		{
			name: "github with subpath",
			ref: &Reference{
				Original: "gh:user/repo/templates",
				Provider: ProviderGitHub,
				Owner:    "user",
				Repo:     "repo",
				SubPath:  "templates",
			},
			expected: "gh:user/repo/templates",
		},
		{
			name: "generic uses original",
			ref: &Reference{
				Original: "https://example.com/template.zip",
				Provider: ProviderGeneric,
			},
			expected: "https://example.com/template.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ref.String())
		})
	}
}

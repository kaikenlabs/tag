package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/validate"
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

func TestUT_Parse_ShorthandWithGitSuffix(t *testing.T) {
	// Test that .git suffix is stripped from shorthand refs to avoid double .git in URL
	tests := []struct {
		name         string
		input        string
		expectedURL  string
		expectedRepo string
	}{
		{
			name:         "github with .git",
			input:        "gh:user/repo.git",
			expectedURL:  "https://github.com/user/repo.git",
			expectedRepo: "repo",
		},
		{
			name:         "gitlab with .git",
			input:        "gl:org/project.git",
			expectedURL:  "https://gitlab.com/org/project.git",
			expectedRepo: "project",
		},
		{
			name:         "bitbucket with .git",
			input:        "bb:team/myrepo.git",
			expectedURL:  "https://bitbucket.org/team/myrepo.git",
			expectedRepo: "myrepo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := Parse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedRepo, ref.Repo, "Repo should have .git stripped")
			assert.Equal(t, tt.expectedURL, ref.URL, "URL should have single .git suffix")
		})
	}
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

func TestUT_ValidateSubPath(t *testing.T) {
	tests := []struct {
		name    string
		subPath string
		wantErr bool
	}{
		{name: "empty string", subPath: "", wantErr: false},
		{name: "simple directory", subPath: "templates", wantErr: false},
		{name: "nested directory", subPath: "templates/go/api", wantErr: false},
		{name: "dotdot traversal", subPath: "../../etc/passwd", wantErr: true},
		{name: "leading dotdot", subPath: "../secret", wantErr: true},
		{name: "embedded dotdot", subPath: "a/../../b", wantErr: true},
		{name: "dotdot only", subPath: "..", wantErr: true},
		{name: "absolute path unix", subPath: "/etc/passwd", wantErr: true},
		{name: "backslash separator", subPath: "a\\b\\c", wantErr: true},
		{name: "dotdot with trailing slash", subPath: "../", wantErr: true},
		{name: "single dot is valid", subPath: ".", wantErr: false},
		{name: "normalized clean path", subPath: "a/b/../c", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSubPath(tt.subPath)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUT_Parse_SubPath_Rejects_Traversal(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		errContains string
	}{
		{
			name:        "shorthand with dotdot",
			input:       "gh:user/repo/../../etc",
			errContains: "path traversal",
		},
		{
			name:        "shorthand version with dotdot subpath",
			input:       "gh:user/repo@main/../../../etc",
			errContains: "path traversal",
		},
		{
			name:        "https URL with dotdot subpath",
			input:       "https://github.com/user/repo/tree/main/../../etc",
			errContains: "path traversal",
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

func TestUT_ValidateRefComponent(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid owner", value: "user", wantErr: false},
		{name: "valid org with hyphens", value: "my-org-123", wantErr: false},
		{name: "empty", value: "", wantErr: true},
		{name: "dotdot", value: "..", wantErr: true},
		{name: "single dot", value: ".", wantErr: true},
		{name: "forward slash", value: "user/evil", wantErr: true},
		{name: "backslash", value: "user\\evil", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRefComponent("test", tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUT_Parse_RejectsTraversalInOwnerRepo(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		errContains string
	}{
		{
			name:        "shorthand dotdot owner",
			input:       "gh:../repo",
			errContains: "reserved name",
		},
		{
			name:        "shorthand dotdot repo",
			input:       "gh:user/..",
			errContains: "reserved name",
		},
		{
			name:        "https dotdot owner",
			input:       "https://github.com/../repo.git",
			errContains: "reserved name",
		},
		{
			name:        "https dotdot repo",
			input:       "https://github.com/user/...git",
			errContains: "reserved name",
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
		name       string
		ref        *Reference
		wantPrefix string
	}{
		{
			name: "github simple",
			ref: &Reference{
				Provider: ProviderGitHub,
				Owner:    "user",
				Repo:     "repo",
			},
			wantPrefix: "gh_user_repo",
		},
		{
			name: "github with version",
			ref: &Reference{
				Provider: ProviderGitHub,
				Owner:    "user",
				Repo:     "repo",
				Version:  "v1.0.0",
			},
			wantPrefix: "gh_user_repo@v1.0.0",
		},
		{
			name: "gitlab",
			ref: &Reference{
				Provider: ProviderGitLab,
				Owner:    "org",
				Repo:     "project",
			},
			wantPrefix: "gl_org_project",
		},
		{
			name: "bitbucket",
			ref: &Reference{
				Provider: ProviderBitbucket,
				Owner:    "team",
				Repo:     "repo",
			},
			wantPrefix: "bb_team_repo",
		},
		{
			name: "generic URL uses hash",
			ref: &Reference{
				Provider: ProviderGeneric,
				URL:      "https://example.com/template.zip",
			},
			wantPrefix: "_url_", // Starts with _url_, rest is hash
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.ref.CacheKey()
			if tt.ref.Provider == ProviderGeneric {
				assert.True(t, len(key) > len("_url_"))
				assert.Equal(t, "_url_", key[:5])
			} else {
				got := tt.ref.CacheKey()
				assert.True(t, strings.HasPrefix(got, tt.wantPrefix+"-"),
					"key %q must keep the readable prefix %q for debuggability", got, tt.wantPrefix)
				assert.Regexp(t, `^`+regexp.QuoteMeta(tt.wantPrefix)+`-[0-9a-f]{12}$`, got,
					"the identity digest is what makes the key collision-proof; see CacheKey")
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

func TestUT_DeriveName(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{
			name:     "github shorthand",
			ref:      "gh:user/my-template",
			expected: "my-template",
		},
		{
			name:     "gitlab shorthand",
			ref:      "gl:org/my-template",
			expected: "my-template",
		},
		{
			name:     "bitbucket shorthand",
			ref:      "bb:team/go-service",
			expected: "go-service",
		},
		{
			name:     "strips cookiecutter prefix",
			ref:      "gh:user/cookiecutter-django",
			expected: "django",
		},
		{
			name:     "strips .git suffix",
			ref:      "https://github.com/user/my-template.git",
			expected: "my-template",
		},
		{
			name:     "strips cookiecutter prefix and .git suffix",
			ref:      "https://github.com/user/cookiecutter-flask.git",
			expected: "flask",
		},
		{
			name:     "local path",
			ref:      "/some/path/to/template",
			expected: "template",
		},
		{
			name:     "relative path",
			ref:      "./my-template",
			expected: "my-template",
		},
		{
			name:     "with version tag",
			ref:      "gh:user/my-template@v1.0.0",
			expected: "my-template",
		},
		{
			name:     "empty string returns original",
			ref:      "",
			expected: "",
		},
		{
			name:     "dot returns original",
			ref:      ".",
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, DeriveName(tt.ref))
		})
	}
}

var libraryNameDigestSuffix = regexp.MustCompile(`-[0-9a-f]{12}$`)

func TestUT_LibraryName_DistinctIdentities(t *testing.T) {
	tests := []struct {
		name string
		a, b string
	}{
		{"owner differs", "gh:acme/api", "gh:other/api"},
		{"provider differs", "gh:a/b", "gl:a/b"},
		{"host differs", "https://git.one.invalid/a/b.git", "https://git.two.invalid/a/b.git"},
		// subpath-in / version-out is a DELIBERATE divergence from
		// identityDigest() (used by CacheKey): a library slot stores the
		// copied subdirectory, so two subpaths of one repo are two
		// different templates. Do not "align" this with the cache digest.
		{"subpath differs", "gh:a/b/x", "gh:a/b/y"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEqual(t, LibraryName(tt.a), LibraryName(tt.b))
		})
	}
}

func TestUT_LibraryName_VersionExcluded(t *testing.T) {
	tests := []struct {
		name   string
		v1, v2 string
	}{
		{"shorthand", "gh:a/b@v1", "gh:a/b@v2"},
		// These three git URL syntaxes keep "@version" inside Reference.URL
		// itself (verified by reading buildReference/parseGitURL/
		// parseSSHStyle), unlike shorthand and zip URLs. Without stripping
		// the version suffix from the URL before hashing, these three rows
		// would fail while the others pass.
		{"git URL", "git://example.com/a/b.git@v1", "git://example.com/a/b.git@v2"},
		{"git+ssh URL", "git+ssh://git@example.com/a/b.git@v1", "git+ssh://git@example.com/a/b.git@v2"},
		{"ssh-style", "git@github.com:a/b.git@v1", "git@github.com:a/b.git@v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, LibraryName(tt.v1), LibraryName(tt.v2))
		})
	}

	unversioned := map[string]string{
		"gh:a/b":                            "gh:a/b@v1",
		"git://example.com/a/b.git":         "git://example.com/a/b.git@v1",
		"git+ssh://git@example.com/a/b.git": "git+ssh://git@example.com/a/b.git@v1",
		"git@github.com:a/b.git":            "git@github.com:a/b.git@v1",
	}
	for bare, versioned := range unversioned {
		t.Run("matches unversioned/"+bare, func(t *testing.T) {
			assert.Equal(t, LibraryName(bare), LibraryName(versioned))
		})
	}
}

func TestUT_LibraryName_ZipURLsAreDistinct(t *testing.T) {
	// Parsed zip refs have Host and URL but empty Owner/Repo, so a digest
	// over owner/repo alone would collapse all three onto the same suffix.
	names := []string{
		LibraryName("https://a.invalid/x.zip"),
		LibraryName("https://b.invalid/x.zip"),
		LibraryName("https://a.invalid/y.zip"),
	}
	assert.NotEqual(t, names[0], names[1])
	assert.NotEqual(t, names[0], names[2])
	assert.NotEqual(t, names[1], names[2])
}

func TestUT_LibraryName_LocalRefsMatchDeriveName(t *testing.T) {
	// No-change guard: local refs are out of scope for #430 (--as covers
	// local disambiguation), so LibraryName must be byte-identical to
	// DeriveName for them. This passes on both sides of the #430 change and
	// exists to catch a future regression, not to prove the change works.
	dir := t.TempDir()
	sub := filepath.Join(dir, "tpl")
	require.NoError(t, os.Mkdir(sub, 0o755))

	zipPath := filepath.Join(dir, "archive.zip")
	require.NoError(t, os.WriteFile(zipPath, []byte("PK\x03\x04"), 0o600))

	for _, ref := range []string{sub, zipPath} {
		t.Run(ref, func(t *testing.T) {
			require.True(t, IsLocal(ref))
			assert.Equal(t, DeriveName(ref), LibraryName(ref))
		})
	}
}

func TestUT_LibraryName_StableAndWellFormed(t *testing.T) {
	const ref = "gh:acme/service-template@v1.2.3/sub"

	first := LibraryName(ref)
	second := LibraryName(ref)
	assert.Equal(t, first, second, "LibraryName must be deterministic")
	assert.Regexp(t, libraryNameDigestSuffix, first)
	assert.True(t, utf8.ValidString(first))
}

func TestUT_LibraryName_DigestTuple(t *testing.T) {
	const ref = "gh:acme/api/subdir@v2.0.0"

	parsed, err := Parse(ref)
	require.NoError(t, err)

	versionStrippedURL := parsed.URL
	if parsed.Version != "" {
		versionStrippedURL = strings.TrimSuffix(versionStrippedURL, "@"+parsed.Version)
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{
		string(parsed.Provider), parsed.Host, parsed.Owner, parsed.Repo, parsed.SubPath, versionStrippedURL,
	}, "\x00")))
	wantSuffix := hex.EncodeToString(sum[:])[:12]

	got := LibraryName(ref)
	assert.True(t, strings.HasSuffix(got, "-"+wantSuffix),
		"LibraryName(%q) = %q, want suffix -%s", ref, got, wantSuffix)
}

func TestUT_LibraryName_LongPrefixTruncatesPrefixNotDigest(t *testing.T) {
	recompute := func(t *testing.T, ref string) string {
		t.Helper()
		parsed, err := Parse(ref)
		require.NoError(t, err)
		versionStrippedURL := parsed.URL
		if parsed.Version != "" {
			versionStrippedURL = strings.TrimSuffix(versionStrippedURL, "@"+parsed.Version)
		}
		sum := sha256.Sum256([]byte(strings.Join([]string{
			string(parsed.Provider), parsed.Host, parsed.Owner, parsed.Repo, parsed.SubPath, versionStrippedURL,
		}, "\x00")))
		return hex.EncodeToString(sum[:])[:12]
	}

	t.Run("very long prefix is truncated", func(t *testing.T) {
		ref := "gh:acme/" + strings.Repeat("x", 300)
		digest := recompute(t, ref)

		got := LibraryName(ref)
		assert.LessOrEqual(t, len(got), validate.MaxNameLen)
		assert.True(t, utf8.ValidString(got))
		assert.True(t, strings.HasSuffix(got, "-"+digest),
			"truncation must shorten the prefix, never the digest: got %q", got)
	})

	t.Run("prefix exactly at the boundary is not truncated", func(t *testing.T) {
		repo := strings.Repeat("y", validate.MaxNameLen-13)
		ref := "gh:acme/" + repo
		digest := recompute(t, ref)

		got := LibraryName(ref)
		assert.Equal(t, validate.MaxNameLen, len(got))
		assert.Equal(t, repo+"-"+digest, got)
	})

	t.Run("multi-byte prefix truncates on a rune boundary", func(t *testing.T) {
		repo := strings.Repeat("é", 300)
		ref := "gh:acme/" + repo
		digest := recompute(t, ref)

		got := LibraryName(ref)
		assert.LessOrEqual(t, len(got), validate.MaxNameLen)
		assert.True(t, utf8.ValidString(got), "truncation must not split a multi-byte rune: got %q", got)
		assert.True(t, strings.HasSuffix(got, "-"+digest))
	})
}

package remote

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUT_Parse_VersionSubPathSplit(t *testing.T) {
	t.Parallel()
	// Version with embedded subpath: version is split on first /
	ref, err := Parse("gh:user/repo@v2.0.0/templates/python")
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", ref.Version)
	assert.Equal(t, "templates/python", ref.SubPath)
}

func TestUT_Parse_ShorthandTrailingSlash(t *testing.T) {
	t.Parallel()
	ref, err := Parse("gl:user/repo/subdir/")
	require.NoError(t, err)
	assert.Equal(t, ProviderGitLab, ref.Provider)
	assert.Equal(t, "subdir", ref.SubPath, "trailing slash should be normalized")
}

func TestUT_Parse_HTTPSURLWithVersion(t *testing.T) {
	t.Parallel()
	ref, err := Parse("https://github.com/user/repo@v3.0.0")
	require.NoError(t, err)
	assert.Equal(t, ReferenceTypeGit, ref.Type)
	assert.Equal(t, "v3.0.0", ref.Version)
	assert.Equal(t, "user", ref.Owner)
	assert.Equal(t, "repo", ref.Repo)
}

func TestUT_Parse_HTTPZipWithVersion(t *testing.T) {
	t.Parallel()
	ref, err := Parse("https://example.com/templates/archive.zip@v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, ReferenceTypeZip, ref.Type)
	assert.Equal(t, "v1.0.0", ref.Version)
}

func TestUT_Parse_GitPlusSSH(t *testing.T) {
	t.Parallel()
	ref, err := Parse("git+ssh://git@github.com/org/project.git")
	require.NoError(t, err)
	assert.Equal(t, ReferenceTypeGit, ref.Type)
	assert.Equal(t, ProviderGitHub, ref.Provider)
	assert.Equal(t, "org", ref.Owner)
	assert.Equal(t, "project", ref.Repo)
}

func TestUT_Parse_GitSchemeWithVersion(t *testing.T) {
	t.Parallel()
	ref, err := Parse("git+ssh://git@gitlab.com/org/repo@v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, ReferenceTypeGit, ref.Type)
	assert.Equal(t, ProviderGitLab, ref.Provider)
	assert.Equal(t, "v1.0.0", ref.Version)
}

func TestUT_Parse_SSHStyleWithSubPath(t *testing.T) {
	t.Parallel()
	ref, err := Parse("git@github.com:user/repo.git@v1.0.0/subdir")
	require.NoError(t, err)
	assert.Equal(t, ReferenceTypeGit, ref.Type)
	assert.Equal(t, "v1.0.0", ref.Version)
	assert.Equal(t, "subdir", ref.SubPath)
}

func TestUT_Parse_InvalidSSHFormat(t *testing.T) {
	t.Parallel()
	_, err := Parse("git@:user/repo.git")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid SSH URL format")
}

func TestUT_Parse_InvalidGitURLFormat(t *testing.T) {
	t.Parallel()
	_, err := Parse("git://")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid git URL format")
}

func TestUT_Parse_HTTPSOnlyHost(t *testing.T) {
	t.Parallel()
	_, err := Parse("https://github.com/useronly")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Git URL, expected /user/repo")
}

func TestUT_Parse_GitHubTreeURLWithVersionAlreadySet(t *testing.T) {
	t.Parallel()
	// When tree URL already specifies version via @, the tree branch is ignored
	ref, err := Parse("https://github.com/user/repo/tree/main/templates@v2.0.0")
	require.NoError(t, err)
	assert.Equal(t, ReferenceTypeGit, ref.Type)
	assert.Equal(t, "v2.0.0", ref.Version)
}

func TestUT_Parse_ShorthandBitbucketWithVersion(t *testing.T) {
	t.Parallel()
	ref, err := Parse("bb:team/project@release/1.0")
	require.NoError(t, err)
	assert.Equal(t, ProviderBitbucket, ref.Provider)
	assert.Equal(t, "release", ref.Version)
	assert.Equal(t, "1.0", ref.SubPath)
}

func TestUT_SanitizeForPath_SpecialChars(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"with/slash", "with_slash"},
		{"with\\backslash", "with_backslash"},
		{"with:colon", "with_colon"},
		{"with*star", "with_star"},
		{"with?question", "with_question"},
		{"with\"quote", "with_quote"},
		{"with<less", "with_less"},
		{"with>greater", "with_greater"},
		{"with|pipe", "with_pipe"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, sanitizeForPath(tt.input))
		})
	}
}

func TestUT_ShortProvider_AllProviders(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "gh", shortProvider(ProviderGitHub))
	assert.Equal(t, "gl", shortProvider(ProviderGitLab))
	assert.Equal(t, "bb", shortProvider(ProviderBitbucket))
	assert.Equal(t, "gen", shortProvider(ProviderGeneric))
	assert.Equal(t, "gen", shortProvider(Provider("unknown")))
}

func TestUT_ResolveHEADFromRefs_MultipleBranches(t *testing.T) {
	t.Parallel()
	mainHash := plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	devHash := plumbing.NewHash("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	refs := []*plumbing.Reference{
		plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("main")),
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("dev"), devHash),
		plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), mainHash),
	}

	sha, err := resolveHEADFromRefs(refs)
	require.NoError(t, err)
	assert.Equal(t, mainHash.String(), sha, "should resolve to the branch HEAD points at")
}

func TestUT_IsLocal_Various(t *testing.T) {
	t.Parallel()
	assert.False(t, IsLocal("gh:user/repo"))
	assert.False(t, IsLocal("https://github.com/user/repo"))
	assert.False(t, IsLocal("invalid input without path"))
}

func TestUT_IsZipFile(t *testing.T) {
	t.Parallel()
	assert.True(t, isZipFile("template.zip"))
	assert.True(t, isZipFile("template.ZIP"))
	assert.True(t, isZipFile("/path/to/template.Zip"))
	assert.False(t, isZipFile("template.tar.gz"))
	assert.False(t, isZipFile("template"))
}

func TestUT_DeriveName_EdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{"ssh url", "git@github.com:user/my-repo.git", "my-repo"},
		{"just cookiecutter prefix", "gh:user/cookiecutter-django", "django"},
		{"url path only", "https://example.com/team/service.git", "service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, DeriveName(tt.ref))
		})
	}
}

package remote

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUT_EnvAuthProvider_GitHubToken(t *testing.T) {
	provider := NewEnvAuthProvider()

	// Set env var
	t.Setenv("GITHUB_TOKEN", "test-github-token")

	token, ok := provider.TokenFor(ProviderGitHub)
	assert.True(t, ok)
	assert.Equal(t, "test-github-token", token)
}

func TestUT_EnvAuthProvider_GitLabToken(t *testing.T) {
	provider := NewEnvAuthProvider()

	t.Setenv("GITLAB_TOKEN", "test-gitlab-token")

	token, ok := provider.TokenFor(ProviderGitLab)
	assert.True(t, ok)
	assert.Equal(t, "test-gitlab-token", token)
}

func TestUT_EnvAuthProvider_BitbucketToken(t *testing.T) {
	provider := NewEnvAuthProvider()

	t.Setenv("BITBUCKET_TOKEN", "test-bitbucket-token")

	token, ok := provider.TokenFor(ProviderBitbucket)
	assert.True(t, ok)
	assert.Equal(t, "test-bitbucket-token", token)
}

func TestUT_EnvAuthProvider_NoToken(t *testing.T) {
	provider := NewEnvAuthProvider()

	// Don't set any env var
	token, ok := provider.TokenFor(ProviderGitHub)
	assert.False(t, ok)
	assert.Empty(t, token)
}

func TestUT_EnvAuthProvider_GenericProvider(t *testing.T) {
	provider := NewEnvAuthProvider()

	token, ok := provider.TokenFor(ProviderGeneric)
	assert.False(t, ok)
	assert.Empty(t, token)
}

func TestUT_EnvAuthProvider_GitAuth_HTTPSWithToken(t *testing.T) {
	provider := NewEnvAuthProvider()
	t.Setenv("GITHUB_TOKEN", "test-token")

	ref := &Reference{
		Type:     ReferenceTypeGit,
		Provider: ProviderGitHub,
		URL:      "https://github.com/user/repo.git",
	}

	auth, err := provider.GitAuth(ref)
	assert.NoError(t, err)
	assert.NotNil(t, auth)
}

func TestUT_EnvAuthProvider_GitAuth_HTTPSWithoutToken(t *testing.T) {
	provider := NewEnvAuthProvider()
	// Don't set token

	ref := &Reference{
		Type:     ReferenceTypeGit,
		Provider: ProviderGitHub,
		URL:      "https://github.com/user/repo.git",
	}

	auth, err := provider.GitAuth(ref)
	assert.NoError(t, err)
	assert.Nil(t, auth) // No auth, will try public access
}

func TestUT_IsSSHURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"git@github.com:user/repo.git", true},
		{"git+ssh://git@github.com/user/repo.git", true},
		{"ssh://git@github.com/user/repo.git", true},
		{"https://github.com/user/repo.git", false},
		{"http://github.com/user/repo.git", false},
		{"git://github.com/user/repo.git", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.expected, isSSHURL(tt.url))
		})
	}
}

func TestUT_MockAuthProvider(t *testing.T) {
	mock := &MockAuthProvider{
		Tokens: map[Provider]string{
			ProviderGitHub: "mock-token",
		},
	}

	token, ok := mock.TokenFor(ProviderGitHub)
	assert.True(t, ok)
	assert.Equal(t, "mock-token", token)

	_, ok = mock.TokenFor(ProviderGitLab)
	assert.False(t, ok)
}

package remote

import (
	"os"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// AuthProvider provides authentication credentials for remote operations.
type AuthProvider interface {
	// TokenFor returns the auth token for a provider, if available.
	TokenFor(provider Provider) (token string, ok bool)

	// GitAuth returns the appropriate go-git auth method for a reference.
	GitAuth(ref *Reference) (transport.AuthMethod, error)
}

// EnvAuthProvider implements AuthProvider using environment variables.
type EnvAuthProvider struct{}

// NewEnvAuthProvider creates a new environment-based auth provider.
func NewEnvAuthProvider() *EnvAuthProvider {
	return &EnvAuthProvider{}
}

// TokenFor returns the auth token from environment variables.
func (p *EnvAuthProvider) TokenFor(provider Provider) (string, bool) {
	var envVar string
	switch provider {
	case ProviderGitHub:
		envVar = "GITHUB_TOKEN"
	case ProviderGitLab:
		envVar = "GITLAB_TOKEN"
	case ProviderBitbucket:
		envVar = "BITBUCKET_TOKEN"
	default:
		return "", false
	}
	return os.LookupEnv(envVar)
}

// GitAuth returns the appropriate go-git auth method for a reference.
func (p *EnvAuthProvider) GitAuth(ref *Reference) (transport.AuthMethod, error) {
	// For SSH URLs, use SSH agent or keys
	if isSSHURL(ref.URL) {
		return p.sshAuth()
	}

	// For HTTPS URLs, try token-based auth
	token, ok := p.TokenFor(ref.Provider)
	if !ok || token == "" {
		// No token available, try without auth (public repo)
		return nil, nil
	}

	// Bitbucket supports multiple token types:
	// - Atlassian API tokens (ATAT...): basic auth with email as username
	// - Workspace/repo access tokens: basic auth with "x-token-auth" as username
	// BITBUCKET_USERNAME selects which format to use.
	if ref.Provider == ProviderBitbucket {
		username := "x-token-auth"
		if bbUser := os.Getenv("BITBUCKET_USERNAME"); bbUser != "" {
			username = bbUser
		}
		return &http.BasicAuth{
			Username: username,
			Password: token,
		}, nil
	}

	// GitHub and GitLab use basic auth with x-access-token
	return &http.BasicAuth{
		Username: "x-access-token",
		Password: token,
	}, nil
}

// isSSHURL checks if the URL is an SSH URL.
func isSSHURL(url string) bool {
	// Check for git@ style
	if len(url) >= 4 && url[:4] == "git@" {
		return true
	}
	// Check for git+ssh:// or ssh://
	if len(url) >= 10 && url[:10] == "git+ssh://" {
		return true
	}
	if len(url) >= 6 && url[:6] == "ssh://" {
		return true
	}
	return false
}

// sshAuth returns SSH authentication from the SSH agent.
func (p *EnvAuthProvider) sshAuth() (transport.AuthMethod, error) {
	// Try to use SSH agent first
	auth, err := ssh.NewSSHAgentAuth("git")
	if err == nil {
		return auth, nil
	}

	// Fall back to default SSH key paths
	// go-git will try ~/.ssh/id_rsa, ~/.ssh/id_dsa, etc.
	return nil, nil
}

// MockAuthProvider is a test double for AuthProvider.
type MockAuthProvider struct {
	Tokens map[Provider]string
}

// TokenFor returns tokens from the mock's Tokens map.
func (m *MockAuthProvider) TokenFor(provider Provider) (string, bool) {
	if m.Tokens == nil {
		return "", false
	}
	token, ok := m.Tokens[provider]
	return token, ok
}

// GitAuth returns auth for testing.
func (m *MockAuthProvider) GitAuth(ref *Reference) (transport.AuthMethod, error) {
	token, ok := m.TokenFor(ref.Provider)
	if !ok || token == "" {
		return nil, nil
	}
	return &http.BasicAuth{
		Username: "x-access-token",
		Password: token,
	}, nil
}

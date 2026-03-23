package remote

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ParseError ---

func TestUT_ParseError_Error(t *testing.T) {
	t.Parallel()

	pe := &ParseError{Input: "bad://ref", Message: "unsupported scheme"}
	got := pe.Error()
	assert.Contains(t, got, "bad://ref")
	assert.Contains(t, got, "unsupported scheme")
}

// --- FetchError ---

func TestUT_FetchError_Error_WithErr(t *testing.T) {
	t.Parallel()

	ref := &Reference{Original: "gh:acme/repo", Provider: ProviderGitHub, Owner: "acme", Repo: "repo"}
	inner := errors.New("timeout")
	fe := &FetchError{Ref: ref, Message: "connection failed", Err: inner}

	got := fe.Error()
	assert.Contains(t, got, "connection failed")
	assert.Contains(t, got, "timeout")
	assert.Contains(t, got, ref.String())
}

func TestUT_FetchError_Error_WithoutErr(t *testing.T) {
	t.Parallel()

	ref := &Reference{Original: "gh:acme/repo", Provider: ProviderGitHub, Owner: "acme", Repo: "repo"}
	fe := &FetchError{Ref: ref, Message: "not found"}

	got := fe.Error()
	assert.Contains(t, got, "not found")
	assert.NotContains(t, got, "nil")
}

func TestUT_FetchError_Unwrap(t *testing.T) {
	t.Parallel()

	inner := errors.New("wrapped")
	fe := &FetchError{Err: inner}
	assert.Equal(t, inner, fe.Unwrap())
}

func TestUT_FetchError_Unwrap_Nil(t *testing.T) {
	t.Parallel()

	fe := &FetchError{}
	assert.Nil(t, fe.Unwrap())
}

// --- CacheError ---

func TestUT_CacheError_Error_WithErr(t *testing.T) {
	t.Parallel()

	inner := errors.New("io error")
	ce := &CacheError{Key: "tmpl-key", Op: "set", Message: "write failed", Err: inner}

	got := ce.Error()
	assert.Contains(t, got, "set")
	assert.Contains(t, got, "tmpl-key")
	assert.Contains(t, got, "write failed")
	assert.Contains(t, got, "io error")
}

func TestUT_CacheError_Error_WithoutErr(t *testing.T) {
	t.Parallel()

	ce := &CacheError{Key: "tmpl-key", Op: "get", Message: "expired"}

	got := ce.Error()
	assert.Contains(t, got, "get")
	assert.Contains(t, got, "tmpl-key")
	assert.Contains(t, got, "expired")
}

func TestUT_CacheError_Unwrap(t *testing.T) {
	t.Parallel()

	inner := errors.New("cache inner")
	ce := &CacheError{Err: inner}
	assert.Equal(t, inner, ce.Unwrap())
}

func TestUT_CacheError_Unwrap_Nil(t *testing.T) {
	t.Parallel()

	ce := &CacheError{}
	assert.Nil(t, ce.Unwrap())
}

// --- AuthError ---

func TestUT_AuthError_Error_GitHub(t *testing.T) {
	t.Parallel()

	ae := &AuthError{Provider: ProviderGitHub, Message: "401 unauthorized"}

	got := ae.Error()
	assert.Contains(t, got, "github")
	assert.Contains(t, got, "401 unauthorized")
	assert.Contains(t, got, "GITHUB_TOKEN")
}

func TestUT_AuthError_Error_GitLab(t *testing.T) {
	t.Parallel()

	ae := &AuthError{Provider: ProviderGitLab, Message: "forbidden"}

	got := ae.Error()
	assert.Contains(t, got, "gitlab")
	assert.Contains(t, got, "GITLAB_TOKEN")
}

func TestUT_AuthError_Error_Bitbucket(t *testing.T) {
	t.Parallel()

	ae := &AuthError{Provider: ProviderBitbucket, Message: "no credentials"}

	got := ae.Error()
	assert.Contains(t, got, "bitbucket")
	assert.Contains(t, got, "BITBUCKET_TOKEN")
}

func TestUT_AuthError_Error_WithErr(t *testing.T) {
	t.Parallel()

	inner := errors.New("network error")
	ae := &AuthError{Provider: ProviderGitHub, Message: "failed", Err: inner}

	got := ae.Error()
	assert.Contains(t, got, "network error")
	assert.Contains(t, got, "GITHUB_TOKEN")
}

func TestUT_AuthError_Unwrap(t *testing.T) {
	t.Parallel()

	inner := errors.New("auth inner")
	ae := &AuthError{Err: inner}
	assert.Equal(t, inner, ae.Unwrap())
}

func TestUT_AuthError_Is_ErrAuthRequired(t *testing.T) {
	t.Parallel()

	ae := &AuthError{Provider: ProviderGitHub, Message: "test"}
	assert.ErrorIs(t, ae, ErrAuthRequired)
}

// --- Provider Constants ---

func TestUT_ProviderConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Provider("github"), ProviderGitHub)
	assert.Equal(t, Provider("gitlab"), ProviderGitLab)
	assert.Equal(t, Provider("bitbucket"), ProviderBitbucket)
}

// --- Sentinel Errors ---

func TestUT_SentinelErrors(t *testing.T) {
	t.Parallel()

	sentinels := []struct {
		name string
		err  error
	}{
		{name: "ErrNotCached", err: ErrNotCached},
		{name: "ErrAuthRequired", err: ErrAuthRequired},
		{name: "ErrNotFound", err: ErrNotFound},
		{name: "ErrVersionNotFound", err: ErrVersionNotFound},
		{name: "ErrSubPathNotFound", err: ErrSubPathNotFound},
	}

	for _, tt := range sentinels {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.NotNil(t, tt.err)
			assert.NotEmpty(t, tt.err.Error())
		})
	}
}

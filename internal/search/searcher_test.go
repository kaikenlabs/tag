package search_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kaikenlabs/tag/internal/search"
)

// fakeRepo builds a minimal GitHub repository JSON object.
func fakeRepo(name, fullName, description, htmlURL string, stars int) map[string]any {
	return map[string]any{
		"name":             name,
		"full_name":        fullName,
		"description":      description,
		"html_url":         htmlURL,
		"stargazers_count": stars,
		"updated_at":       "2025-01-15T12:00:00Z",
		"language":         "Go",
	}
}

func TestUT_SearchGitHub_ReturnsResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search/repositories", r.URL.Path)
		assert.Contains(t, r.URL.Query().Get("q"), "topic:tag-template")
		assert.Contains(t, r.URL.Query().Get("q"), "go api")
		assert.Equal(t, "stars", r.URL.Query().Get("sort"))
		assert.Equal(t, "desc", r.URL.Query().Get("order"))
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		assert.Equal(t, "tag-cli", r.Header.Get("User-Agent"))

		resp := map[string]any{
			"total_count": 2,
			"items": []any{
				fakeRepo("go-api-template", "acme/go-api-template", "A Go API template", "https://github.com/acme/go-api-template", 42),
				fakeRepo("go-grpc-template", "acme/go-grpc-template", "A Go gRPC template", "https://github.com/acme/go-grpc-template", 10),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	results, err := search.SearchGitHub(context.Background(), srv.Client(), "go api", srv.URL, "", search.Options{})
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "go-api-template", results[0].Name)
	assert.Equal(t, "acme/go-api-template", results[0].FullName)
	assert.Equal(t, "A Go API template", results[0].Description)
	assert.Equal(t, "https://github.com/acme/go-api-template", results[0].URL)
	assert.Equal(t, 42, results[0].Stars)
	assert.Equal(t, "Go", results[0].Language)
}

func TestUT_SearchGitHub_EmptyQuery_SearchesAllTemplates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		assert.Equal(t, "topic:tag-template", q, "empty query should only include topic filter")

		resp := map[string]any{"total_count": 0, "items": []any{}}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	results, err := search.SearchGitHub(context.Background(), srv.Client(), "", srv.URL, "", search.Options{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestUT_SearchGitHub_WithToken_SetsAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token ghp_test123", r.Header.Get("Authorization"))

		resp := map[string]any{"total_count": 0, "items": []any{}}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	_, err := search.SearchGitHub(context.Background(), srv.Client(), "", srv.URL, "ghp_test123", search.Options{})
	require.NoError(t, err)
}

func TestUT_SearchGitHub_NoToken_NoAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"))

		resp := map[string]any{"total_count": 0, "items": []any{}}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	_, err := search.SearchGitHub(context.Background(), srv.Client(), "", srv.URL, "", search.Options{})
	require.NoError(t, err)
}

func TestUT_SearchGitHub_Options_LimitAndSort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "5", r.URL.Query().Get("per_page"))
		assert.Equal(t, "updated", r.URL.Query().Get("sort"))
		assert.Equal(t, "asc", r.URL.Query().Get("order"))

		resp := map[string]any{"total_count": 0, "items": []any{}}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	_, err := search.SearchGitHub(context.Background(), srv.Client(), "", srv.URL, "", search.Options{
		Limit: 5,
		Sort:  "updated",
		Order: "asc",
	})
	require.NoError(t, err)
}

func TestUT_SearchGitHub_NonOKStatus_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	_, err := search.SearchGitHub(context.Background(), srv.Client(), "", srv.URL, "", search.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Bad credentials")
}

func TestUT_SearchGitHub_Non200NoBody_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := search.SearchGitHub(context.Background(), srv.Client(), "", srv.URL, "", search.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestUT_SearchGitHub_InvalidJSON_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer srv.Close()

	_, err := search.SearchGitHub(context.Background(), srv.Client(), "", srv.URL, "", search.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestUT_SearchGitHub_ContextCancelled_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate slow server
		time.Sleep(10 * time.Millisecond)
		resp := map[string]any{"total_count": 0, "items": []any{}}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := search.SearchGitHub(ctx, srv.Client(), "", srv.URL, "", search.Options{})
	require.Error(t, err)
}

func TestUT_SearchGitHub_LimitClamped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Limit > 100 should be clamped to 100
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))

		resp := map[string]any{"total_count": 0, "items": []any{}}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	_, err := search.SearchGitHub(context.Background(), srv.Client(), "", srv.URL, "", search.Options{Limit: 999})
	require.NoError(t, err)
}

package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.github.com"
	topicFilter    = "topic:tag-template"
	defaultLimit   = 10
	maxLimit       = 100
)

// SearchGitHub searches GitHub for repositories tagged with "tag-template" plus
// an optional free-text query string.
//
// baseURL overrides the API base (use "" for production). This allows tests to
// inject a httptest.Server URL.
// token is a GitHub personal access token (GITHUB_TOKEN). Pass "" for
// unauthenticated requests (lower rate limits apply).
func SearchGitHub(ctx context.Context, client *http.Client, query, baseURL, token string, opts Options) ([]SearchResult, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	// Build query: always include the topic filter; append free-text if provided.
	q := topicFilter
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		q += " " + trimmed
	}

	// Apply and clamp options.
	limit := opts.Limit
	switch {
	case limit <= 0:
		limit = defaultLimit
	case limit > maxLimit:
		limit = maxLimit
	}
	sort := opts.Sort
	if sort == "" {
		sort = "stars"
	}
	order := opts.Order
	if order == "" {
		order = "desc"
	}

	reqURL, err := url.Parse(baseURL + "/search/repositories")
	if err != nil {
		return nil, fmt.Errorf("parse github api url: %w", err)
	}
	params := url.Values{}
	params.Set("q", q)
	params.Set("sort", sort)
	params.Set("order", order)
	params.Set("per_page", strconv.Itoa(limit))
	reqURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create search request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-Github-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "tag-cli")
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := client.Do(req) //nolint:gosec // G704: baseURL is caller-controlled (empty = production GitHub API, non-empty = test server); SSRF not applicable in a CLI tool
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var ghErr githubErrorResponse
		if jsonErr := json.NewDecoder(resp.Body).Decode(&ghErr); jsonErr == nil && ghErr.Message != "" {
			return nil, fmt.Errorf("github api (%s): %s", resp.Status, ghErr.Message)
		}
		return nil, fmt.Errorf("github api returned %s", resp.Status)
	}

	var apiResp githubSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]SearchResult, 0, len(apiResp.Items))
	for _, repo := range apiResp.Items {
		results = append(results, SearchResult{
			Name:        repo.Name,
			FullName:    repo.FullName,
			Description: repo.Description,
			URL:         repo.HTMLURL,
			Stars:       repo.Stars,
			UpdatedAt:   repo.UpdatedAt,
			Language:    repo.Language,
		})
	}
	return results, nil
}

// Package search provides GitHub repository search for TAG-compatible templates.
// Templates are discovered by the "tag-template" GitHub topic.
package search

import "time"

// SearchResult holds the display data for a single template found on GitHub.
type SearchResult struct {
	Name        string    `json:"name"`        // Repository name (without owner)
	FullName    string    `json:"full_name"`   // owner/repo format
	Description string    `json:"description"` // Repository description
	URL         string    `json:"url"`         // HTML URL to the repository
	Stars       int       `json:"stars"`       // GitHub stargazers count
	UpdatedAt   time.Time `json:"updated_at"`  // Timestamp of the last push
	Language    string    `json:"language"`    // Primary programming language (may be empty)
}

// Options controls GitHub repository search behaviour.
type Options struct {
	// Limit is the maximum number of results to return (1–100). Defaults to 10.
	Limit int
	// Sort orders results by "stars", "forks", or "updated". Defaults to "stars".
	Sort string
	// Order is "asc" or "desc". Defaults to "desc".
	Order string
}

// githubSearchResponse is the top-level GitHub search repositories API response.
type githubSearchResponse struct {
	TotalCount int          `json:"total_count"`
	Items      []githubRepo `json:"items"`
}

// githubRepo is a single repository item from the GitHub search API.
type githubRepo struct {
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"`
	Description string    `json:"description"`
	HTMLURL     string    `json:"html_url"`
	Stars       int       `json:"stargazers_count"`
	UpdatedAt   time.Time `json:"updated_at"`
	Language    string    `json:"language"`
}

// githubErrorResponse is the error body returned by the GitHub API on failure.
type githubErrorResponse struct {
	Message string `json:"message"`
}

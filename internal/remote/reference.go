// Package remote provides functionality for fetching and caching templates from remote sources.
package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// ReferenceType indicates the type of template source.
type ReferenceType string

const (
	// ReferenceTypeGit represents a Git repository source.
	ReferenceTypeGit ReferenceType = "git"
	// ReferenceTypeZip represents a zip file source (remote or local).
	ReferenceTypeZip ReferenceType = "zip"
	// ReferenceTypeLocal represents a local directory source.
	ReferenceTypeLocal ReferenceType = "local"
)

// Provider indicates the Git hosting provider.
type Provider string

const (
	// ProviderGitHub represents GitHub.
	ProviderGitHub Provider = "github"
	// ProviderGitLab represents GitLab.
	ProviderGitLab Provider = "gitlab"
	// ProviderBitbucket represents Bitbucket.
	ProviderBitbucket Provider = "bitbucket"
	// ProviderGeneric represents any other Git host.
	ProviderGeneric Provider = "generic"
)

// Reference represents a parsed template reference.
type Reference struct {
	Original string        // Original input string
	Type     ReferenceType // Git, Zip, Local
	Provider Provider      // GitHub, GitLab, Bitbucket, Generic
	Host     string        // github.com, gitlab.com, etc.
	Owner    string        // user or org
	Repo     string        // repository name
	Version  string        // tag, branch, or commit (empty = default branch)
	SubPath  string        // subdirectory within repo/archive
	URL      string        // Resolved full URL for cloning/downloading
}

// shorthandPrefixes maps shorthand prefixes to their providers and hosts.
var shorthandPrefixes = map[string]struct {
	provider Provider
	host     string
}{
	"gh:": {ProviderGitHub, "github.com"},
	"gl:": {ProviderGitLab, "gitlab.com"},
	"bb:": {ProviderBitbucket, "bitbucket.org"},
}

// hostToProvider maps known hosts to providers.
var hostToProvider = map[string]Provider{
	"github.com":    ProviderGitHub,
	"gitlab.com":    ProviderGitLab,
	"bitbucket.org": ProviderBitbucket,
}

// Parse parses a template reference string into a Reference struct.
// Supported formats:
//   - Shorthand: gh:user/repo, gl:user/repo, bb:user/repo
//   - With version: gh:user/repo@v1.0.0
//   - With subpath: gh:user/repo/subdir or gh:user/repo@v1.0.0/subdir
//   - Full Git URL: https://github.com/user/repo.git
//   - Git+SSH: git+ssh://git@github.com/user/repo.git
//   - Zip URL: https://example.com/template.zip
//   - Local path: ./template, ../template, /absolute/path
//   - Local zip: ./template.zip
func Parse(input string) (*Reference, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, &ParseError{Input: input, Message: "empty reference"}
	}

	// Check for shorthand prefixes (gh:, gl:, bb:)
	for prefix, info := range shorthandPrefixes {
		if strings.HasPrefix(input, prefix) {
			return parseShorthand(input, prefix, info.provider, info.host)
		}
	}

	// Check for URL schemes
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		return parseHTTPURL(input)
	}

	if strings.HasPrefix(input, "git+ssh://") || strings.HasPrefix(input, "git://") {
		return parseGitURL(input)
	}

	// Check for SSH-style URL (git@host:user/repo.git)
	if strings.HasPrefix(input, "git@") {
		return parseSSHStyle(input)
	}

	// Must be a local path
	return parseLocalPath(input)
}

// parseShorthand parses shorthand references like gh:user/repo@v1.0.0/subdir.
func parseShorthand(input, prefix string, provider Provider, host string) (*Reference, error) {
	// Remove prefix
	rest := strings.TrimPrefix(input, prefix)
	if rest == "" {
		return nil, &ParseError{Input: input, Message: "missing repository path after prefix"}
	}

	// Extract version if present (split on @)
	version := ""
	if atIdx := strings.Index(rest, "@"); atIdx != -1 {
		version = rest[atIdx+1:]
		rest = rest[:atIdx]
	}

	// Parse owner/repo/subpath
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return nil, &ParseError{Input: input, Message: "invalid shorthand format, expected user/repo"}
	}

	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git") // Strip .git if present
	subPath := ""
	if len(parts) > 2 {
		subPath = strings.Join(parts[2:], "/")
	}

	ref, err := buildReference(refParts{
		input:    input,
		host:     host,
		owner:    owner,
		repo:     repo,
		version:  version,
		subPath:  subPath,
		cloneURL: fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo),
	})
	if err != nil {
		return nil, err
	}

	// Override provider (shorthand has explicit provider from prefix, not host lookup)
	ref.Provider = provider
	return ref, nil
}

// parseHTTPURL parses HTTP(S) URLs, determining if they're Git repos or zip files.
func parseHTTPURL(input string) (*Reference, error) {
	// Extract version suffix before parsing URL
	version := ""
	urlPart := input
	if atIdx := strings.LastIndex(input, "@"); atIdx != -1 {
		// Check if @ is after the host (not in username:password)
		possibleVersion := input[atIdx+1:]
		// Version shouldn't contain / at the start (that would be a path)
		if !strings.HasPrefix(possibleVersion, "/") && !strings.Contains(input[:atIdx], "?") {
			version = possibleVersion
			urlPart = input[:atIdx]
			// Version with subpath (@v1.0.0/subdir) is handled in parseGitHTTPURL
		}
	}

	parsed, err := url.Parse(urlPart)
	if err != nil {
		return nil, &ParseError{Input: input, Message: fmt.Sprintf("invalid URL: %v", err)}
	}

	// Check if it's a zip file
	if strings.HasSuffix(strings.ToLower(parsed.Path), ".zip") {
		return &Reference{
			Original: input,
			Type:     ReferenceTypeZip,
			Provider: ProviderGeneric,
			Host:     parsed.Host,
			URL:      urlPart,
			Version:  version,
		}, nil
	}

	// Parse as Git URL
	return parseGitHTTPURL(input, parsed, version)
}

// parseGitHTTPURL parses Git repository HTTP URLs.
func parseGitHTTPURL(input string, parsed *url.URL, version string) (*Reference, error) {
	// Parse path: /user/repo.git or /user/repo or /user/repo/tree/branch/subdir
	path := strings.TrimPrefix(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return nil, &ParseError{Input: input, Message: "invalid Git URL, expected /user/repo"}
	}

	owner := parts[0]
	repo := parts[1]
	subPath := ""

	// Handle GitHub/GitLab web URLs: /user/repo/tree/branch/subdir or /user/repo/blob/branch/file
	if len(parts) > 3 && (parts[2] == "tree" || parts[2] == "blob") {
		// parts[3] is the branch/tag
		if version == "" {
			version = parts[3]
		}
		if len(parts) > 4 {
			subPath = strings.Join(parts[4:], "/")
		}
	} else if len(parts) > 2 {
		// Direct subpath without tree/blob
		subPath = strings.Join(parts[2:], "/")
	}

	return buildReference(refParts{
		input:    input,
		host:     parsed.Host,
		owner:    owner,
		repo:     repo,
		version:  version,
		subPath:  subPath,
		cloneURL: fmt.Sprintf("https://%s/%s/%s.git", parsed.Host, owner, repo),
	})
}

// gitURLRegex matches git+ssh:// and git:// URLs.
// Format: scheme://[user@]host[:port]/owner/repo[.git][@version][/subpath]
var gitURLRegex = regexp.MustCompile(`^(git\+ssh|git)://(?:([^@]+)@)?([^/:]+)(?::\d+)?/([^/@]+)/([^/@.]+)(?:\.git)?(?:@([^/]+))?(/.*)?$`)

// parseGitURL parses git:// and git+ssh:// URLs.
func parseGitURL(input string) (*Reference, error) {
	matches := gitURLRegex.FindStringSubmatch(input)
	if matches == nil {
		return nil, &ParseError{Input: input, Message: "invalid git URL format"}
	}

	// matches[1] = scheme (git+ssh or git)
	// matches[2] = user (optional, e.g., "git")
	// matches[3] = host
	// matches[4] = owner
	// matches[5] = repo
	// matches[6] = version (optional)
	// matches[7] = subpath (optional)

	return buildReference(refParts{
		input:   input,
		host:    matches[3],
		owner:   matches[4],
		repo:    matches[5],
		version: matches[6],
		subPath: strings.TrimPrefix(matches[7], "/"),
	})
}

// sshStyleRegex matches git@host:user/repo.git format.
var sshStyleRegex = regexp.MustCompile(`^git@([^:]+):([^/]+)/([^@/]+?)(\.git)?(@[^/]+)?(/.*)?$`)

// parseSSHStyle parses SSH-style URLs like git@github.com:user/repo.git.
func parseSSHStyle(input string) (*Reference, error) {
	matches := sshStyleRegex.FindStringSubmatch(input)
	if matches == nil {
		return nil, &ParseError{Input: input, Message: "invalid SSH URL format, expected git@host:user/repo"}
	}

	return buildReference(refParts{
		input:   input,
		host:    matches[1],
		owner:   matches[2],
		repo:    matches[3],
		version: strings.TrimPrefix(matches[5], "@"),
		subPath: strings.TrimPrefix(matches[6], "/"),
	})
}

// parseLocalPath parses local file system paths.
func parseLocalPath(input string) (*Reference, error) {
	// Check if it's a zip file
	if strings.HasSuffix(strings.ToLower(input), ".zip") {
		// Verify the file exists
		if _, err := os.Stat(input); err != nil {
			if os.IsNotExist(err) {
				return nil, &ParseError{Input: input, Message: "local zip file not found"}
			}
			return nil, &ParseError{Input: input, Message: fmt.Sprintf("cannot access file: %v", err)}
		}

		absPath, err := filepath.Abs(input)
		if err != nil {
			return nil, &ParseError{Input: input, Message: fmt.Sprintf("cannot resolve path: %v", err)}
		}

		return &Reference{
			Original: input,
			Type:     ReferenceTypeZip,
			Provider: ProviderGeneric,
			URL:      absPath,
		}, nil
	}

	// It's a local directory
	absPath, err := filepath.Abs(input)
	if err != nil {
		return nil, &ParseError{Input: input, Message: fmt.Sprintf("cannot resolve path: %v", err)}
	}

	// Verify it exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ParseError{Input: input, Message: "local path not found"}
		}
		return nil, &ParseError{Input: input, Message: fmt.Sprintf("cannot access path: %v", err)}
	}

	if !info.IsDir() {
		return nil, &ParseError{Input: input, Message: "local path is not a directory"}
	}

	return &Reference{
		Original: input,
		Type:     ReferenceTypeLocal,
		Provider: ProviderGeneric,
		URL:      absPath,
	}, nil
}

// IsRemote returns true if this reference requires network access to fetch.
func (r *Reference) IsRemote() bool {
	if r.Type == ReferenceTypeLocal {
		return false
	}
	if r.Type == ReferenceTypeZip {
		// Local zip files don't need network
		return strings.HasPrefix(r.URL, "http://") || strings.HasPrefix(r.URL, "https://")
	}
	return true
}

// CacheKey returns a filesystem-safe cache key for this reference.
// For shorthands: gh_user_repo or gh_user_repo@v1.0.0
// For URLs: _url_{hash}
func (r *Reference) CacheKey() string {
	if r.Provider != ProviderGeneric && r.Owner != "" && r.Repo != "" {
		// Use human-readable format for known providers
		// sanitizeForPath is applied as defense-in-depth (validation rejects bad values at parse time)
		key := fmt.Sprintf("%s_%s_%s", shortProvider(r.Provider), sanitizeForPath(r.Owner), sanitizeForPath(r.Repo))
		if r.Version != "" {
			key += "@" + sanitizeForPath(r.Version)
		}
		return key
	}

	// Hash the URL for generic sources
	hash := sha256.Sum256([]byte(r.URL))
	return "_url_" + hex.EncodeToString(hash[:])[:12]
}

// shortProvider returns a short prefix for the provider.
func shortProvider(p Provider) string {
	switch p {
	case ProviderGitHub:
		return "gh"
	case ProviderGitLab:
		return "gl"
	case ProviderBitbucket:
		return "bb"
	default:
		return "gen"
	}
}

// pathSanitizer is a pre-computed Replacer for characters problematic in file paths.
var pathSanitizer = strings.NewReplacer(
	"/", "_",
	"\\", "_",
	":", "_",
	"*", "_",
	"?", "_",
	"\"", "_",
	"<", "_",
	">", "_",
	"|", "_",
)

// sanitizeForPath removes or replaces characters that are problematic in file paths.
func sanitizeForPath(s string) string {
	return pathSanitizer.Replace(s)
}

// refParts holds the raw parsed components from any URL format.
// buildReference validates and normalizes them into a Reference.
type refParts struct {
	input    string
	host     string
	owner    string
	repo     string
	version  string
	subPath  string
	cloneURL string // pre-built clone URL (empty → use input as URL)
}

// buildReference validates owner/repo, splits embedded version subpaths,
// normalizes the subpath, and resolves the provider from the host.
// This consolidates the shared validation logic across all parse functions.
func buildReference(p refParts) (*Reference, error) {
	// Validate owner and repo against path traversal
	if err := validateRefComponent("owner", p.owner); err != nil {
		return nil, &ParseError{Input: p.input, Message: err.Error()}
	}
	if err := validateRefComponent("repo", p.repo); err != nil {
		return nil, &ParseError{Input: p.input, Message: err.Error()}
	}

	// Split version with embedded subpath (e.g., "v1.0.0/subdir")
	if p.version != "" {
		if slashIdx := strings.Index(p.version, "/"); slashIdx != -1 {
			versionSubPath := p.version[slashIdx+1:]
			p.version = p.version[:slashIdx]
			if p.subPath != "" {
				// Both repo path and version path have subpaths - combine them
				p.subPath = versionSubPath + "/" + p.subPath
			} else {
				p.subPath = versionSubPath
			}
		}
	}

	// Normalize: remove trailing slashes from subpath
	p.subPath = strings.TrimSuffix(p.subPath, "/")

	// Validate subpath against path traversal
	if err := validateSubPath(p.subPath); err != nil {
		return nil, &ParseError{Input: p.input, Message: err.Error()}
	}

	// Resolve provider from host
	provider := ProviderGeneric
	if pr, ok := hostToProvider[p.host]; ok {
		provider = pr
	}

	// Determine URL
	refURL := p.cloneURL
	if refURL == "" {
		refURL = p.input
	}

	return &Reference{
		Original: p.input,
		Type:     ReferenceTypeGit,
		Provider: provider,
		Host:     p.host,
		Owner:    p.owner,
		Repo:     p.repo,
		Version:  p.version,
		SubPath:  p.subPath,
		URL:      refURL,
	}, nil
}

// validateRefComponent checks that a reference component (owner, repo) is safe.
// It rejects empty strings, path traversal sequences, and path separators.
func validateRefComponent(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if value == "." || value == ".." {
		return fmt.Errorf("%s contains path traversal component: %s", name, value)
	}
	if strings.ContainsAny(value, "/\\") {
		return fmt.Errorf("%s contains path separator: %s", name, value)
	}
	return nil
}

// validateSubPath checks that a subpath is safe and does not contain path traversal components.
// It rejects absolute paths, paths with ".." segments, and backslash separators.
func validateSubPath(subPath string) error {
	if subPath == "" {
		return nil
	}

	// Reject absolute paths
	if filepath.IsAbs(subPath) {
		return fmt.Errorf("subpath must be relative, got absolute path: %s", subPath)
	}

	// Reject backslash separators (normalize to forward slash for checking)
	if strings.ContainsRune(subPath, '\\') {
		return fmt.Errorf("subpath contains invalid backslash separator: %s", subPath)
	}

	// Reject any ".." component in the raw path (defense-in-depth: reject before normalization)
	if slices.Contains(strings.Split(subPath, "/"), "..") {
		return fmt.Errorf("subpath contains path traversal component: %s", subPath)
	}

	// Also check the cleaned path for ".." components
	cleaned := filepath.Clean(subPath)
	if slices.Contains(strings.Split(cleaned, string(filepath.Separator)), "..") {
		return fmt.Errorf("subpath contains path traversal component: %s", subPath)
	}

	return nil
}

// String returns a human-readable representation of the reference.
func (r *Reference) String() string {
	if r.Provider != ProviderGeneric && r.Owner != "" && r.Repo != "" {
		result := fmt.Sprintf("%s:%s/%s", shortProvider(r.Provider), r.Owner, r.Repo)
		if r.Version != "" {
			result += "@" + r.Version
		}
		if r.SubPath != "" {
			result += "/" + r.SubPath
		}
		return result
	}
	return r.Original
}

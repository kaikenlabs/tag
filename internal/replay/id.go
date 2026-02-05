package replay

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
)

// nonAlphanumRegex matches any character that is not alphanumeric, underscore, or hyphen.
var nonAlphanumRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// GenerateTemplateID generates a filesystem-safe identifier for a template source.
// The ID is used as the filename for the replay JSON file.
//
// For different source types:
//   - Shorthand (gh:user/repo): gh_user_repo
//   - With version (gh:user/repo@v1.0.0): gh_user_repo_v1.0.0
//   - With subpath (gh:user/repo/subdir): gh_user_repo_subdir
//   - Local paths: local_<sha256-hash-prefix>
//   - HTTP URLs: url_<sha256-hash-prefix>
func GenerateTemplateID(templateSource string) string {
	source := strings.TrimSpace(templateSource)
	if source == "" {
		return ""
	}

	// Handle shorthand prefixes (gh:, gl:, bb:)
	for _, prefix := range []string{"gh:", "gl:", "bb:"} {
		if strings.HasPrefix(source, prefix) {
			return sanitizeShorthand(source, prefix)
		}
	}

	// Handle HTTP/HTTPS URLs
	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://") {
		return hashBasedID("url", source)
	}

	// Handle git+ssh:// and git:// URLs
	if strings.HasPrefix(source, "git+ssh://") || strings.HasPrefix(source, "git://") {
		return hashBasedID("git", source)
	}

	// Handle SSH-style URLs (git@host:user/repo)
	if strings.HasPrefix(source, "git@") {
		return hashBasedID("ssh", source)
	}

	// Handle local paths
	// Normalize to absolute path for consistency
	absPath, err := filepath.Abs(source)
	if err != nil {
		absPath = source
	}
	return hashBasedID("local", absPath)
}

// sanitizeShorthand creates an ID from a shorthand reference.
// Example: gh:user/repo@v1.0.0/subdir -> gh_user_repo_v1.0.0_subdir
func sanitizeShorthand(source, prefix string) string {
	// Remove prefix
	rest := strings.TrimPrefix(source, prefix)

	// Get prefix without colon for the result
	providerPrefix := strings.TrimSuffix(prefix, ":")

	// Extract and process version if present
	version := ""
	if atIdx := strings.Index(rest, "@"); atIdx != -1 {
		version = rest[atIdx+1:]
		rest = rest[:atIdx]
	}

	// Split path components (user/repo/subdir)
	parts := strings.Split(rest, "/")

	// Build result: prefix_user_repo
	var result strings.Builder
	result.WriteString(providerPrefix)
	for _, part := range parts {
		if part != "" {
			result.WriteString("_")
			result.WriteString(sanitizeComponent(part))
		}
	}

	// Add version if present
	if version != "" {
		// Version might contain subpath (v1.0.0/subdir)
		versionParts := strings.Split(version, "/")
		for _, vp := range versionParts {
			if vp != "" {
				result.WriteString("_")
				result.WriteString(sanitizeComponent(vp))
			}
		}
	}

	return result.String()
}

// sanitizeComponent removes or replaces characters that are problematic in file paths.
func sanitizeComponent(s string) string {
	// Replace non-alphanumeric characters (except underscore and hyphen) with underscore
	sanitized := nonAlphanumRegex.ReplaceAllString(s, "_")

	// Remove leading/trailing underscores
	sanitized = strings.Trim(sanitized, "_")

	// Collapse multiple underscores
	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}

	return sanitized
}

// hashBasedID creates an ID using a prefix and a hash of the source.
// Returns: prefix_<12-char-hash>
func hashBasedID(prefix, source string) string {
	hash := sha256.Sum256([]byte(source))
	hashStr := hex.EncodeToString(hash[:])[:12]
	return prefix + "_" + hashStr
}

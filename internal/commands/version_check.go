package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/pkg/app"
)

const (
	defaultGitHubRepo = "https://github.com/kaikenlabs/tag"
	httpTimeout       = 5 * time.Second
)

// VersionCheckCommand returns the version-check command definition.
func VersionCheckCommand(version string) *cli.Command {
	return &cli.Command{
		Name:  "version-check",
		Usage: "Check if a newer version of TAG is available",
		Description: `Checks the latest release on GitHub and compares it to the
currently installed version. Requires network access.

Examples:
  tag version-check`,
		Action: func(c *cli.Context) error {
			return versionCheckAction(c.Context, os.Stdout, version, defaultGitHubRepo)
		},
	}
}

func versionCheckAction(ctx context.Context, w io.Writer, currentVersion, repoURL string) error {
	if isDevBuild(currentVersion) {
		fmt.Fprintln(w, "Development build, version check skipped.")
		return nil
	}

	latest, err := fetchLatestVersion(ctx, repoURL)
	if err != nil {
		return app.Errorf("failed to check for updates: %w", err)
	}

	current := strings.TrimPrefix(currentVersion, "v")
	latestClean := strings.TrimPrefix(latest, "v")

	if current == latestClean {
		fmt.Fprintf(w, "You are up to date! (v%s)\n", current)
		return nil
	}

	fmt.Fprintf(w, "Update available: v%s → v%s\n\n", current, latestClean)
	fmt.Fprintf(w, "  go install github.com/kaikenlabs/tag@v%s\n", latestClean)
	fmt.Fprintf(w, "  # or\n")
	fmt.Fprintf(w, "  curl -sSfL https://raw.githubusercontent.com/kaikenlabs/tag/main/install.sh | sh\n")

	return nil
}

func isDevBuild(version string) bool {
	return version == "" || version == "dev" || strings.HasPrefix(version, "dev-")
}

// fetchLatestVersion resolves the latest release version by following the
// GitHub /releases/latest redirect and extracting the tag from the Location header.
func fetchLatestVersion(ctx context.Context, repoURL string) (string, error) {
	client := &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repoURL+"/releases/latest", http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("unexpected response status %d (expected redirect)", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", errors.New("redirect response missing Location header")
	}

	version := path.Base(location)
	if version == "" || version == "." || version == "/" {
		return "", fmt.Errorf("could not extract version from redirect URL: %s", location)
	}

	return version, nil
}

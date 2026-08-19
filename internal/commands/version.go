package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/jsonout"
	"github.com/kaikenlabs/tag/pkg/app"
)

const (
	defaultGitHubRepo = "https://github.com/kaikenlabs/tag"
	httpTimeout       = 5 * time.Second
)

// versionFlags returns the flags for the version command. version takes no
// positional argument, so there is nothing to reparse.
func versionFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:  "check",
			Usage: "Check if a newer version is available",
		},
		formatFlag(formatText, formatJSON),
	}
}

// VersionCommand returns the version command definition.
func VersionCommand(version string) *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Print the current version, optionally check for updates",
		Description: `Prints the currently installed version of TAG.

With --check, also queries GitHub for the latest release and reports
whether an update is available. Requires network access.

Examples:
  tag version
  tag version --check`,
		Flags: versionFlags(),
		Action: func(c *cli.Context) error {
			return versionAction(c, cmdOut(c), version, defaultGitHubRepo)
		},
	}
}

// versionReport is the `--format json` shape for `tag version`.
//
// Latest and UpdateAvailable are omitted entirely (via a nil pointer / empty
// string) whenever no update check ran — for a plain `tag version` and for a
// dev build's Latest, there is nothing to report and the field must not
// silently claim "no update" by defaulting to false. UpdateAvailable is only
// ever an explicit true/false once a check (network or dev-build-skip)
// actually determined an answer.
type versionReport struct {
	Version         string `json:"version"`
	DevBuild        bool   `json:"dev_build"`
	Latest          string `json:"latest,omitempty"`
	UpdateAvailable *bool  `json:"update_available,omitempty"`
}

// versionAction is the CLI entry point. It resolves --format itself (version
// has no positional argument, so no reparse is needed) and either reproduces
// the historical print sequence or writes the JSON report.
func versionAction(c *cli.Context, w io.Writer, currentVersion, repoURL string) error {
	format, err := resolveFormat(c, formatText, formatJSON)
	if err != nil {
		return err
	}

	check := c.Bool("check")

	if format == formatJSON {
		return writeVersionJSON(c.Context, w, currentVersion, repoURL, check)
	}

	fmt.Fprintf(w, "tag version %s\n", currentVersion)

	if !check {
		return nil
	}

	return versionCheckAction(c.Context, w, currentVersion, repoURL)
}

// writeVersionJSON builds the JSON report and writes it as the single
// document on w. A --check network failure still aborts — text error,
// non-zero exit, per the epic's cross-cutting decision — rather than
// downgrading to a `check_error` field or exit 0: see the ticket's decision
// record. No partial JSON is ever written on that path.
func writeVersionJSON(ctx context.Context, w io.Writer, currentVersion, repoURL string, check bool) error {
	report := versionReport{
		Version:  currentVersion,
		DevBuild: isDevBuild(currentVersion),
	}

	// update_available is present only when an update check was actually
	// requested. Emitting false for a plain `tag version` would read as "checked,
	// nothing new" when nothing was checked at all — absence says "unknown", which
	// is the truth. A dev build answers statically without touching the network.
	if check {
		if report.DevBuild {
			updateAvailable := false
			report.UpdateAvailable = &updateAvailable
		} else {
			latest, updateAvailable, err := checkVersionUpdate(ctx, currentVersion, repoURL)
			if err != nil {
				return app.Errorf("failed to check for updates: %w", err)
			}
			report.Latest = latest
			report.UpdateAvailable = &updateAvailable
		}
	}

	return jsonout.Write(w, report)
}

// checkVersionUpdate fetches the latest release and reports whether it
// differs from currentVersion, both with any "v" prefix stripped.
func checkVersionUpdate(ctx context.Context, currentVersion, repoURL string) (latest string, updateAvailable bool, err error) {
	latestRaw, err := fetchLatestVersion(ctx, repoURL)
	if err != nil {
		return "", false, err
	}

	current := strings.TrimPrefix(currentVersion, "v")
	latestClean := strings.TrimPrefix(latestRaw, "v")

	return latestClean, current != latestClean, nil
}

// versionCheckAction prints the text --check result. Its signature is kept
// stable (ctx, w, currentVersion, repoURL) because the golden text tests call
// it directly with an httptest URL.
func versionCheckAction(ctx context.Context, w io.Writer, currentVersion, repoURL string) error {
	if isDevBuild(currentVersion) {
		fmt.Fprintln(w, "Development build, version check skipped.")
		return nil
	}

	latest, updateAvailable, err := checkVersionUpdate(ctx, currentVersion, repoURL)
	if err != nil {
		return app.Errorf("failed to check for updates: %w", err)
	}

	current := strings.TrimPrefix(currentVersion, "v")

	if !updateAvailable {
		fmt.Fprintf(w, "You are up to date! (v%s)\n", current)
		return nil
	}

	fmt.Fprintf(w, "Update available: v%s → v%s\n\n", current, latest)
	fmt.Fprintf(w, "  tag upgrade\n")

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

	resp, err := client.Do(req) //nolint:gosec // G704: URL is a fixed GitHub releases endpoint, not user-supplied
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

package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/update"
	"github.com/kaikenlabs/tag/pkg/app"
)

// UpdateCommand returns the CLI command for self-updating TAG.
func UpdateCommand(version string) *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Update TAG to the latest version",
		Description: `Downloads and installs the latest TAG release from GitHub.

Checks the current version against the latest release, downloads the
appropriate binary for the current platform, verifies its SHA256 checksum,
and replaces the running binary in-place.

Examples:
  tag update`,
		Action: func(c *cli.Context) error {
			return updateAction(c, os.Stdout, version, defaultGitHubRepo)
		},
	}
}

func updateAction(c *cli.Context, w io.Writer, currentVersion, repoURL string) error {
	fmt.Fprintln(w, "Checking for latest version...")

	latest, err := fetchLatestVersion(c.Context, repoURL)
	if err != nil {
		return app.Errorf("failed to check for updates: %w", err)
	}

	current := strings.TrimPrefix(currentVersion, "v")
	latestClean := strings.TrimPrefix(latest, "v")

	if !isDevBuild(currentVersion) && current == latestClean {
		fmt.Fprintf(w, "Already up to date! (v%s)\n", current)
		return nil
	}

	if isDevBuild(currentVersion) {
		fmt.Fprintf(w, "Development build detected, updating to latest release v%s...\n", latestClean)
	} else {
		fmt.Fprintf(w, "Updating tag v%s → v%s...\n", current, latestClean)
	}

	binaryPath, err := resolveBinaryPath()
	if err != nil {
		return app.Errorf("cannot determine binary path: %w", err)
	}

	updater := update.New(repoURL, w)
	if err := updater.Update(c.Context, latest, binaryPath); err != nil {
		return app.Errorf("update failed: %w", err)
	}

	fmt.Fprintf(w, "Successfully updated to v%s!\n", latestClean)

	return nil
}

// resolveBinaryPath returns the real path of the currently running executable,
// resolving any symlinks.
func resolveBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find executable: %w", err)
	}

	return filepath.EvalSymlinks(exe)
}

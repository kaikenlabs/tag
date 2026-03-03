package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/library"
	"github.com/kaikenlabs/tag/internal/lint"
	"github.com/kaikenlabs/tag/internal/types"
	"github.com/kaikenlabs/tag/internal/xdg"
	"github.com/kaikenlabs/tag/pkg/app"
)

const (
	// doctorExitWarnings is the exit code when one or more checks produce warnings.
	doctorExitWarnings = 1
	// doctorExitFailures is the exit code when one or more checks fail.
	doctorExitFailures = 2
)

// DoctorCommand returns the doctor command definition.
func DoctorCommand(version string) *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Diagnose TAG installation, project setup, and template health",
		Description: `Runs diagnostic checks across four categories:

  ENVIRONMENT   TAG version, Git installation
  PROJECT       .tag/ directory structure
  TEMPLATES     Template config validation
  LIBRARIES     Installed library accessibility

Exit codes:
  0  All checks pass
  1  One or more warnings
  2  One or more failures`,
		Action: func(c *cli.Context) error {
			return doctorAction(c.Context, c.App.Writer, version)
		},
	}
}

type doctorStatus int

const (
	doctorPass doctorStatus = iota
	doctorWarn
	doctorFail
)

type doctorResult struct {
	label   string
	status  doctorStatus
	message string
}

func doctorResultPass(label string) doctorResult {
	return doctorResult{label: label, status: doctorPass}
}

func doctorResultWarn(label, msg string) doctorResult {
	return doctorResult{label: label, status: doctorWarn, message: msg}
}

func doctorResultFail(label, msg string) doctorResult {
	return doctorResult{label: label, status: doctorFail, message: msg}
}

func doctorAction(ctx context.Context, w io.Writer, version string) error {
	var all []doctorResult

	sections := []struct {
		heading string
		results []doctorResult
	}{
		{"ENVIRONMENT", doctorCheckEnvironment(ctx, version)},
		{"PROJECT", doctorCheckProject(".")},
		{"TEMPLATES", doctorCheckTemplates(".")},
		{"LIBRARIES", doctorCheckLibraries()},
	}

	for i, s := range sections {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, s.heading)
		printDoctorResults(w, s.results)
		all = append(all, s.results...)
	}

	hasFail, hasWarn := false, false
	for _, r := range all {
		switch r.status {
		case doctorFail:
			hasFail = true
		case doctorWarn:
			hasWarn = true
		}
	}

	if hasFail {
		return &app.CommandError{Message: "doctor: one or more checks failed", Code: doctorExitFailures}
	}
	if hasWarn {
		return &app.CommandError{Message: "doctor: one or more checks produced warnings", Code: doctorExitWarnings}
	}
	return nil
}

func printDoctorResults(w io.Writer, results []doctorResult) {
	for _, r := range results {
		icon := "✓"
		switch r.status {
		case doctorWarn:
			icon = "⚠"
		case doctorFail:
			icon = "✗"
		}
		if r.message != "" {
			fmt.Fprintf(w, "  %s  %s — %s\n", icon, r.label, r.message)
		} else {
			fmt.Fprintf(w, "  %s  %s\n", icon, r.label)
		}
	}
}

// ---- Environment checks ----

func doctorCheckEnvironment(ctx context.Context, version string) []doctorResult {
	return []doctorResult{
		doctorCheckGit(),
		doctorCheckGitHubToken(),
		doctorCheckTAGVersion(ctx, version),
	}
}

func doctorCheckGit() doctorResult {
	const label = "Git installed"
	if _, err := exec.LookPath("git"); err != nil {
		return doctorResultFail(label, "git not found in PATH — required for remote templates")
	}
	return doctorResultPass(label)
}

func doctorCheckGitHubToken() doctorResult {
	const label = "GITHUB_TOKEN"
	if os.Getenv("GITHUB_TOKEN") == "" {
		return doctorResultWarn(label, "not set — needed for private repos and higher API rate limits")
	}
	return doctorResultPass(label)
}

func doctorCheckTAGVersion(ctx context.Context, version string) doctorResult {
	label := fmt.Sprintf("TAG version (%s)", version)
	if isDevBuild(version) {
		return doctorResultPass(label + " — dev build, skipping update check")
	}
	latest, err := fetchLatestVersion(ctx, defaultGitHubRepo)
	if err != nil {
		return doctorResultWarn(label, fmt.Sprintf("could not check for updates: %v", err))
	}
	if strings.TrimPrefix(version, "v") != strings.TrimPrefix(latest, "v") {
		return doctorResultWarn(label, fmt.Sprintf("update available: %s → %s  (run: tag update)", version, latest))
	}
	return doctorResultPass(label)
}

// ---- Project checks ----

func doctorCheckProject(root string) []doctorResult {
	tagDir := filepath.Join(root, types.TemplatesDir)

	info, err := os.Stat(tagDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []doctorResult{doctorResultWarn(".tag/ directory", "not found — run: tag template init")}
	}
	if err != nil {
		return []doctorResult{doctorResultFail(".tag/ directory", err.Error())}
	}
	if !info.IsDir() {
		return []doctorResult{doctorResultFail(".tag/ directory", ".tag exists but is not a directory")}
	}

	results := []doctorResult{doctorResultPass(".tag/ directory")}
	results = append(results, doctorCheckSubdir(tagDir, types.SharedDir, ".tag/_shared/")...)
	results = append(results, doctorCheckSubdir(tagDir, types.BundlesDir, ".tag/_bundles/")...)
	return results
}

func doctorCheckSubdir(parent, subdir, label string) []doctorResult {
	if _, err := os.Stat(filepath.Join(parent, subdir)); errors.Is(err, fs.ErrNotExist) {
		return []doctorResult{doctorResultWarn(label, "directory not found")}
	} else if err != nil {
		return []doctorResult{doctorResultFail(label, err.Error())}
	}
	return []doctorResult{doctorResultPass(label)}
}

// ---- Template checks ----

func doctorCheckTemplates(root string) []doctorResult {
	tagDir := filepath.Join(root, types.TemplatesDir)
	if _, err := os.Stat(tagDir); err != nil {
		return []doctorResult{doctorResultPass("templates (no .tag/ found, skipped)")}
	}

	entries, err := os.ReadDir(tagDir)
	if err != nil {
		return []doctorResult{doctorResultFail("templates", fmt.Sprintf("cannot read .tag/: %v", err))}
	}

	var results []doctorResult
	found := false
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		tmplDir := filepath.Join(tagDir, e.Name())
		cfgPath := filepath.Join(tmplDir, types.TemplateConfigFile)
		if _, statErr := os.Stat(cfgPath); statErr != nil {
			continue // directory without tag.template.json is not a template
		}
		found = true
		label := fmt.Sprintf("template %q", e.Name())
		linter, lintErr := lint.NewLinter(tmplDir)
		if lintErr != nil {
			results = append(results, doctorResultFail(label, lintErr.Error()))
			continue
		}
		result, lintErr := linter.Run()
		if lintErr != nil {
			results = append(results, doctorResultFail(label, lintErr.Error()))
			continue
		}
		switch {
		case result.HasErrors():
			results = append(results, doctorResultFail(label, fmt.Sprintf("%d error(s)", result.ErrorCount())))
		case result.WarningCount() > 0:
			results = append(results, doctorResultWarn(label, fmt.Sprintf("%d warning(s)", result.WarningCount())))
		default:
			results = append(results, doctorResultPass(label))
		}
	}

	if !found {
		return []doctorResult{doctorResultPass("templates (none found in .tag/)")}
	}
	return results
}

// ---- Library checks ----

func doctorCheckLibraries() []doctorResult {
	dataDir, err := xdg.DataHome()
	if err != nil {
		return []doctorResult{doctorResultFail("libraries", fmt.Sprintf("cannot determine data directory: %v", err))}
	}
	lib, err := library.New(dataDir)
	if err != nil {
		return []doctorResult{doctorResultFail("libraries", fmt.Sprintf("cannot open library store: %v", err))}
	}
	entries, err := lib.List()
	if err != nil {
		return []doctorResult{doctorResultFail("libraries", fmt.Sprintf("cannot list libraries: %v", err))}
	}
	if len(entries) == 0 {
		return []doctorResult{doctorResultPass("libraries (none installed)")}
	}
	results := make([]doctorResult, 0, len(entries))
	for _, entry := range entries {
		label := fmt.Sprintf("library %q", entry.Name)
		tmplPath, pathErr := lib.TemplatePath(entry.Name)
		if pathErr != nil {
			results = append(results, doctorResultFail(label, pathErr.Error()))
			continue
		}
		if _, statErr := os.Stat(tmplPath); statErr != nil {
			results = append(results, doctorResultFail(label, "path not accessible: "+tmplPath))
		} else {
			results = append(results, doctorResultPass(label))
		}
	}
	return results
}

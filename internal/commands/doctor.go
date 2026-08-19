package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/internal/jsonout"
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

// doctorFlags returns the flags for the doctor command. doctor takes no
// positional argument, so there is nothing to reparse.
func doctorFlags() []cli.Flag {
	return []cli.Flag{formatFlag(formatText, formatJSON)}
}

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
		Flags: doctorFlags(),
		Action: func(c *cli.Context) error {
			format, err := resolveFormat(c, formatText, formatJSON)
			if err != nil {
				return err
			}
			return doctorAction(c.Context, cmdOut(c), version, format)
		},
	}
}

// doctorStatus is the pass/warn/fail verdict of a single check. It marshals
// as its String() form — never as the underlying int — so a JSON consumer
// never has to hardcode the enum ordering.
type doctorStatus int

const (
	doctorPass doctorStatus = iota
	doctorWarn
	doctorFail
)

// Wire names for doctorStatus. Named constants rather than inline literals so
// the JSON vocabulary has one definition that tests can reference too.
const (
	doctorStatusPass    = "pass"
	doctorStatusWarn    = "warn"
	doctorStatusFail    = "fail"
	doctorStatusUnknown = "unknown"
)

func (s doctorStatus) String() string {
	switch s {
	case doctorPass:
		return doctorStatusPass
	case doctorWarn:
		return doctorStatusWarn
	case doctorFail:
		return doctorStatusFail
	default:
		return doctorStatusUnknown
	}
}

// MarshalJSON encodes the status through String() so it always serialises as
// a string, never as the underlying int.
func (s doctorStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// DoctorResult is one check's outcome. Exported and tagged so it can be
// serialised directly for `doctor --format json`.
type DoctorResult struct {
	Label   string       `json:"label"`
	Status  doctorStatus `json:"status"`
	Message string       `json:"message,omitempty"`
}

// DoctorSection groups the checks that ran under one heading (ENVIRONMENT,
// PROJECT, TEMPLATES, LIBRARIES).
type DoctorSection struct {
	Name   string         `json:"name"`
	Checks []DoctorResult `json:"checks"`
}

// DoctorReport is the full result of a doctor run. Status is the worst status
// across all checks, so a consumer can gate on one field.
type DoctorReport struct {
	Status   doctorStatus    `json:"status"`
	Sections []DoctorSection `json:"sections"`
}

func doctorResultPass(label string) DoctorResult {
	return DoctorResult{Label: label, Status: doctorPass}
}

func doctorResultWarn(label, msg string) DoctorResult {
	return DoctorResult{Label: label, Status: doctorWarn, Message: msg}
}

func doctorResultFail(label, msg string) DoctorResult {
	return DoctorResult{Label: label, Status: doctorFail, Message: msg}
}

// buildDoctorReport runs every check section and assembles the report value.
// This is the single data-collection pass; both the text and JSON writers
// render from the resulting report rather than re-running checks.
func buildDoctorReport(ctx context.Context, version string) DoctorReport {
	raw := []struct {
		heading string
		results []DoctorResult
	}{
		{"ENVIRONMENT", doctorCheckEnvironment(ctx, version)},
		{"PROJECT", doctorCheckProject(".")},
		{"TEMPLATES", doctorCheckTemplates(".")},
		{"LIBRARIES", doctorCheckLibraries()},
	}

	sections := make([]DoctorSection, 0, len(raw))
	worst := doctorPass
	for _, s := range raw {
		checks := make([]DoctorResult, 0, len(s.results))
		checks = append(checks, s.results...)
		sections = append(sections, DoctorSection{Name: s.heading, Checks: checks})
		for _, r := range s.results {
			if r.Status > worst {
				worst = r.Status
			}
		}
	}

	return DoctorReport{Status: worst, Sections: sections}
}

// doctorAction collects the report, writes it in the requested format, and
// returns the exit-code-carrying error AFTER the write so JSON output is
// never truncated by an early return.
func doctorAction(ctx context.Context, w io.Writer, version, format string) error {
	report := buildDoctorReport(ctx, version)

	if format == formatJSON {
		if err := jsonout.Write(w, report); err != nil {
			return app.Errorf("write json: %w", err)
		}
	} else {
		printDoctorReport(w, report)
	}

	switch report.Status {
	case doctorFail:
		return &app.CommandError{Message: "doctor: one or more checks failed", Code: doctorExitFailures}
	case doctorWarn:
		return &app.CommandError{Message: "doctor: one or more checks produced warnings", Code: doctorExitWarnings}
	}
	return nil
}

func printDoctorReport(w io.Writer, report DoctorReport) {
	for i, s := range report.Sections {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, s.Name)
		printDoctorResults(w, s.Checks)
	}
}

func printDoctorResults(w io.Writer, results []DoctorResult) {
	for _, r := range results {
		icon := "✓"
		switch r.Status {
		case doctorWarn:
			icon = "⚠"
		case doctorFail:
			icon = "✗"
		}
		if r.Message != "" {
			fmt.Fprintf(w, "  %s  %s — %s\n", icon, r.Label, r.Message)
		} else {
			fmt.Fprintf(w, "  %s  %s\n", icon, r.Label)
		}
	}
}

// ---- Environment checks ----

func doctorCheckEnvironment(ctx context.Context, version string) []DoctorResult {
	return []DoctorResult{
		doctorCheckGit(),
		doctorCheckGitHubToken(),
		doctorCheckTAGVersion(ctx, version),
	}
}

func doctorCheckGit() DoctorResult {
	const label = "Git installed"
	if _, err := exec.LookPath("git"); err != nil {
		return doctorResultFail(label, "git not found in PATH — required for remote templates")
	}
	return doctorResultPass(label)
}

func doctorCheckGitHubToken() DoctorResult {
	const label = "GITHUB_TOKEN"
	if os.Getenv("GITHUB_TOKEN") == "" {
		return doctorResultWarn(label, "not set — needed for private repos and higher API rate limits")
	}
	return doctorResultPass(label)
}

func doctorCheckTAGVersion(ctx context.Context, version string) DoctorResult {
	label := fmt.Sprintf("TAG version (%s)", version)
	if isDevBuild(version) {
		return doctorResultPass(label + " — dev build, skipping update check")
	}
	latest, err := fetchLatestVersion(ctx, defaultGitHubRepo)
	if err != nil {
		return doctorResultWarn(label, fmt.Sprintf("could not check for updates: %v", err))
	}
	if strings.TrimPrefix(version, "v") != strings.TrimPrefix(latest, "v") {
		return doctorResultWarn(label, fmt.Sprintf("update available: %s → %s  (run: tag upgrade)", version, latest))
	}
	return doctorResultPass(label)
}

// ---- Project checks ----

func doctorCheckProject(root string) []DoctorResult {
	tagDir := filepath.Join(root, types.TemplatesDir)

	info, err := os.Stat(tagDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []DoctorResult{doctorResultWarn(".tag/ directory", "not found — run: tag template init")}
	}
	if err != nil {
		return []DoctorResult{doctorResultFail(".tag/ directory", err.Error())}
	}
	if !info.IsDir() {
		return []DoctorResult{doctorResultFail(".tag/ directory", ".tag exists but is not a directory")}
	}

	results := []DoctorResult{doctorResultPass(".tag/ directory")}
	results = append(results, doctorCheckSubdir(tagDir, types.SharedDir, ".tag/_shared/")...)
	results = append(results, doctorCheckSubdir(tagDir, types.BundlesDir, ".tag/_bundles/")...)
	return results
}

func doctorCheckSubdir(parent, subdir, label string) []DoctorResult {
	if _, err := os.Stat(filepath.Join(parent, subdir)); errors.Is(err, fs.ErrNotExist) {
		return []DoctorResult{doctorResultWarn(label, "directory not found")}
	} else if err != nil {
		return []DoctorResult{doctorResultFail(label, err.Error())}
	}
	return []DoctorResult{doctorResultPass(label)}
}

// ---- Template checks ----

func doctorCheckTemplates(root string) []DoctorResult {
	tagDir := filepath.Join(root, types.TemplatesDir)
	if _, err := os.Stat(tagDir); err != nil {
		return []DoctorResult{doctorResultPass("templates (no .tag/ found, skipped)")}
	}

	entries, err := os.ReadDir(tagDir)
	if err != nil {
		return []DoctorResult{doctorResultFail("templates", fmt.Sprintf("cannot read .tag/: %v", err))}
	}

	var results []DoctorResult
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
		return []DoctorResult{doctorResultPass("templates (none found in .tag/)")}
	}
	return results
}

// ---- Library checks ----

func doctorCheckLibraries() []DoctorResult {
	dataDir, err := xdg.DataHome()
	if err != nil {
		return []DoctorResult{doctorResultFail("libraries", fmt.Sprintf("cannot determine data directory: %v", err))}
	}
	lib, err := library.New(dataDir)
	if err != nil {
		return []DoctorResult{doctorResultFail("libraries", fmt.Sprintf("cannot open library store: %v", err))}
	}
	entries, err := lib.List()
	if err != nil {
		return []DoctorResult{doctorResultFail("libraries", fmt.Sprintf("cannot list libraries: %v", err))}
	}
	if len(entries) == 0 {
		return []DoctorResult{doctorResultPass("libraries (none installed)")}
	}
	results := make([]DoctorResult, 0, len(entries))
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

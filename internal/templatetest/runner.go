package templatetest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kaikenlabs/tag/internal/engine"
	"github.com/kaikenlabs/tag/internal/scaffold"
)

// RunOptions configures a test run.
type RunOptions struct {
	// TemplateRoot is the absolute path to the template being tested.
	// Defaults to the current directory.
	TemplateRoot string
	// TestsGlob is the glob pattern for fixture files.
	// Defaults to ".tag/tests/*.json" relative to TemplateRoot.
	TestsGlob string
	// FixtureFiles allows callers to pass explicit fixture paths instead of
	// using the glob (useful in tests). When non-nil, TestsGlob is ignored.
	FixtureFiles []string
}

// Run loads and executes all test fixtures matching opts.TestsGlob and returns
// a Report.  It never returns a non-nil error for assertion failures; errors
// are only returned for fatal setup problems (e.g. cannot read the fixtures
// directory).
func Run(ctx context.Context, opts RunOptions) (Report, error) {
	if opts.TemplateRoot == "" {
		opts.TemplateRoot = "."
	}

	var fixtureFiles []string
	if opts.FixtureFiles != nil {
		fixtureFiles = opts.FixtureFiles
	} else {
		glob := opts.TestsGlob
		if glob == "" {
			glob = filepath.Join(opts.TemplateRoot, ".tag", "tests", "*.json")
		}
		matches, err := filepath.Glob(glob)
		if err != nil {
			return Report{}, fmt.Errorf("glob fixtures: %w", err)
		}
		fixtureFiles = matches
	}

	var report Report
	for _, path := range fixtureFiles {
		fixture, err := loadFixture(path)
		if err != nil {
			report.Cases = append(report.Cases, CaseResult{
				Name:   path,
				Status: CaseErrored,
				Error:  "load fixture: " + err.Error(),
			})
			report.Errored++
			continue
		}

		result := runFixture(ctx, fixture, opts.TemplateRoot)
		report.Cases = append(report.Cases, result)
		switch result.Status {
		case CasePassed:
			report.Passed++
		case CaseFailed:
			report.Failed++
		case CaseErrored:
			report.Errored++
		}
	}

	return report, nil
}

// loadFixture reads and validates a fixture file.
func loadFixture(path string) (Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Fixture{}, fmt.Errorf("read %s: %w", path, err)
	}
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		return Fixture{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := validateFixture(&f); err != nil {
		return Fixture{}, fmt.Errorf("invalid fixture %s: %w", path, err)
	}
	return f, nil
}

// validateFixture checks required fields are present.
func validateFixture(f *Fixture) error {
	if f.Name == "" {
		return errors.New("field 'name' is required")
	}
	if f.Mode != ModeScaffold && f.Mode != ModeGenerate {
		return fmt.Errorf("field 'mode' must be 'scaffold' or 'generate', got %q", f.Mode)
	}
	if f.Template == "" {
		return errors.New("field 'template' is required")
	}
	if f.Mode == ModeGenerate && f.Target == "" {
		return errors.New("field 'target' is required when mode=generate")
	}
	for i, a := range f.Assertions {
		switch a.Type {
		case AssertFileExists, AssertFileNotExists:
		case AssertContentContains, AssertContentExcludes:
			if a.Value == "" {
				return fmt.Errorf("assertion[%d]: 'value' required for %s", i, a.Type)
			}
		case AssertContentMatches:
			if a.Pattern == "" {
				return fmt.Errorf("assertion[%d]: 'pattern' required for content_matches", i)
			}
		default:
			return fmt.Errorf("assertion[%d]: unknown type %q", i, a.Type)
		}
		if a.Path == "" {
			return fmt.Errorf("assertion[%d]: 'path' is required", i)
		}
	}
	return nil
}

// runFixture executes a single test case and returns its result.
func runFixture(ctx context.Context, f Fixture, templateRoot string) CaseResult {
	// Create isolated temp directory.
	tmpDir, err := os.MkdirTemp("", "tag-test-*")
	if err != nil {
		return caseError(f.Name, fmt.Sprintf("create temp dir: %v", err))
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Materialise setup_files before the generator runs.
	for rel, content := range f.SetupFiles {
		dest := filepath.Join(tmpDir, rel)
		if mkdirErr := os.MkdirAll(filepath.Dir(dest), 0o750); mkdirErr != nil {
			return caseError(f.Name, fmt.Sprintf("create setup dir for %s: %v", rel, mkdirErr))
		}
		// #nosec G306 -- fixture file for a template test run, no sensitive content
		if writeErr := os.WriteFile(dest, []byte(content), 0o644); writeErr != nil {
			return caseError(f.Name, fmt.Sprintf("write setup file %s: %v", rel, writeErr))
		}
	}

	// Execute the template in the temp directory.
	var outputDir string
	switch f.Mode {
	case ModeScaffold:
		outputDir, err = runScaffoldFixture(ctx, f, templateRoot, tmpDir)
	case ModeGenerate:
		outputDir = tmpDir
		err = runGenerateFixture(ctx, f, templateRoot, tmpDir)
	}
	if err != nil {
		return caseError(f.Name, fmt.Sprintf("execute fixture: %v", err))
	}

	// Run assertions.
	results := make([]AssertionResult, 0, len(f.Assertions))
	allPassed := true
	for _, a := range f.Assertions {
		ar := runAssertion(a, outputDir)
		results = append(results, ar)
		if !ar.Passed {
			allPassed = false
		}
	}

	status := CasePassed
	if !allPassed {
		status = CaseFailed
	}
	return CaseResult{
		Name:       f.Name,
		Status:     status,
		Assertions: results,
	}
}

// runScaffoldFixture runs the scaffold pipeline and returns the output directory.
func runScaffoldFixture(ctx context.Context, f Fixture, templateRoot, tmpBase string) (string, error) {
	// Resolve template path: if it starts with ./ or / treat it as relative to templateRoot.
	templateDir := f.Template
	if !filepath.IsAbs(templateDir) {
		templateDir = filepath.Join(templateRoot, templateDir)
	}

	opts := scaffold.Options{
		TemplateDir: templateDir,
		OutputDir:   filepath.Join(tmpBase, "output"),
		NoInput:     true,
		NoSave:      true,
		AcceptHooks: false, // never run hooks during tests
	}
	// Apply vars from fixture.
	if f.Vars != nil {
		meta := make(map[string]string, len(f.Vars))
		for k, v := range f.Vars {
			meta[k] = fmt.Sprintf("%v", v)
		}
		opts.Meta = meta
	}

	s, err := scaffold.NewScaffold(opts, scaffold.WithIsTTY(false))
	if err != nil {
		return "", fmt.Errorf("create scaffold: %w", err)
	}

	result, err := s.Run(opts)
	if err != nil {
		return "", fmt.Errorf("scaffold run: %w", err)
	}
	_ = ctx
	return result.OutputDir, nil
}

// runGenerateFixture runs the engine generator/bundle in the temp directory.
func runGenerateFixture(ctx context.Context, f Fixture, templateRoot, outputDir string) error {
	// Build the generator directory path.
	templateDir := filepath.Join(templateRoot, ".tag", f.Template)
	sharedDir := filepath.Join(templateRoot, ".tag", "_shared")

	if _, err := os.Stat(templateDir); err != nil {
		return fmt.Errorf("generator directory not found: %s", templateDir)
	}

	// Build raw meta from fixture Meta map.
	rawMeta := make([]string, 0, len(f.Meta))
	for k, v := range f.Meta {
		rawMeta = append(rawMeta, k+"="+v)
	}
	// Also add Vars as meta.
	for k, v := range f.Vars {
		rawMeta = append(rawMeta, fmt.Sprintf("%s=%v", k, v))
	}

	data := engine.Data{
		Name:    f.Target,
		RawMeta: rawMeta,
	}

	// Override the generator output directory to the temp dir.
	// We chdir BEFORE creating the generator so the writer's path safety
	// check uses the output directory as its base (it captures os.Getwd()
	// at construction time).
	origDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	if chdirErr := os.Chdir(outputDir); chdirErr != nil {
		return fmt.Errorf("chdir to output dir: %w", chdirErr)
	}
	defer func() { _ = os.Chdir(origDir) }()

	gen, err := engine.NewGenerator(false, templateDir, sharedDir, os.Stderr)
	if err != nil {
		return fmt.Errorf("create generator: %w", err)
	}

	_, err = gen.Generate(data)
	_ = ctx
	return err
}

// runAssertion checks a single assertion against the output directory.
func runAssertion(a Assertion, outputDir string) AssertionResult {
	fullPath := filepath.Join(outputDir, a.Path)

	switch a.Type {
	case AssertFileExists:
		if _, err := os.Stat(fullPath); err != nil {
			return AssertionResult{
				Assertion: a, Passed: false,
				Detail: "file not found: " + a.Path,
			}
		}
		return AssertionResult{Assertion: a, Passed: true}

	case AssertFileNotExists:
		if _, err := os.Stat(fullPath); err == nil {
			return AssertionResult{
				Assertion: a, Passed: false,
				Detail: "expected file to not exist: " + a.Path,
			}
		}
		return AssertionResult{Assertion: a, Passed: true}

	case AssertContentContains, AssertContentExcludes:
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return AssertionResult{
				Assertion: a, Passed: false,
				Detail: fmt.Sprintf("read file %s: %v", a.Path, err),
			}
		}
		contains := strings.Contains(string(content), a.Value)
		if a.Type == AssertContentContains && !contains {
			return AssertionResult{
				Assertion: a, Passed: false,
				Detail: fmt.Sprintf("%s: does not contain %q", a.Path, a.Value),
			}
		}
		if a.Type == AssertContentExcludes && contains {
			return AssertionResult{
				Assertion: a, Passed: false,
				Detail: fmt.Sprintf("%s: should not contain %q", a.Path, a.Value),
			}
		}
		return AssertionResult{Assertion: a, Passed: true}

	case AssertContentMatches:
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return AssertionResult{
				Assertion: a, Passed: false,
				Detail: fmt.Sprintf("read file %s: %v", a.Path, err),
			}
		}
		re, err := regexp.Compile(a.Pattern)
		if err != nil {
			return AssertionResult{
				Assertion: a, Passed: false,
				Detail: fmt.Sprintf("invalid pattern %q: %v", a.Pattern, err),
			}
		}
		if !re.Match(content) {
			return AssertionResult{
				Assertion: a, Passed: false,
				Detail: fmt.Sprintf("%s: content does not match /%s/", a.Path, a.Pattern),
			}
		}
		return AssertionResult{Assertion: a, Passed: true}
	}

	return AssertionResult{
		Assertion: a, Passed: false,
		Detail: "unknown assertion type: " + string(a.Type),
	}
}

// caseError returns a CaseErrored result.
func caseError(name, msg string) CaseResult {
	return CaseResult{Name: name, Status: CaseErrored, Error: msg}
}

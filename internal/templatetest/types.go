// Package templatetest provides a test runner for TAG templates.
// Test fixtures are defined as JSON files under .tag/tests/*.json, each
// describing input variables, optional pre-existing files, and assertions
// about the generated output.
package templatetest

// Mode indicates whether a fixture exercises scaffold (full-project) or
// generate (file-injection) mode.
type Mode string

const (
	// ModeScaffold runs the full scaffold pipeline.
	ModeScaffold Mode = "scaffold"
	// ModeGenerate runs a single generator or bundle.
	ModeGenerate Mode = "generate"
)

// AssertionType identifies the kind of assertion to perform.
type AssertionType string

const (
	// AssertFileExists asserts that the file at Path exists.
	AssertFileExists AssertionType = "file_exists"
	// AssertFileNotExists asserts that the file at Path does not exist.
	AssertFileNotExists AssertionType = "file_not_exists"
	// AssertContentContains asserts that the file contains Value as a substring.
	AssertContentContains AssertionType = "content_contains"
	// AssertContentExcludes asserts that the file does not contain Value as a substring.
	AssertContentExcludes AssertionType = "content_excludes"
	// AssertContentMatches asserts that the file content matches the regex in Pattern.
	AssertContentMatches AssertionType = "content_matches"
)

// Assertion describes one expected property of generated output.
type Assertion struct {
	// Type is the assertion kind.
	Type AssertionType `json:"type"`
	// Path is the file path relative to the output directory.
	Path string `json:"path"`
	// Value is used by content_contains and content_excludes assertions.
	Value string `json:"value,omitempty"`
	// Pattern is a regex used by content_matches assertions.
	Pattern string `json:"pattern,omitempty"`
}

// Fixture describes a single template test case.
type Fixture struct {
	// Name is a human-readable identifier for the test.
	Name string `json:"name"`
	// Mode indicates scaffold or generate mode.
	Mode Mode `json:"mode"`
	// Template is the scaffold ref (local path or remote) for scaffold mode,
	// or the generator/bundle name for generate mode.
	Template string `json:"template"`
	// Target is the entity name argument for generate mode (required when mode=generate).
	Target string `json:"target,omitempty"`
	// Vars contains variable values passed to the scaffold or generator.
	Vars map[string]any `json:"vars,omitempty"`
	// Meta contains key=value overrides for generate mode (--meta flag).
	Meta map[string]string `json:"meta,omitempty"`
	// SetupFiles maps relative paths to content strings that are written into
	// the output directory before the generator/scaffold runs. This is useful
	// for testing inject and append actions that require pre-existing files.
	SetupFiles map[string]string `json:"setup_files,omitempty"`
	// Assertions is the list of expected properties to verify.
	Assertions []Assertion `json:"assertions"`
}

// CaseStatus represents the outcome of a single test case.
type CaseStatus string

const (
	// CasePassed indicates all assertions passed.
	CasePassed CaseStatus = "passed"
	// CaseFailed indicates one or more assertions failed.
	CaseFailed CaseStatus = "failed"
	// CaseErrored indicates the test runner could not execute the fixture.
	CaseErrored CaseStatus = "errored"
)

// AssertionResult captures the outcome of one assertion.
type AssertionResult struct {
	Assertion Assertion
	Passed    bool
	Detail    string
}

// CaseResult captures the outcome of one test fixture.
type CaseResult struct {
	Name       string
	Status     CaseStatus
	Assertions []AssertionResult
	Error      string // set when Status == CaseErrored
}

// Report summarises the results of a full test run.
type Report struct {
	Cases   []CaseResult
	Passed  int
	Failed  int
	Errored int
}

// ExitCode returns the appropriate process exit code for the report.
//
//	0 = all pass
//	1 = one or more assertion failures
//	2 = one or more test errors (fixture/runner problems)
func (r Report) ExitCode() int {
	if r.Errored > 0 {
		return 2
	}
	if r.Failed > 0 {
		return 1
	}
	return 0
}

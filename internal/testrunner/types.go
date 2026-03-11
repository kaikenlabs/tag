package testrunner

import (
	"encoding/json"
	"fmt"
	"time"
)

// Exit codes for the test command.
const (
	ExitOK      = 0
	ExitFailure = 1
	ExitError   = 2
)

// Config holds the full configuration for a matrix test run.
type Config struct {
	TemplateDir string            // Path to template directory
	Meta        map[string]string // Required string variable overrides
	ValuesFile  string            // Path to values JSON file
	SkipVars    []string          // Boolean vars to exclude from permutation (use default)
	PinVars     map[string]string // Vars to fix at a specific value (not permuted)
	RunCommands []string          // Validation commands (overrides template config)
	Filter      string            // Filter expression (index or key=value pairs)
	Parallel    int               // Max concurrent test runs
	FailFast    bool              // Stop on first failure
	DryRun      bool              // List combinations without running
	KeepFailed  bool              // Keep scaffolded dirs on failure
	Timeout     time.Duration     // Per-command timeout
	MaxCases    int               // Safety limit for total combinations (0 = unlimited)
	Verbose     bool              // Show full output on failures
	AcceptHooks bool              // Opt-in hook and test command execution
	Format      string            // Output format: "text" or "json"
}

// TestPlan holds the resolved plan for a matrix test run.
type TestPlan struct {
	TemplateDir string
	BoolVars    []string
	Combos      []Combination
	Commands    []string
	Env         map[string]string
	ProjectName string
}

// Combination represents a single set of boolean variable assignments.
type Combination struct {
	Index int               `json:"index"`
	Vars  map[string]string `json:"vars"`
}

// CaseStatus represents the outcome of a single test case.
type CaseStatus int

const (
	CasePassed CaseStatus = iota
	CaseFailed
	CaseErrored
)

var caseStatusStrings = [...]string{"passed", "failed", "errored"}

// String returns the human-readable name of the status.
func (s *CaseStatus) String() string {
	if int(*s) < len(caseStatusStrings) {
		return caseStatusStrings[*s]
	}
	return fmt.Sprintf("CaseStatus(%d)", *s)
}

// MarshalJSON encodes CaseStatus as a JSON string.
func (s *CaseStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON decodes a JSON string into CaseStatus.
func (s *CaseStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	for i, name := range caseStatusStrings {
		if name == str {
			*s = CaseStatus(i)
			return nil
		}
	}
	return fmt.Errorf("unknown CaseStatus %q", str)
}

// CaseResult holds the outcome of testing a single combination.
type CaseResult struct {
	Combination Combination   `json:"combination"`
	Status      CaseStatus    `json:"status"`
	Phase       string        `json:"phase,omitempty"`    // "scaffold", "validate:<cmd>", etc.
	Output      string        `json:"output,omitempty"`   // Captured stdout+stderr on failure
	Error       string        `json:"error,omitempty"`    // Error message
	KeptDir     string        `json:"kept_dir,omitempty"` // Path to preserved output dir
	Duration    time.Duration `json:"duration"`
}

// Report aggregates all test case results.
type Report struct {
	Cases       []CaseResult  `json:"cases"`
	Passed      int           `json:"passed"`
	Failed      int           `json:"failed"`
	Errored     int           `json:"errored"`
	TotalCases  int           `json:"total_cases"`
	Duration    time.Duration `json:"duration"`
	TemplateDir string        `json:"template_dir"`
}

// ExitCode returns the appropriate process exit code for the report.
func (r Report) ExitCode() int {
	if r.Errored > 0 {
		return ExitError
	}
	if r.Failed > 0 {
		return ExitFailure
	}
	return ExitOK
}
